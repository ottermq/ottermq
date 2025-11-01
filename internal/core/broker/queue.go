package broker

import (
	"fmt"
	"net"

	"github.com/andrelcunha/ottermq/internal/amqp/errors"
	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/rs/zerolog/log"
)

func (b *Broker) queueHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	switch request.MethodID {
	case uint16(amqp.QUEUE_DECLARE):
		return queueDeclareHandler(request, vh, b, conn)

	case uint16(amqp.QUEUE_BIND):
		log.Debug().Interface("request", request).Msg("Received queue bind request")
		content, ok := request.Content.(*amqp.QueueBindMessage)
		if !ok {
			log.Error().Msg("Invalid content type for QueueBindMessage")
			return nil, fmt.Errorf("invalid content type for QueueBindMessage")
		}
		log.Debug().Interface("content", content).Msg("Content")
		queue := content.Queue
		exchange := content.Exchange
		routingKey := content.RoutingKey

		err := vh.BindQueue(exchange, queue, routingKey)
		if err != nil {
			log.Debug().Err(err).Msg("Error binding to exchange")
			return nil, err
		}
		frame := b.framer.CreateQueueBindOkFrame(request)
		if err := b.framer.SendFrame(conn, frame); err != nil {
			log.Error().Err(err).Msg("Failed to send queue bind ok frame")
		}
		return nil, nil

	case uint16(amqp.QUEUE_DELETE):
		log.Debug().Interface("request", request).Msg("Received queue delete request")
		content, ok := request.Content.(*amqp.QueueDeleteMessage)
		if !ok {
			log.Error().Msg("Invalid content type for QueueDeleteMessage")
			return nil, fmt.Errorf("invalid content type for QueueDeleteMessage")
		}
		log.Debug().Interface("content", content).Msg("Content")
		queueName := content.QueueName

		// Get queue object and message count before deletion
		queue, exists := vh.Queues[queueName]
		if !exists {
			return nil, fmt.Errorf("queue %s does not exist", queueName)
		}
		messageCount := uint32(queue.Len())
		// consumerCount := uint32(0)
		// if cc, ok := interface{}(queue).(interface{ ConsumerCount() int }); ok {
		// 	consumerCount = uint32(cc.ConsumerCount())
		// }
		// TODO: Implement the following flags: if-unused, if-empty

		// // Honor if-empty flag
		// if content.IfEmpty && messageCount > 0 {
		// 	return nil, fmt.Errorf("queue %s not empty", queueName)
		// }
		// // Honor if-unused flag
		// if content.IfUnused && consumerCount > 0 {
		// 	return nil, fmt.Errorf("queue %s is in use", queueName)
		// }

		err := vh.DeleteQueue(queueName)
		if err != nil {
			return nil, err
		}

		// Honor no-wait flag
		if !content.NoWait {
			frame := b.framer.CreateQueueDeleteOkFrame(request, messageCount)
			if err := b.framer.SendFrame(conn, frame); err != nil {
				log.Error().Err(err).Msg("Failed to send queue delete ok frame")
			}
		}
		return nil, nil

	case uint16(amqp.QUEUE_UNBIND):
		return nil, fmt.Errorf("not implemented")

	default:
		return nil, fmt.Errorf("unsupported command")
	}
}

func queueDeclareHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, b *Broker, conn net.Conn) (any, error) {
	log.Debug().Interface("request", request).Msg("Received queue declare request")
	content, ok := request.Content.(*amqp.QueueDeclareMessage)
	if !ok {
		log.Error().Msg("Invalid content type for ExchangeDeclareMessage")
		return nil, fmt.Errorf("invalid content type for ExchangeDeclareMessage")
	}
	log.Debug().Interface("content", content).Msg("Content")
	queueName := content.QueueName

	queue, err := vh.CreateQueue(queueName, &vhost.QueueProperties{
		Passive:    content.Passive,
		Durable:    content.Durable,
		AutoDelete: content.AutoDelete,
		Exclusive:  content.Exclusive,
		Arguments:  content.Arguments,
	})
	if err != nil {
		if amqpErr, ok := err.(errors.AMQPError); ok {
			b.sendChannelClosing(conn,
				request.Channel,
				amqpErr.ReplyCode(),
				amqpErr.ClassID(),
				amqpErr.MethodID(),
				amqpErr.ReplyText(),
			)
			return nil, nil
		}
		return nil, err
	}

	err = vh.BindToDefaultExchange(queueName)
	if err != nil {
		log.Debug().Err(err).Msg("Error binding to default exchange")
		return nil, err
	}
	messageCount := uint32(queue.Len())
	consumerCount := uint32(0)

	if !content.NoWait {
		frame := b.framer.CreateQueueDeclareOkFrame(request, queueName, messageCount, consumerCount)
		if err := b.framer.SendFrame(conn, frame); err != nil {
			log.Error().Err(err).Msg("Failed to send queue declare frame")
		}
	}
	return nil, nil
}
