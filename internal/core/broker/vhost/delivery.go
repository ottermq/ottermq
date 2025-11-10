package vhost

import (
	"fmt"
	"net"
	"sync"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/rs/zerolog/log"
)

type DeliveryRecord struct {
	DeliveryTag uint64
	ConsumerTag string
	QueueName   string
	Message     amqp.Message
	Persistent  bool
}

type ChannelDeliveryState struct {
	mu                  sync.Mutex
	LastDeliveryTag     uint64
	Unacked             map[uint64]*DeliveryRecord // deliveryTag -> DeliveryRecord
	GlobalPrefetchCount uint16
	NextPrefetchCount   uint16 // to be applied on the next consumer start
	PrefetchGlobal      bool
	unackedChanged      chan struct{}
	FlowActive          bool
}

// HandleChannelFlow processes a CHANNEL.FLOW request for the given channel
func (vh *VHost) HandleChannelFlow(conn net.Conn, channel uint16, flowActive bool) error {
	channelState := vh.getChannelDeliveryState(conn, channel)
	if channelState == nil {
		return fmt.Errorf("channel %d not found", channel)
	}
	channelState.mu.Lock()
	defer channelState.mu.Unlock()
	channelState.FlowActive = flowActive
	return nil
}

func (vh *VHost) GetChannelFlowState(conn net.Conn, channel uint16) bool {
	channelState := vh.getChannelDeliveryState(conn, channel)
	if channelState == nil {
		return true // default to active if channel not found
	}
	channelState.mu.Lock()
	defer channelState.mu.Unlock()
	return channelState.FlowActive
}

func (vh *VHost) deliverToConsumer(consumer *Consumer, msg amqp.Message, redelivered bool) error {
	// If caller didn't force redelivered, consult the mark set during requeue/recover.
	if !redelivered {
		redelivered = vh.ShouldRedeliver(msg.ID)
	}
	if !consumer.Active {
		return fmt.Errorf("consumer %s on channel %d is not active", consumer.Tag, consumer.Channel)
	}
	channelKey := ConnectionChannelKey{consumer.Connection, consumer.Channel}
	ch := vh.GetOrCreateChannelDelivery(channelKey)

	ch.mu.Lock()
	ch.LastDeliveryTag++
	tag := ch.LastDeliveryTag

	track := !consumer.Props.NoAck // only track when manual ack is required
	if track {
		ch.Unacked[tag] = &DeliveryRecord{
			DeliveryTag: tag,
			ConsumerTag: consumer.Tag,
			QueueName:   consumer.QueueName,
			Message:     msg,
			Persistent:  msg.Properties.DeliveryMode == amqp.PERSISTENT,
		}
	}
	ch.mu.Unlock()

	deliverFrame := vh.framer.CreateBasicDeliverFrame(
		consumer.Channel,
		consumer.Tag,
		msg.Exchange,
		msg.RoutingKey,
		tag,
		redelivered,
	)
	headerFrame := vh.framer.CreateHeaderFrame(consumer.Channel, uint16(amqp.BASIC), msg)
	bodyFrame := vh.framer.CreateBodyFrame(consumer.Channel, msg.Body)

	if err := vh.framer.SendFrame(consumer.Connection, deliverFrame); err != nil {
		log.Error().Err(err).Msg("Failed to send deliver frame")
		if track {
			ch.mu.Lock()
			delete(ch.Unacked, tag)
			ch.mu.Unlock()
		}
		return err
	}
	if err := vh.framer.SendFrame(consumer.Connection, headerFrame); err != nil {
		log.Error().Err(err).Msg("Failed to send header frame")
		if track {
			ch.mu.Lock()
			delete(ch.Unacked, tag)
			ch.mu.Unlock()
		}
		return err
	}
	if err := vh.framer.SendFrame(consumer.Connection, bodyFrame); err != nil {
		log.Error().Err(err).Msg("Failed to send body frame")
		if track {
			ch.mu.Lock()
			delete(ch.Unacked, tag)
			ch.mu.Unlock()
		}
		return err
	}

	// Persistence
	if consumer.Props.NoAck && vh.persist != nil && msg.Properties.DeliveryMode == amqp.PERSISTENT {
		if err := vh.persist.DeleteMessage(vh.Name, consumer.QueueName, msg.ID); err != nil {
			log.Error().Err(err).Msg("Failed to delete persisted message after delivery with no-ack")
		}
	}

	if redelivered {
		// Clear the one-shot mark so subsequent deliveries default to non-redelivered.
		vh.clearRedeliveredMark(msg.ID)
	}
	return nil
}

func (vh *VHost) GetOrCreateChannelDelivery(channelKey ConnectionChannelKey) *ChannelDeliveryState {
	vh.mu.Lock()
	ch := vh.ChannelDeliveries[channelKey]
	if ch == nil {
		ch = &ChannelDeliveryState{
			Unacked:        make(map[uint64]*DeliveryRecord),
			unackedChanged: make(chan struct{}, 1),
		}
		vh.ChannelDeliveries[channelKey] = ch
	}
	vh.mu.Unlock()
	return ch
}

func (vh *VHost) ShouldRedeliver(msgID string) bool {
	vh.redeliveredMu.Lock()
	_, exists := vh.redeliveredMessages[msgID]
	vh.redeliveredMu.Unlock()
	return exists
}

func (vh *VHost) clearRedeliveredMark(msgID string) {
	vh.redeliveredMu.Lock()
	delete(vh.redeliveredMessages, msgID)
	vh.redeliveredMu.Unlock()
}

// shouldThrottle checks if the consumer should be throttled based on the channel's delivery state
func (vh *VHost) shouldThrottle(consumer *Consumer, channelState *ChannelDeliveryState) bool {
	if channelState == nil {
		return false // No QoS set, no throttling
	}
	// It should throttle if:
	// a) Prefetch is global AND total unacked on channel >= global prefetch AND global prefetch > 0
	// b) Prefetch is per-consumer AND total unacked on channel for this consumer >= consumer prefetch AND consumer prefetch > 0
	// c) Flow is paused on the channel

	reason := []struct {
		name      string
		condition bool
	}{
		{"flow paused", !channelState.FlowActive},
		{"global qos", channelState.PrefetchGlobal &&
			channelState.GlobalPrefetchCount > 0 &&
			vh.getUnackedCountChannel(channelState) >= channelState.GlobalPrefetchCount},
		{"consumer_qos", !channelState.PrefetchGlobal &&
			consumer.PrefetchCount > 0 &&
			vh.getUnackedCountConsumer(channelState, consumer) >= consumer.PrefetchCount},
	}
	for _, r := range reason {
		if r.condition {
			log.Trace().Str("reason", r.name).
				Str("consumer", consumer.Tag).
				Uint16("channel", consumer.Channel).
				Msg("Throttling delivery")
			return true
		}
	}
	/// Previous implementation:
	// if channelState.PrefetchGlobal {
	// 	if channelState.GlobalPrefetchCount > 0 {
	// 		unackedCount := vh.getUnackedCountChannel(channelState)
	// 		if unackedCount >= channelState.GlobalPrefetchCount {
	// 			return true
	// 		}
	// 	}
	// } else {
	// 	if consumer.PrefetchCount > 0 {
	// 		unackedCount := vh.getUnackedCountConsumer(channelState, consumer)
	// 		if unackedCount >= consumer.PrefetchCount {
	// 			return true
	// 		}
	// 	}
	// }
	return false
}

func (vh *VHost) getUnackedCountChannel(channelState *ChannelDeliveryState) uint16 {
	channelState.mu.Lock()
	unackedCount := len(channelState.Unacked)
	channelState.mu.Unlock()
	return uint16(unackedCount)
}

func (vh *VHost) getUnackedCountConsumer(channelState *ChannelDeliveryState, consumer *Consumer) uint16 {
	channelState.mu.Lock()
	defer channelState.mu.Unlock()
	unackedCount := 0
	for _, record := range channelState.Unacked {
		if record.ConsumerTag == consumer.Tag {
			unackedCount++
		}
	}
	return uint16(unackedCount)
}

func (vh *VHost) getChannelDeliveryState(connection net.Conn, channel uint16) *ChannelDeliveryState {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	key := ConnectionChannelKey{connection, channel}
	return vh.ChannelDeliveries[key]
}

func (ch *ChannelDeliveryState) TrackDelivery(noAck bool, msg *amqp.Message, queue string) uint64 {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.LastDeliveryTag++
	deliveryTag := ch.LastDeliveryTag

	// Track the delivery if manual ack is required
	if !noAck {
		record := &DeliveryRecord{
			DeliveryTag: deliveryTag,
			ConsumerTag: "",
			Message:     *msg,
			QueueName:   queue,
			Persistent:  msg.Properties.DeliveryMode == amqp.PERSISTENT,
		}
		ch.Unacked[deliveryTag] = record
		log.Debug().Uint64("delivery_tag", deliveryTag).Msg("Tracking Basic.Get delivery for manual ack")
	}
	return deliveryTag
}
