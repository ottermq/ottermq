package vhost

import (
	"fmt"
	"net"

	"github.com/rs/zerolog/log"
)

func (vh *VHost) HandleBasicAck(conn net.Conn, channel uint16, deliveryTag uint64, multiple bool) error {
	toDelete, err := vh.popUnackedRecords(conn, channel, deliveryTag, multiple)
	if err != nil {
		return err
	}

	if len(toDelete) == 0 {
		return nil
	}

	if vh.persist != nil {
		for _, rec := range toDelete {
			if rec.Persistent {
				_ = vh.persist.DeleteMessage(vh.Name, rec.QueueName, rec.Message.ID)
			}
		}
	}

	return nil
}

func (vh *VHost) popUnackedRecords(conn net.Conn, channel uint16, deliveryTag uint64, multiple bool) ([]*DeliveryRecord, error) {
	key := ConnectionChannelKey{conn, channel}

	vh.mu.Lock()
	ch := vh.ChannelDeliveries[key]
	vh.mu.Unlock()
	if ch == nil {
		return nil, fmt.Errorf("no channel delivery state for channel %d", channel)
	}

	removed := make([]*DeliveryRecord, 0)

	ch.mu.Lock()
	if multiple {
		// Multiple=true: must scan flat map for numeric ordering
		for tag, record := range ch.UnackedByTag {
			log.Debug().Uint64("tag", tag).Msg("Checking unacked tag for multiple ack")
			if tag <= deliveryTag {
				removed = append(removed, record)

				deleteUnackedDelivery(ch, tag, record.ConsumerTag)

				log.Debug().Uint64("tag", tag).Msg("Removed unacked tag for multiple ack")
			}
		}
	} else {
		// Single ack: O(1) lookup in flat map
		if record, exists := ch.UnackedByTag[deliveryTag]; exists {
			// TODO: refactor to avoid code duplication with above
			removed = append(removed, record)

			// Remove from both maps
			deleteUnackedDelivery(ch, deliveryTag, record.ConsumerTag)

			log.Debug().Uint64("tag", deliveryTag).Msg("Removed unacked tag for single ack")
		}
	}
	// Notify any waiters that unacked state has changed
	select {
	case ch.unackedChanged <- struct{}{}:
	default:
	}

	ch.mu.Unlock()
	return removed, nil
}

func deleteUnackedDelivery(ch *ChannelDeliveryState, tag uint64, consumerTag string) {
	delete(ch.UnackedByTag, tag)

	// Remove from consumer's nested map
	if consumerMap, exists := ch.UnackedByConsumer[consumerTag]; exists {
		delete(consumerMap, tag)
		if len(consumerMap) == 0 {
			delete(ch.UnackedByConsumer, consumerTag)
		}
	}
}

func (vh *VHost) HandleBasicReject(conn net.Conn, channel uint16, deliveryTag uint64, requeue bool) error {
	return vh.HandleBasicNack(conn, channel, deliveryTag, false, requeue)
}

func (vh *VHost) HandleBasicNack(conn net.Conn, channel uint16, deliveryTag uint64, multiple, requeue bool) error {
	recordsToNack, err := vh.popUnackedRecords(conn, channel, deliveryTag, multiple)
	if err != nil {
		return err
	}
	if len(recordsToNack) == 0 {
		return nil
	}

	if requeue {
		for _, record := range recordsToNack {
			vh.mu.Lock()
			queue := vh.Queues[record.QueueName]
			vh.mu.Unlock()
			if queue != nil {
				if vh.handleTTLExpiration(record.Message, queue) {
					continue
				}
				log.Debug().Msgf("Requeuing message with delivery tag %d on channel %d\n", record.DeliveryTag, channel)
				vh.markAsRedelivered(record.Message.ID)
				queue.Push(record.Message)
			}
		}
		return nil
	}
	// Dead-letter or discard the messages
	for _, record := range recordsToNack {
		// Check if DLX extension is enabled and queue has DLX configured
		// DLX properties comes directly from arguments: single source of truth
		vh.mu.Lock()
		queue, exists := vh.Queues[record.QueueName]
		vh.mu.Unlock()
		if !exists {
			continue
		}
		ok := vh.handleDeadLetter(queue, record.Message, REASON_REJECTED)
		if ok {
			continue
		}
		// Discard the message (no DLX configured or dead-lettering failed)
		log.Debug().Uint64("delivery_tag", record.DeliveryTag).Uint16("channel", channel).Msg("Nack: discarding message")
		if record.Persistent {
			_ = vh.persist.DeleteMessage(vh.Name, record.QueueName, record.Message.ID)
		}
	}
	return nil
}

// handleDeadLetter checks if dead-lettering is possible and performs it
func (vh *VHost) handleDeadLetter(queue *Queue, msg Message, reason ReasonType) bool {
	if vh.ActiveExtensions["dlx"] && vh.DeadLetterer != nil {
		vh.mu.Lock()
		args := queue.Props.Arguments
		vh.mu.Unlock()
		dlx, dlxExists := args["x-dead-letter-exchange"].(string)

		if dlxExists && dlx != "" {
			err := vh.DeadLetterer.DeadLetter(msg, queue, reason)
			if err == nil {
				log.Debug().Msg("Dead-lettering succeeded")
				// Dead-lettering succeeded
				return true
			}
			log.Error().Err(err).Msg("Dead-lettering failed")
		}
	}
	// No DLX configured or dead-lettering not possible
	return false
}

func (vh *VHost) HandleBasicQos(conn net.Conn, channel uint16, prefetchCount uint16, global bool) error {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	key := ConnectionChannelKey{conn, channel}
	state := vh.ChannelDeliveries[key]
	// fetch the channel delivery state if global is true
	if state == nil {
		state = &ChannelDeliveryState{
			UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
			UnackedByTag:      make(map[uint64]*DeliveryRecord),
			unackedChanged:    make(chan struct{}, 1),
			FlowActive:        true, // Default to flow active
		}
		vh.ChannelDeliveries[key] = state
	}
	if global {
		state.GlobalPrefetchCount = prefetchCount
		state.PrefetchGlobal = true
	} else {
		// Store the prefetch count to apply to new consumers if global is false
		state.NextPrefetchCount = prefetchCount
	}
	return nil
}
