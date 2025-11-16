package vhost

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/amqp/errors"
	"github.com/andrelcunha/ottermq/pkg/persistence"
)

type SaveMessageRequest struct {
	Props      *amqp.BasicProperties
	Queue      *Queue
	RoutingKey string
	MsgID      string
	Body       []byte
	MsgProps   persistence.MessageProperties
}

type Message struct {
	ID         string
	EnqueuedAt time.Time
	Body       []byte
	Properties amqp.BasicProperties
	Exchange   string
	RoutingKey string
}

func NewMessage(msg amqp.Message, id string) Message {
	props := msg.Properties

	// Convert relative Expiration (TTL in ms) to absolute timestamp
	// RabbitMQ sends Expiration as a string like "100" meaning 100ms from now
	if props.Expiration != "" {
		if ttlMs, err := strconv.ParseInt(props.Expiration, 10, 64); err == nil && ttlMs > 0 {
			// Convert relative TTL to absolute expiration timestamp
			expiresAt := time.Now().Add(time.Duration(ttlMs) * time.Millisecond)
			props.Expiration = strconv.FormatInt(expiresAt.UnixMilli(), 10)
		}
	}

	return Message{
		ID:         id,
		EnqueuedAt: time.Now().UTC(),
		Body:       msg.Body,
		Properties: props,
		Exchange:   msg.Exchange,
		RoutingKey: msg.RoutingKey,
	}
}
func GenerateMessageId() string {
	return uuid.New().String()
}

func (m *Message) ToAMQPMessage() amqp.Message {
	return amqp.Message{
		Body:       m.Body,
		Properties: m.Properties,
		Exchange:   m.Exchange,
		RoutingKey: m.RoutingKey,
	}
}

func (m *Message) ToPersistence() persistence.Message {
	return persistence.Message{
		ID:         m.ID,
		Body:       m.Body,
		Properties: convertToPersistedProperties(m.Properties),
		EnqueuedAt: m.EnqueuedAt.UnixMilli(),
	}
}

func FromPersistence(msgData persistence.Message) Message {
	msg := Message{
		ID:         msgData.ID,
		EnqueuedAt: time.UnixMilli(msgData.EnqueuedAt),
		Body:       msgData.Body,
		Properties: amqp.BasicProperties{
			ContentType:     amqp.ContentType(msgData.Properties.ContentType),
			ContentEncoding: msgData.Properties.ContentEncoding,
			Headers:         msgData.Properties.Headers,
			DeliveryMode:    amqp.DeliveryMode(msgData.Properties.DeliveryMode),
			Priority:        msgData.Properties.Priority,
			CorrelationID:   msgData.Properties.CorrelationID,
			ReplyTo:         msgData.Properties.ReplyTo,
			Expiration:      msgData.Properties.Expiration,
			MessageID:       msgData.Properties.MessageID,
			Timestamp:       time.Unix(msgData.Properties.Timestamp, 0),
			Type:            msgData.Properties.Type,
			UserID:          msgData.Properties.UserID,
			AppID:           msgData.Properties.AppID,
		},
	}
	return msg
}

func convertToPersistedProperties(amqpProps amqp.BasicProperties) persistence.MessageProperties {
	var timestamp int64
	if amqpProps.Timestamp.IsZero() {
		timestamp = 0
	} else {
		timestamp = amqpProps.Timestamp.Unix()
	}
	msgProps := persistence.MessageProperties{
		ContentType:     string(amqpProps.ContentType),
		ContentEncoding: amqpProps.ContentEncoding,
		Headers:         amqpProps.Headers,
		DeliveryMode:    uint8(amqpProps.DeliveryMode),
		Priority:        amqpProps.Priority,
		CorrelationID:   amqpProps.CorrelationID,
		ReplyTo:         amqpProps.ReplyTo,
		Expiration:      amqpProps.Expiration,
		MessageID:       amqpProps.MessageID,
		Timestamp:       timestamp,
		Type:            amqpProps.Type,
		UserID:          amqpProps.UserID,
		AppID:           amqpProps.AppID,
	}
	return msgProps
}

func (vh *VHost) Publish(exchangeName, routingKey string, msg *Message) (string, error) {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	exchange, ok := vh.Exchanges[exchangeName]
	if !ok {
		log.Error().Str("exchange", exchangeName).Msg("Exchange not found")
		return "", errors.NewChannelError(fmt.Sprintf("no exchange '%s' in vhost '%s'", exchangeName, vh.Name), uint16(amqp.NOT_FOUND), uint16(amqp.BASIC), uint16(amqp.BASIC_PUBLISH))
	}

	// verify if exchange is internal
	if exchange.Props.Internal {
		// TODO: raise channel exception (403)
		return "", fmt.Errorf("cannot publish to internal exchange %s", exchangeName)
	}

	log.Trace().Str("id", msg.ID).Str("exchange", exchangeName).Str("routing_key", routingKey).Str("body", string(msg.Body)).Interface("properties", msg.Properties).Msg("Publishing message")

	msgProps := convertToPersistedProperties(msg.Properties)
	switch exchange.Typ {
	case DIRECT:
		bindings, ok := exchange.Bindings[routingKey]
		if !ok {
			log.Error().Str("routing_key", routingKey).Str("exchange", exchangeName).Msg("Routing key not found for exchange")
			return "", fmt.Errorf("routing key %s not found for exchange %s", routingKey, exchangeName)
		}
		for _, binding := range bindings {
			queue := binding.Queue
			err := vh.saveMessageIfDurable(SaveMessageRequest{
				Props:      &msg.Properties,
				Queue:      queue,
				RoutingKey: routingKey,
				MsgID:      msg.ID,
				Body:       msg.Body,
				MsgProps:   msgProps,
			})
			if err != nil {
				return "", err
			}

			queue.Push(*msg)
		}
		return msg.ID, nil

	case FANOUT:
		routingKey = ""
		bindings, ok := exchange.Bindings[routingKey]
		if !ok {
			log.Error().Str("routing_key", routingKey).Str("exchange", exchangeName).Msg("Routing key not found for exchange")
			return "", fmt.Errorf("routing key %s not found for exchange %s", routingKey, exchangeName)
		}
		for _, binding := range bindings {
			queue := binding.Queue
			err := vh.saveMessageIfDurable(SaveMessageRequest{
				Props:      &msg.Properties,
				Queue:      queue,
				RoutingKey: routingKey,
				MsgID:      msg.ID,
				Body:       msg.Body,
				MsgProps:   msgProps,
			})
			if err != nil {
				return "", err
			}

			queue.Push(*msg)
		}
		return msg.ID, nil
	case TOPIC:
		matchedQueues := make(map[*Queue]struct{})

		for bindingKey, bindings := range exchange.Bindings {
			if MatchTopic(routingKey, bindingKey) {
				for _, binding := range bindings {
					matchedQueues[binding.Queue] = struct{}{}
				}
			}
		}

		if len(matchedQueues) == 0 {
			log.Debug().Str("routing_key", routingKey).
				Str("exchange", exchangeName).
				Msg("No matching bindings for routing key")
			return msg.ID, nil
		}

		for queue := range matchedQueues {
			err := vh.saveMessageIfDurable(SaveMessageRequest{
				Props:      &msg.Properties,
				Queue:      queue,
				RoutingKey: routingKey,
				MsgID:      msg.ID,
				Body:       msg.Body,
				MsgProps:   msgProps,
			})
			if err != nil {
				return "", err
			}

			queue.Push(*msg)
		}
		return msg.ID, nil
	default:
		return "", fmt.Errorf("exchange type '%s' not supported yet", exchange.Typ)
	}
}

// func (vh *Broker) GetMessage(queueName string) <-chan Message {
func (vh *VHost) GetMessage(queueName string) *Message {
	vh.mu.Lock()
	queue, ok := vh.Queues[queueName]
	if !ok {
		vh.mu.Unlock()
		log.Error().Str("queue", queueName).Msg("Queue not found")
		return nil
	}
	msg := queue.Pop()
	if msg == nil {
		vh.mu.Unlock()
		log.Debug().Str("queue", queueName).Msg("No messages in queue")
		return nil
	}
	// Must unlock before handleTTLExpiration which may call Publish (DLX)
	vh.mu.Unlock()

	// verify the expiration
	if vh.handleTTLExpiration(*msg, queue) {
		log.Debug().Str("queue", queueName).Str("msg_id", msg.ID).Msg("Message expired upon retrieval")
		return nil
	}
	return msg
}

func (vh *VHost) GetMessageCount(queueName string) (int, error) {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	queue, ok := vh.Queues[queueName]
	if !ok {
		return 0, fmt.Errorf("queue %s not found", queueName)
	}
	return queue.Len(), nil
}

// acknowledge removes the message with the given ID from the unackedMessages map.
func (vh *VHost) Acknowledge(consumerID, msgID string) error {
	panic("Not implemented")
}

func (vh *VHost) saveMessageIfDurable(req SaveMessageRequest) error {
	if req.Props.DeliveryMode == amqp.PERSISTENT { // Persistent
		if req.Queue.Props.Durable {
			// Persist the message
			if vh.persist == nil {
				log.Error().Msg("Persistence layer is not initialized")
				return fmt.Errorf("persistence layer is not initialized")
			}
			if err := vh.persist.SaveMessage(vh.Name, req.Queue.Name, req.MsgID, req.Body, req.MsgProps); err != nil {
				log.Error().Err(err).Msg("Failed to save message to file")
				return err
			}
		}
	}
	return nil
}

func (vh *VHost) HasRoutingForMessage(exchangeName, routingKey string) (bool, error) {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	exchange, ok := vh.Exchanges[exchangeName]
	if !ok {
		return false, errors.NewChannelError(fmt.Sprintf("no exchange '%s' in vhost '%s'", exchangeName, vh.Name), uint16(amqp.NOT_FOUND), uint16(amqp.QUEUE), uint16(amqp.QUEUE_BIND))
	}

	switch exchange.Typ {
	case DIRECT:
		// For direct exchanges, check if there are bindings for the specific routing key
		queues, ok := exchange.Bindings[routingKey]
		return ok && len(queues) > 0, nil
	case FANOUT:
		// For fanout exchanges, check if there are any bound queues
		// Use unlocked version since we already hold the lock
		return len(vh.listFanoutQueuesUnlocked(exchange)) > 0, nil
	case TOPIC:
		for bindingKey := range exchange.Bindings {
			if MatchTopic(routingKey, bindingKey) {
				queues := exchange.Bindings[bindingKey]
				if len(queues) > 0 {
					return true, nil
				}
			}
		}
		return false, nil
	default:
		return false, nil
	}
}

func (vh *VHost) handleTTLExpiration(msg Message, q *Queue) bool {
	expired := false
	if vh.ActiveExtensions["ttl"] && vh.TTLManager != nil {
		// Verify message expiration before delivery
		expired, err := vh.TTLManager.CheckExpiration(&msg, q)
		if err != nil && err != ErrNoTTLConfigured {
			log.Error().Err(err).Str("queue", q.Name).Str("msg_id", msg.ID).Msg("Error checking message expiration")
		}
		if expired {
			// verify if dlx argument is configured
			ok := vh.handleDeadLetter(q, msg, REASON_EXPIRED)
			if !ok {
				return expired
			}

			// Skip delivery
			// message is discarded due to expiration
			log.Debug().Str("queue", q.Name).Str("id", msg.ID).Msg("Message expired, discarding")
			// Should we persistently delete the message here?
			vh.deleteMessage(msg, q)
			return expired
		}
	}
	return expired
}

func (vh *VHost) deleteMessage(msg Message, q *Queue) {
	if msg.Properties.DeliveryMode == amqp.PERSISTENT {
		if vh.persist != nil {
			if err := vh.persist.DeleteMessage(vh.Name, q.Name, msg.ID); err != nil {
				log.Error().Err(err).Str("queue", q.Name).Str("msg_id", msg.ID).Msg("Failed to delete persisted message")
			} else {
				log.Debug().Str("queue", q.Name).Str("msg_id", msg.ID).Msg("Deleted persisted message")
			}
		}
	}
}
