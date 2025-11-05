package vhost

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/amqp/errors"
	"github.com/andrelcunha/ottermq/pkg/persistence"
	"github.com/rs/zerolog/log"
)

type QueueArgs map[string]any

type Queue struct {
	Name      string            `json:"name"`
	Props     *QueueProperties  `json:"properties"`
	messages  chan amqp.Message `json:"-"`
	count     int               `json:"-"`
	mu        sync.Mutex        `json:"-"`
	OwnerConn net.Conn          `json:"-"`
	/* Delivery */
	deliveryCtx    context.Context    `json:"-"`
	deliveryCancel context.CancelFunc `json:"-"`
	delivering     bool
}

type QueueProperties struct {
	Passive    bool      `json:"passive"`
	Durable    bool      `json:"durable"`
	AutoDelete bool      `json:"auto_delete"`
	Exclusive  bool      `json:"exclusive"` // not implemented yet
	Arguments  QueueArgs `json:"arguments"`
}

func NewQueue(name string, bufferSize int) *Queue {

	return &Queue{
		Name:       name,
		Props:      &QueueProperties{},
		messages:   make(chan amqp.Message, bufferSize),
		count:      0,
		delivering: false,
	}
}

func (q *Queue) startDeliveryLoop(vh *VHost) {
	if q.delivering {
		return // already running
	}
	q.deliveryCtx, q.deliveryCancel = context.WithCancel(context.Background())
	q.delivering = true
	go func() {
		for {
			select {
			case <-q.deliveryCtx.Done():
				log.Debug().Str("queue", q.Name).Msg("Stopping delivery loop")
				q.delivering = false
				return
			case msg := <-q.messages:
				q.mu.Lock()
				q.count--
				q.mu.Unlock()

				log.Debug().Str("queue", q.Name).Str("id", msg.ID).Msg("Delivering message to consumers")
				consumers := vh.GetActiveConsumersForQueue(q.Name)

				// No consumers available, requeue and wait
				if len(consumers) == 0 {
					log.Debug().Str("queue", q.Name).Msg("No consumers available, requeuing message")
					q.Push(msg)
					time.Sleep(100 * time.Millisecond)
					continue
				}

				delivered := false
				maxRounds := 100 // Prevent infinite loop
				rounds := 0

				for !delivered && rounds < maxRounds {
					rounds++
					allThrottled := true

					// Try each consumer once per round
					for i := 0; i < len(consumers); i++ {
						consumer := consumers[i]
						state := vh.getChannelDeliveryState(consumer.Connection, consumer.Channel)

						if vh.shouldThrottle(consumer, state) {
							continue // Try next consumer
						}

						// Found available consumer
						allThrottled = false
						if err := vh.deliverToConsumer(consumer, msg, false); err != nil {
							log.Error().Err(err).Str("consumer", consumer.Tag).Msg("Delivery failed, removing consumer")
							if cancelErr := vh.CancelConsumer(consumer.Channel, consumer.Tag); cancelErr != nil {
								log.Error().Err(cancelErr).Str("consumer", consumer.Tag).Msg("Error cancelling consumer")
							}
							// Refresh consumer list and retry
							consumers = vh.GetActiveConsumersForQueue(q.Name)
							if len(consumers) == 0 {
								q.Push(msg)
								delivered = true // Exit loop
							}
							break // Retry with new consumer list
						}
						delivered = true
						break // Successfully delivered
					}

					if allThrottled && !delivered {
						// All consumers are throttled, wait for signal or timeout
						// Try to get a signal from any consumer's channel
						var anyState *ChannelDeliveryState
						for _, c := range consumers {
							if s := vh.getChannelDeliveryState(c.Connection, c.Channel); s != nil {
								anyState = s
								break
							}
						}

						if anyState != nil {
							select {
							case <-anyState.unackedChanged:
								// A slot opened up, retry immediately
								continue
							case <-time.After(1 * time.Second):
								// Timeout, will retry or give up based on maxRounds
								continue
							case <-q.deliveryCtx.Done():
								return
							}
						} else {
							// No state available, just sleep briefly
							time.Sleep(100 * time.Millisecond)
						}
					}
				}

				// If we exhausted retries, requeue the message
				if !delivered {
					log.Warn().Str("queue", q.Name).Int("rounds", rounds).Msg("Could not deliver after max rounds, requeuing")
					q.Push(msg)
				}
			}
		}
	}()
}

func (q *Queue) stopDeliveryLoop() {
	if !q.delivering {
		return
	}
	q.deliveryCancel()
	q.delivering = false
}

func (vh *VHost) CreateQueue(name string, props *QueueProperties, conn net.Conn) (*Queue, error) {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	// Passive declaration: error if queue doesn't exist
	if props != nil && props.Passive {
		queue := vh.Queues[name]
		if queue == nil {
			text := amqp.NOT_FOUND.Format(fmt.Sprintf("no queue '%s' in vhost '%s'", name, vh.Name))
			return nil, errors.NewChannelError(text, uint16(amqp.NOT_FOUND), uint16(amqp.CHANNEL), uint16(amqp.QUEUE_DECLARE))
		} else {
			log.Debug().Str("queue", name).Msg("Passive queue declare: queue exists")
			return queue, nil
		}
	}

	// Queue already exists: validate compatibility
	if existing, ok := vh.Queues[name]; ok {
		if existing.Props == nil || props == nil {
			return nil, fmt.Errorf("queue %s already exists with incompatible properties", name)
		}
		if existing.Props.Durable != props.Durable ||
			existing.Props.AutoDelete != props.AutoDelete ||
			existing.Props.Exclusive != props.Exclusive ||
			!equalArgs(existing.Props.Arguments, props.Arguments) {
			return nil, fmt.Errorf("queue %s already exists with different properties", name)
		}
		log.Debug().Str("queue", name).Msg("Queue already exists with matching properties")
		return existing, nil
	}

	// Create new queue
	if props == nil {
		props = NewQueueProperties()
	}
	queue := NewQueue(name, vh.queueBufferSize)
	queue.Props = props
	if props.Exclusive {
		// Exclusive queues are tied to the connection that declared them
		// This is a placeholder; actual connection should be set during declaration
		queue.OwnerConn = nil
	}
	if props.Exclusive {
		queue.OwnerConn = conn
	}
	vh.Queues[name] = queue

	if props.Durable {
		if err := vh.persist.SaveQueueMetadata(vh.Name, name, props.ToPersistence()); err != nil {
			log.Error().Err(err).Str("queue", name).Msg("Failed to save queue metadata")
		}
	}

	log.Debug().Str("queue", name).Msg("Created queue")
	return queue, nil
}

func NewQueueProperties() *QueueProperties {
	return &QueueProperties{
		Passive:    false,
		Durable:    false,
		AutoDelete: false,
		Exclusive:  false,
		Arguments:  make(map[string]any),
	}
}

func (vh *VHost) DeleteQueue(name string) error {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	return vh.deleteQueueUnlocked(name)
}

func (vh *VHost) deleteQueueUnlocked(name string) error {
	queue, exists := vh.Queues[name]
	if !exists {
		return fmt.Errorf("queue %s not found", name)
	}
	close(queue.messages)
	// verify if there are any bindings to this queue and remove them
	for _, exchange := range vh.Exchanges {
		// Routing keys to bindings
		switch exchange.Typ {
		case DIRECT:
			for rk, bindings := range exchange.Bindings {
				for i, b := range bindings {
					if b.Queue.Name == name {
						exchange.Bindings[rk] = append(bindings[:i], bindings[i+1:]...)
						break
					}
				}
				if len(exchange.Bindings[rk]) == 0 {
					delete(exchange.Bindings, rk)
					// Check if the exchange can be auto-deleted
					if deleted, err := vh.checkAutoDeleteExchangeUnlocked(exchange.Name); err != nil {
						log.Printf("Failed to check auto-delete exchange: %v", err)
					} else if deleted {
						log.Printf("Exchange %s was auto-deleted", exchange.Name)
					}
				}
			}
		case FANOUT:
			bindings := exchange.Bindings[""]
			for i, b := range bindings {
				if b.Queue.Name == name {
					exchange.Bindings[""] = append(bindings[:i], bindings[i+1:]...)
					break
				}
			}
			if len(exchange.Bindings[""]) == 0 {
				delete(exchange.Bindings, "")
				// Check if the exchange can be auto-deleted
				if deleted, err := vh.checkAutoDeleteExchangeUnlocked(exchange.Name); err != nil {
					log.Printf("Failed to check auto-delete exchange: %v", err)
				} else if deleted {
					log.Printf("Exchange %s was auto-deleted", exchange.Name)
				}
			}
		}
	}

	// Remove the queue from the VHost's queue map
	delete(vh.Queues, name)

	log.Debug().Str("queue", name).Msg("Deleted queue")

	// Persistence cleanup
	if queue.Props.Durable {
		if err := vh.persist.DeleteQueueMetadata(vh.Name, name); err != nil {
			log.Error().Err(err).Str("queue", name).Msg("Failed to delete queue from persistence")
			return err
		}
	}
	return nil
}

func (qp *QueueProperties) ToPersistence() persistence.QueueProperties {
	return persistence.QueueProperties{
		Durable:    qp.Durable,
		AutoDelete: qp.AutoDelete,
		Exclusive:  qp.Exclusive,
		Arguments:  qp.Arguments,
	}
}

func (q *Queue) Push(msg amqp.Message) {
	q.mu.Lock()
	defer q.mu.Unlock()
	select {
	case q.messages <- msg:
		q.count++
		log.Debug().Str("queue", q.Name).Str("id", msg.ID).Bytes("body", msg.Body).Msg("Pushed message to queue")
	default:
		log.Debug().Str("queue", q.Name).Str("id", msg.ID).Msg("Queue channel full, dropping message")
	}
}

func (q *Queue) Pop() *amqp.Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	select {
	case msg := <-q.messages:
		q.count--
		log.Debug().Str("queue", q.Name).Str("id", msg.ID).Str("body", string(msg.Body)).Msg("Popped message from queue")
		return &msg
	default:
		log.Debug().Str("queue", q.Name).Msg("Queue is empty")
		return nil
	}
}

func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.count
}
