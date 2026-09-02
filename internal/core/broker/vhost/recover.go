package vhost

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

// HandleBasicRecover processes a Basic.Recover AMQP frame, handling message requeueing if necessary.
func (vh *VHost) HandleBasicRecover(connID ConnectionID, channel uint16, requeue bool) error {
	key := ConnectionChannelKey{connID, channel}

	vh.mu.Lock()
	ch := vh.ChannelDeliveries[key]
	vh.mu.Unlock()
	if ch == nil {
		return fmt.Errorf("no channel delivery state for channel %d", channel)
	}

	ch.mu.Lock()
	unackedMessages := fetchUnackedMessages(ch)

	// Clear both indexes
	ch.UnackedByConsumer = make(map[string]map[uint64]*DeliveryRecord)
	ch.UnackedByTag = make(map[uint64]*DeliveryRecord)
	ch.mu.Unlock()

	for _, record := range unackedMessages {
		vh.mu.Lock()
		q := vh.Queues[record.QueueName]
		vh.mu.Unlock()
		if q != nil && vh.handleTTLExpiration(record.Message, q) {
			continue
		}
		if requeue {
			log.Debug().Msgf("Requeuing message with delivery tag %d on channel %d\n", record.DeliveryTag, channel)
			vh.requeueMessage(record)
		} else {
			// Basic.Recover with requeue=false: redeliver to original consumer
			// Skip Basic.Get deliveries (sentinel value) - they have no consumer to redeliver to
			if record.ConsumerTag == BASIC_GET_SENTINEL {
				log.Debug().Msg("Skipping Basic.Get delivery on Basic.Recover(requeue=false), requeuing")
				vh.requeueMessage(record)
				continue
			}

			vh.mu.Lock()
			consumerKey := ConnectionChannelKey{connID, channel}
			consumers := vh.ConsumersByChannel[consumerKey]
			vh.mu.Unlock()
			var targetConsumer *Consumer
			if len(consumers) > 0 {
				for _, c := range consumers {
					if c.Tag == record.ConsumerTag {
						targetConsumer = c
						break
					}
				}
			}
			if targetConsumer != nil {
				err := vh.deliverToConsumer(targetConsumer, record.Message, true)
				if err != nil {
					// Delivery failed, requeue to avoid message loss
					log.Debug().Err(err).Msg("Failed to redeliver recovered message, requeuing")
					vh.requeueMessage(record)
				}
			} else {
				// consumer no longer exists, requeue
				log.Debug().Str("consumer", record.ConsumerTag).Msgf("Consumer not found for recovered message, requeuing")
				vh.requeueMessage(record)
			}
		}
	}
	return nil
}

func (vh *VHost) requeueMessage(record *DeliveryRecord) {
	vh.mu.Lock()
	queue := vh.Queues[record.QueueName]
	vh.mu.Unlock()
	if queue != nil {
		vh.handleTTLExpiration(record.Message, queue)
		vh.markAsRedelivered(record.Message.ID)
		queue.Push(record.Message)
		vh.collector.RecordQueueRequeue(queue.Name)
	}
}

func (vh *VHost) markAsRedelivered(msgID string) {
	// Mark message so the next Basic.Deliver will set redelivered=true.
	// Cleared in deliverToConsumer after a successful delivery (one-shot).
	vh.redeliveredMu.Lock()
	vh.redeliveredMessages[msgID] = struct{}{}
	vh.redeliveredMu.Unlock()
}
