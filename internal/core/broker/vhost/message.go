package vhost

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/andrelcunha/ottermq/internal/amqp/errors"
	"github.com/andrelcunha/ottermq/internal/core/amqp"
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

// TODO: create a higher level abstraction of amqp.Message, exposing the content, requeued count, etc

func (vh *VHost) Publish(exchangeName, routingKey string, msg *amqp.Message) (string, error) {
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

	var timestamp int64
	if msg.Properties.Timestamp.IsZero() {
		timestamp = 0
	} else {
		timestamp = msg.Properties.Timestamp.Unix()
	}
	msgProps := persistence.MessageProperties{
		ContentType:     string(msg.Properties.ContentType),
		ContentEncoding: msg.Properties.ContentEncoding,
		Headers:         msg.Properties.Headers,
		DeliveryMode:    uint8(msg.Properties.DeliveryMode),
		Priority:        msg.Properties.Priority,
		CorrelationID:   msg.Properties.CorrelationID,
		ReplyTo:         msg.Properties.ReplyTo,
		Expiration:      msg.Properties.Expiration,
		MessageID:       msg.Properties.MessageID,
		Timestamp:       timestamp,
		Type:            msg.Properties.Type,
		UserID:          msg.Properties.UserID,
		AppID:           msg.Properties.AppID,
	}
	switch exchange.Typ {
	case DIRECT:
		queues, ok := exchange.Bindings[routingKey]
		if !ok {
			log.Error().Str("routing_key", routingKey).Str("exchange", exchangeName).Msg("Routing key not found for exchange")
			return "", fmt.Errorf("routing key %s not found for exchange %s", routingKey, exchangeName)
		}
		for _, queue := range queues {
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
		for _, queue := range exchange.Queues {
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
		// TODO: Implement topic exchange routing
		return "", fmt.Errorf("topic exchange not yet implemented")
	default:
		return "", fmt.Errorf("unknown exchange type")
	}
}

// func (vh *Broker) GetMessage(queueName string) <-chan Message {
func (vh *VHost) GetMessage(queueName string) *amqp.Message {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	queue, ok := vh.Queues[queueName]
	if !ok {
		log.Error().Str("queue", queueName).Msg("Queue not found")
		return nil
	}
	msg := queue.Pop()
	if msg == nil {
		log.Debug().Str("queue", queueName).Msg("No messages in queue")
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

func (vh *VHost) HasRoutingForMessage(exchangeName, routingKey string) bool {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	exchange, ok := vh.Exchanges[exchangeName]
	if !ok {
		return false
	}

	switch exchange.Typ {
	case DIRECT:
		// For direct exchanges, check if there are bindings for the specific routing key
		queues, ok := exchange.Bindings[routingKey]
		return ok && len(queues) > 0
	case FANOUT:
		// For fanout exchanges, check if there are any bound queues
		return len(exchange.Queues) > 0
	case TOPIC:
		// TODO: Implement topic routing check (needs pattern matching)
		return false
	default:
		return false
	}
}
