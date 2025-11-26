package broker

import (
	"fmt"
	"net"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/amqp/errors"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/rs/zerolog/log"
)

func (b *Broker) queueHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	switch request.MethodID {
	case uint16(amqp.QUEUE_DECLARE):
		return b.queueDeclareHandler(request, vh, conn)

	case uint16(amqp.QUEUE_BIND):
		return b.queueBindHandler(request, vh, conn)

	case uint16(amqp.QUEUE_PURGE):
		return b.queuePurgeHandler(request, vh, conn)

	case uint16(amqp.QUEUE_DELETE):
		return b.queueDeleteHandler(request, vh, conn)

	case uint16(amqp.QUEUE_UNBIND):
		return b.queueUnbindHandler(request, vh, conn)

	default:
		return nil, fmt.Errorf("unsupported command")
	}
}

func (b *Broker) queueDeleteHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	log.Debug().Interface("request", request).Msg("Received queue delete request")
	content, ok := request.Content.(*amqp.QueueDeleteMessage)
	if !ok {
		log.Error().Msg("Invalid content type for QueueDeleteMessage")
		return nil, fmt.Errorf("invalid content type for QueueDeleteMessage")
	}
	log.Debug().Interface("content", content).Msg("Content")
	queueName := content.QueueName

	queue, exists := vh.Queues[queueName]
	if !exists {
		return sendChannelErrorResponse(
			errors.NewChannelError(
				fmt.Sprintf("no queue '%s' in vhost '%s'", queueName, vh.Name),
				uint16(amqp.NOT_FOUND),
				uint16(amqp.QUEUE),
				uint16(amqp.QUEUE_DELETE),
			),
			b, conn, request,
		)
	}

	// Exclusive queues can only be deleted by their owning connection
	connID := vhost.ConnectionID(GenerateConnectionID(conn))
	if queue.Props.Exclusive && queue.OwnerConn != connID {
		return sendChannelErrorResponse(
			errors.NewChannelError(
				fmt.Sprintf("queue '%s' is exclusive to another connection", queueName),
				uint16(amqp.ACCESS_REFUSED),
				uint16(amqp.QUEUE),
				uint16(amqp.QUEUE_DELETE),
			),
			b, conn, request,
		)
	}

	// Honor if-unused flag
	activeConsumers := vh.GetActiveConsumersForQueue(queueName)
	consumerCount := uint32(len(activeConsumers))
	if content.IfUnused && consumerCount > 0 {
		return sendChannelErrorResponse(
			errors.NewChannelError(
				fmt.Sprintf("queue '%s' in use - has %d consumers", queueName, consumerCount),
				uint16(amqp.PRECONDITION_FAILED),
				uint16(amqp.QUEUE),
				uint16(amqp.QUEUE_DELETE),
			),
			b, conn, request,
		)
	}

	// Honor if-empty flag
	messageCount := uint32(queue.Len())
	if content.IfEmpty && messageCount > 0 {
		return sendChannelErrorResponse(
			errors.NewChannelError(
				fmt.Sprintf("queue '%s' not empty - has %d messages", queueName, messageCount),
				uint16(amqp.PRECONDITION_FAILED),
				uint16(amqp.QUEUE),
				uint16(amqp.QUEUE_DELETE),
			),
			b, conn, request,
		)
	}

	err := vh.DeleteQueuebyName(queueName)
	if err != nil {
		return sendChannelErrorResponse(err, b, conn, request)
	}

	// Honor no-wait flag
	if !content.NoWait {
		frame := b.framer.CreateQueueDeleteOkFrame(request.Channel, messageCount)
		if err := b.framer.SendFrame(conn, frame); err != nil {
			log.Error().Err(err).Msg("Failed to send queue.delete-ok frame")
		}
	}
	return nil, nil
}

func (b *Broker) queuePurgeHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	content, ok := request.Content.(*amqp.QueuePurgeMessage)
	if !ok {
		log.Error().Msg("Invalid content type for QueuePurgeMessage")
		return nil, fmt.Errorf("invalid content type for QueuePurgeMessage")
	}
	log.Debug().Interface("content", content).Msg("Content")
	queueName := content.QueueName
	var messageCount uint32 = 0
	connID := vhost.ConnectionID(GenerateConnectionID(conn))
	messageCount, err := vh.PurgeQueue(queueName, connID)
	if err != nil {
		return sendChannelErrorResponse(err, b, conn, request)
	}

	// Honor no-wait flag
	if !content.NoWait {
		frame := b.framer.CreateQueuePurgeOkFrame(request.Channel, messageCount)
		if err := b.framer.SendFrame(conn, frame); err != nil {
			log.Error().Err(err).Msg("Failed to send queue.purge-ok frame")
			return nil, err
		}
		log.Debug().Str("queue_name", queueName).Uint32("message_count", messageCount).Msg("Sent Queue.PurgeOk frame")
	}

	return nil, nil
}

func (b *Broker) queueBindHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	content, ok := request.Content.(*amqp.QueueBindMessage)
	if !ok {
		log.Error().Msg("Invalid content type for QueueBindMessage")
		return nil, fmt.Errorf("invalid content type for QueueBindMessage")
	}
	log.Debug().Interface("content", content).Msg("Content")
	queue := content.Queue
	exchange := content.Exchange
	routingKey := content.RoutingKey
	args := content.Arguments

	connID := vhost.ConnectionID(GenerateConnectionID(conn))
	err := vh.BindQueue(exchange, queue, routingKey, args, connID)
	if err != nil {
		return sendChannelErrorResponse(err, b, conn, request)
	}
	frame := b.framer.CreateQueueBindOkFrame(request.Channel)
	if err := b.framer.SendFrame(conn, frame); err != nil {
		log.Error().Err(err).Msg("Failed to send queue bind ok frame")
	}
	return nil, nil
}

func (b *Broker) queueUnbindHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	content, ok := request.Content.(*amqp.QueueUnbindMessage)
	if !ok {
		log.Error().Msg("Invalid content type for QueueUnbindMessage")
		return nil, fmt.Errorf("invalid content type for QueueUnbindMessage")
	}
	queue := content.Queue
	exchange := content.Exchange
	routingKey := content.RoutingKey
	args := content.Arguments

	connID := vhost.ConnectionID(GenerateConnectionID(conn))
	err := vh.UnbindQueue(exchange, queue, routingKey, args, connID)
	if err != nil {
		return sendChannelErrorResponse(err, b, conn, request)
	}
	frame := b.framer.CreateQueueUnbindOkFrame(request.Channel)
	if err := b.framer.SendFrame(conn, frame); err != nil {
		log.Error().Err(err).Msg("Failed to send queue unbind ok frame")
	}
	return nil, nil
}

func (b *Broker) queueDeclareHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	log.Debug().Interface("request", request).Msg("Received queue declare request")
	content, ok := request.Content.(*amqp.QueueDeclareMessage)
	if !ok {
		log.Error().Msg("Invalid content type for QueueDeclareMessage")
		return nil, fmt.Errorf("invalid content type for QueueDeclareMessage")
	}

	queueName := content.QueueName
	connID := vhost.ConnectionID(GenerateConnectionID(conn))
	queue, err := vh.CreateQueue(queueName, &vhost.QueueProperties{
		Passive:    content.Passive,
		Durable:    content.Durable,
		AutoDelete: content.AutoDelete,
		Exclusive:  content.Exclusive,
		Arguments:  content.Arguments,
	}, connID)
	if err != nil {
		return sendChannelErrorResponse(err, b, conn, request)
	}

	err = vh.BindToDefaultExchange(queueName)
	if err != nil {
		log.Debug().Err(err).Msg("Error binding to default exchange")
		return nil, err
	}
	messageCount := uint32(queue.Len())
	consumerCount := uint32(0)

	if !content.NoWait {
		frame := b.framer.CreateQueueDeclareOkFrame(request.Channel, queueName, messageCount, consumerCount)
		if err := b.framer.SendFrame(conn, frame); err != nil {
			log.Error().Err(err).Msg("Failed to send queue declare frame")
		}
	}
	return nil, nil
}

func sendChannelErrorResponse(err error, b *Broker, conn net.Conn, request *amqp.RequestMethodMessage) (any, error) {
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
