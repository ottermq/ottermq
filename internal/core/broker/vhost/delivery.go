package vhost

import (
	"fmt"
	"sync"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/rs/zerolog/log"
)

type DeliveryRecord struct {
	DeliveryTag uint64
	ConsumerTag string
	QueueName   string
	Message     Message
	Persistent  bool
}

type ChannelDeliveryState struct {
	mu              sync.Mutex
	LastDeliveryTag uint64

	// Dual-index for unacked messages
	// Primary index: nested by consumer tag for O(1) lookups when acknowledging
	UnackedByConsumer map[string]map[uint64]*DeliveryRecord // consumerTag -> (deliveryTag -> DeliveryRecord)

	// Secondary index: flat map for O(1) deliveryTag lookups (ack/nack)
	UnackedByTag map[uint64]*DeliveryRecord // deliveryTag -> DeliveryRecord

	GlobalPrefetchCount   uint16
	NextPrefetchCount     uint16 // to be applied on the next consumer start
	PrefetchGlobal        bool
	unackedChanged        chan struct{}
	FlowActive            bool
	FlowInitiatedByBroker bool // default false
}

type FlowState struct {
	FlowActive            bool
	FlowInitiatedByBroker bool
}

// HandleChannelFlow processes a CHANNEL.FLOW request for the given channel
func (vh *VHost) HandleChannelFlow(connID ConnectionID, channel uint16, flowActive bool, flowInitiatedByBroker bool) error {
	channelKey := ConnectionChannelKey{connID, channel}
	channelState := vh.GetOrCreateChannelDelivery(channelKey)

	channelState.mu.Lock()
	defer channelState.mu.Unlock()
	channelState.FlowActive = flowActive
	channelState.FlowInitiatedByBroker = flowInitiatedByBroker
	log.Debug().
		Uint16("channel", channel).
		Bool("flow_active", flowActive).
		Bool("flow_initiated_by_broker", flowInitiatedByBroker).
		Msg("Channel flow state updated")
	return nil
}

func (vh *VHost) GetChannelFlowState(connID ConnectionID, channel uint16) FlowState {
	channelState := vh.getChannelDeliveryState(connID, channel)
	if channelState == nil {
		return FlowState{
			FlowActive:            true,
			FlowInitiatedByBroker: false,
		}
	}

	channelState.mu.Lock()
	defer channelState.mu.Unlock()

	return FlowState{
		FlowActive:            channelState.FlowActive,
		FlowInitiatedByBroker: channelState.FlowInitiatedByBroker,
	}
}

func (vh *VHost) deliverToConsumer(consumer *Consumer, msg Message, redelivered bool) error {
	if vh.frameSender == nil {
		return fmt.Errorf("frame sender not set in vhost")
	}
	// If caller didn't force redelivered, consult the mark set during requeue/recover.
	if !redelivered {
		redelivered = vh.ShouldRedeliver(msg.ID)
	}
	if !consumer.Active {
		return fmt.Errorf("consumer %s on channel %d is not active", consumer.Tag, consumer.Channel)
	}
	channelKey := ConnectionChannelKey{consumer.ConnectionID, consumer.Channel}
	ch := vh.GetOrCreateChannelDelivery(channelKey)

	ch.mu.Lock()
	ch.LastDeliveryTag++
	tag := ch.LastDeliveryTag

	track := !consumer.Props.NoAck // only track when manual ack is required
	if track {
		record := &DeliveryRecord{
			DeliveryTag: tag,
			ConsumerTag: consumer.Tag,
			QueueName:   consumer.QueueName,
			Message:     msg,
			Persistent:  msg.Properties.DeliveryMode == amqp.PERSISTENT,
		}

		// Dual-index: add to both maps
		ch.UnackedByTag[tag] = record

		if ch.UnackedByConsumer[consumer.Tag] == nil {
			ch.UnackedByConsumer[consumer.Tag] = make(map[uint64]*DeliveryRecord)
		}
		ch.UnackedByConsumer[consumer.Tag][tag] = record

		log.Debug().
			Uint64("delivery_tag", tag).
			Str("consumer", consumer.Tag).
			Str("queue", consumer.QueueName).
			Msg("Tracking delivery for manual ack")
	}

	ch.mu.Unlock()
	amqpMsg := msg.ToAMQPMessage()
	// Create frames and append them as a single slice
	frames := append(
		vh.framer.CreateBasicDeliverFrame(
			consumer.Channel,
			consumer.Tag,
			msg.Exchange,
			msg.RoutingKey,
			tag,
			redelivered,
		), append(
			vh.framer.CreateHeaderFrame(consumer.Channel, uint16(amqp.BASIC), amqpMsg),
			vh.framer.CreateBodyFrame(consumer.Channel, msg.Body)...)...)

	// if err := vh.framer.SendFrame(consumer.Connection, frames); err != nil {
	if err := vh.frameSender.SendFrame(consumer.ConnectionID, consumer.Channel, frames); err != nil {
		log.Error().Err(err).Msg("Failed to send frames")
		if track {
			ch.mu.Lock()
			deleteUnackedDelivery(ch, tag, consumer.Tag)

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
			UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
			UnackedByTag:      make(map[uint64]*DeliveryRecord),
			unackedChanged:    make(chan struct{}, 1),
			FlowActive:        true,
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
	return false
}

func (vh *VHost) getUnackedCountChannel(channelState *ChannelDeliveryState) uint16 {
	channelState.mu.Lock()
	unackedCount := len(channelState.UnackedByTag)
	channelState.mu.Unlock()
	return uint16(unackedCount)
}

func (vh *VHost) getUnackedCountConsumer(channelState *ChannelDeliveryState, consumer *Consumer) uint16 {
	channelState.mu.Lock()
	defer channelState.mu.Unlock()
	consumerMap, exists := channelState.UnackedByConsumer[consumer.Tag]
	if !exists {
		return 0
	}
	return uint16(len(consumerMap))
}

func (vh *VHost) getChannelDeliveryState(connectionID ConnectionID, channel uint16) *ChannelDeliveryState {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	key := ConnectionChannelKey{connectionID, channel}
	return vh.ChannelDeliveries[key]
}

func (ch *ChannelDeliveryState) TrackDelivery(noAck bool, msg *Message, queue string) uint64 {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.LastDeliveryTag++
	deliveryTag := ch.LastDeliveryTag
	consumerTag := BASIC_GET_SENTINEL // Basic.Get uses sentinel value

	// Track the delivery if manual ack is required
	if !noAck {
		record := &DeliveryRecord{
			DeliveryTag: deliveryTag,
			ConsumerTag: consumerTag,
			Message:     *msg,
			QueueName:   queue,
			Persistent:  msg.Properties.DeliveryMode == amqp.PERSISTENT,
		}
		// Dual-index: add to both maps
		ch.UnackedByTag[deliveryTag] = record

		if ch.UnackedByConsumer[consumerTag] == nil {
			ch.UnackedByConsumer[consumerTag] = make(map[uint64]*DeliveryRecord)
		}
		ch.UnackedByConsumer[consumerTag][deliveryTag] = record

		log.Debug().
			Uint64("delivery_tag", deliveryTag).
			Str("consumer", consumerTag).
			Str("queue", queue).
			Msg("Tracking Basic.Get delivery for manual ack")
	}
	return deliveryTag
}
