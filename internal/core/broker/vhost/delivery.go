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
}

func (vh *VHost) deliverToConsumer(consumer *Consumer, msg amqp.Message, redelivered bool) error {
	// If caller didn't force redelivered, consult the mark set during requeue/recover.
	if !redelivered {
		redelivered = vh.shouldRedeliver(msg.ID)
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

func (vh *VHost) shouldRedeliver(msgID string) bool {
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

func (vh *VHost) shouldThrottle(consumer *Consumer, channelState *ChannelDeliveryState) bool {
	if channelState == nil {
		return false // No QoS set, no throttling
	}
	// It should throttle if:
	// a) Prefetch is global AND total unacked on channel >= global prefetch AND global prefetch > 0
	if channelState.PrefetchGlobal {
		if channelState.GlobalPrefetchCount > 0 {
			unackedCount := vh.getUnackedCountChannel(channelState)
			if unackedCount >= channelState.GlobalPrefetchCount {
				return true
			}
		}
	} else {
		// b) Prefetch is per-consumer AND total unacked on channel for this consumer >= consumer prefetch AND consumer prefetch > 0
		if consumer.PrefetchCount > 0 {
			unackedCount := vh.getUnackedCountConsumer(channelState, consumer)
			if unackedCount >= consumer.PrefetchCount {
				return true
			}
		}
	}
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
