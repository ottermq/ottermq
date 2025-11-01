package broker

import (
	"fmt"
	"net"

	"github.com/andrelcunha/ottermq/internal/amqp/errors"
	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// basicHandler handles the AMQP basic class methods. Receives the request method (from newState.MethodFrame) vhost and connection
func (b *Broker) basicHandler(newState *amqp.ChannelState, vh *vhost.VHost, conn net.Conn) (any, error) {
	request := newState.MethodFrame
	switch request.MethodID {
	case uint16(amqp.BASIC_QOS):
		return b.basicQoSHandler(request, conn, vh)

	case uint16(amqp.BASIC_CONSUME):
		return b.basicConsumeHandler(request, conn, vh)

	case uint16(amqp.BASIC_CANCEL):
		return b.basicCancelHandler(request, conn, vh)

	case uint16(amqp.BASIC_PUBLISH):
		return b.basicPublishHandler(newState, conn, vh)

	case uint16(amqp.BASIC_GET):
		getMsg := request.Content.(*amqp.BasicGetMessageContent)
		queue := getMsg.Queue
		msgCount, err := vh.GetMessageCount(queue)
		if err != nil {
			log.Error().Err(err).Msg("Error getting message count")
			return nil, err
		}
		if msgCount == 0 {
			frame := b.framer.CreateBasicGetEmptyFrame(request.Channel)
			if err := b.framer.SendFrame(conn, frame); err != nil {
				log.Error().Err(err).Msg("Failed to send basic get empty frame")
			}
			return nil, nil
		}

		// Send Basic.GetOk + header + body
		msg := vh.GetMessage(queue)

		frame := b.framer.CreateBasicGetOkFrame(request.Channel, msg.Exchange, msg.RoutingKey, uint32(msgCount))
		err = b.framer.SendFrame(conn, frame)
		log.Debug().Str("queue", queue).Str("id", msg.ID).Msg("Sent message from queue")

		if err != nil {
			log.Debug().Err(err).Msg("Error sending frame")
			return nil, err
		}

		responseContent := amqp.ResponseContent{
			Channel: request.Channel,
			ClassID: request.ClassID,
			Weight:  0,
			Message: *msg,
		}
		// Header
		frame = responseContent.FormatHeaderFrame()
		if err := b.framer.SendFrame(conn, frame); err != nil {
			log.Error().Err(err).Msg("Failed to send header frame")
		}
		// Body
		frame = responseContent.FormatBodyFrame()
		if err := b.framer.SendFrame(conn, frame); err != nil {
			log.Error().Err(err).Msg("Failed to send body frame")
		}
		return nil, nil

	case uint16(amqp.BASIC_ACK):
		return b.basicAckHandler(request, conn, vh)

	case uint16(amqp.BASIC_REJECT):
		return b.basicRejectHandler(request, conn, vh)

	case uint16(amqp.BASIC_RECOVER_ASYNC):
		return b.basicRecoverHandler(request, conn, vh, true)

	case uint16(amqp.BASIC_RECOVER):
		return b.basicRecoverHandler(request, conn, vh, false)

	case uint16(amqp.BASIC_NACK):
		return b.basicNackHandler(request, conn, vh)

	default:
		return nil, fmt.Errorf("unsupported command")
	}
}

func (b *Broker) basicQoSHandler(request *amqp.RequestMethodMessage, conn net.Conn, vh *vhost.VHost) (any, error) {
	// Do nothing for now, just log the request and send the basic.QosOk
	content, ok := request.Content.(*amqp.BasicQosContent)
	if !ok || content == nil {
		return nil, fmt.Errorf("invalid basic.qos content")
	}

	prefetchSize := content.PrefetchSize // should be 0, if raize channel error 540 - not implemented
	if prefetchSize != 0 {
		errCode := uint16(540)
		log.Warn().Uint16("error code", errCode).Msgf("not implemented - this server does not support prefetch size > 0")
	}

	prefetchCount := content.PrefetchCount
	global := content.Global

	if err := vh.HandleBasicQos(conn, request.Channel, prefetchCount, global); err != nil {
		log.Error().Err(err).Msg("Failed to handle basic.qos")
	}

	frame := b.framer.CreateBasicQosOkFrame(request.Channel)
	if err := b.framer.SendFrame(conn, frame); err != nil {
		log.Error().Err(err).Msg("Failed to send basic.qos-ok frame")
	}
	return nil, nil

}

func (b *Broker) basicPublishHandler(newState *amqp.ChannelState, conn net.Conn, vh *vhost.VHost) (any, error) {
	request := newState.MethodFrame
	channel := request.Channel
	currentState := b.getCurrentState(conn, channel)
	if currentState == nil {
		return nil, fmt.Errorf("channel not found")
	}
	if currentState.MethodFrame != request { // request is "newState.MethodFrame"
		b.Connections[conn].Channels[channel].MethodFrame = request
		log.Trace().Interface("state", b.getCurrentState(conn, channel)).Msg("Current state after update method")
		return nil, nil
	}
	// if the class and method are not the same as the current state,
	// it means that it stated the new publish request
	if currentState.HeaderFrame == nil && newState.HeaderFrame != nil {
		b.Connections[conn].Channels[channel].HeaderFrame = newState.HeaderFrame
		b.Connections[conn].Channels[channel].BodySize = newState.HeaderFrame.BodySize
		log.Trace().Interface("state", b.getCurrentState(conn, channel)).Msg("Current state after update header")
		return nil, nil
	}
	if currentState.Body == nil && newState.Body != nil {
		log.Trace().Int("body_len", len(newState.Body)).Uint64("expected", currentState.BodySize).Msg("Updating body")
		// Append the new body to the current body
		b.Connections[conn].Channels[channel].Body = newState.Body
	}
	log.Trace().Interface("state", currentState).Msg("Current state after all")
	if currentState.MethodFrame.Content != nil && currentState.HeaderFrame != nil && currentState.BodySize > 0 && currentState.Body != nil {
		log.Trace().Interface("state", currentState).Msg("All fields must be filled")
		if len(currentState.Body) != int(currentState.BodySize) {
			log.Trace().Int("body_len", len(currentState.Body)).Uint64("expected", currentState.BodySize).Msg("Body size is not correct")
			// TODO: handle this error properly, maybe sending the correct channel exception
			// vide amqp.constants.go Exceptions
			return nil, fmt.Errorf("body size is not correct: %d != %d", len(currentState.Body), currentState.BodySize)
		}
		publishRequest := currentState.MethodFrame.Content.(*amqp.BasicPublishContent)
		exchange := publishRequest.Exchange
		routingKey := publishRequest.RoutingKey
		mandatory := publishRequest.Mandatory
		// immediate := publishRequest.Immediate // immediate is deprecated and should be ignored

		body := currentState.Body
		props := currentState.HeaderFrame.Properties
		hasRouting, err := vh.HasRoutingForMessage(exchange, routingKey)
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

		msgID := uuid.New().String()
		msg := &amqp.Message{
			ID:         msgID,
			Body:       body,
			Properties: *props,
			Exchange:   exchange,
			RoutingKey: routingKey,
		}

		if !hasRouting {
			if mandatory {
				// Return message to the publisher
				log.Debug().Str("exchange", exchange).Str("routing_key", routingKey).Msg("No route for message, returned to publisher")
				return b.BasicReturn(conn, channel, exchange, routingKey, msg)
			}
			// No routing and not mandatory - silently drop the message
			log.Debug().Str("exchange", exchange).Str("routing_key", routingKey).Msg("No route for message, silently dropped (not mandatory)")
			b.Connections[conn].Channels[channel] = &amqp.ChannelState{}
			return nil, nil
		}

		_, err = vh.Publish(exchange, routingKey, msg)
		if err == nil {
			log.Trace().Str("exchange", exchange).Str("routing_key", routingKey).Str("body", string(body)).Msg("Published message")
			b.Connections[conn].Channels[channel] = &amqp.ChannelState{}
		}
		return nil, err

	}
	return nil, nil
}

func (b *Broker) basicCancelHandler(request *amqp.RequestMethodMessage, conn net.Conn, vh *vhost.VHost) (any, error) {
	content, ok := request.Content.(*amqp.BasicCancelContent)
	if !ok || content == nil {
		return nil, fmt.Errorf("invalid basic cancel content")
	}

	consumerTag := content.ConsumerTag
	err := vh.CancelConsumer(request.Channel, consumerTag)
	if err != nil {
		log.Error().Err(err).Str("consumer_tag", consumerTag).Msg("Failed to cancel consumer")
		return nil, err
	}

	if !content.NoWait {
		frame := b.framer.CreateBasicCancelOkFrame(request.Channel, consumerTag)
		if err := b.framer.SendFrame(conn, frame); err != nil {
			log.Error().Err(err).Msg("Failed to send basic cancel ok frame")
			return nil, err
		}
		log.Debug().Str("consumer_tag", consumerTag).Msg("Sent Basic.CancelOk frame")
	}
	return nil, nil
}

func (b *Broker) basicConsumeHandler(request *amqp.RequestMethodMessage, conn net.Conn, vh *vhost.VHost) (any, error) {
	// get properties from request: queue, consumer tag, ❌noLocal, noAck, exclusive, ❌nowait, arguments
	content, ok := request.Content.(*amqp.BasicConsumeContent)
	if !ok || content == nil {
		return nil, fmt.Errorf("invalid basic consume content")
	}

	queueName := content.Queue
	consumerTag := content.ConsumerTag
	noAck := content.NoAck
	exclusive := content.Exclusive
	noWait := content.NoWait
	arguments := content.Arguments

	// no-local means that the server will not deliver messages to the consumer
	//  that were published on the same connection. We would need to track the connection
	//  that published the message to implement this feature.
	//  RabbitMQ does not implement it either.

	// If tag is empty, generate a random one
	consumer := vhost.NewConsumer(conn, request.Channel, queueName, consumerTag, &vhost.ConsumerProperties{
		NoAck:     noAck,
		Exclusive: exclusive,
		Arguments: arguments,
	})

	err := vh.RegisterConsumer(consumer)
	if err != nil {
		log.Error().Err(err).Msg("Failed to register consumer")
		// It would be a 500-like error
		return nil, err
	}

	if !noWait {
		frame := b.framer.CreateBasicConsumeOkFrame(request.Channel, consumerTag)
		if err := b.framer.SendFrame(conn, frame); err != nil {
			log.Error().Err(err).Msg("Failed to send basic consume ok frame")
			// Verify if should return error (as channel exception) or just log it
			// Maybe we should return a custom error, representing a channel exception
			//  and handle it in the caller function (processRequest)
			// This channel exception would have some fields like ClassID, MethodID, ReplyCode, ReplyText
			// This approach would be used in the other handlers as well
			return nil, err
		}
		log.Debug().Str("consumer_tag", consumerTag).Msg("Sent Basic.ConsumeOk frame")
	}
	return nil, nil
}

func (b *Broker) basicAckHandler(request *amqp.RequestMethodMessage, conn net.Conn, vh *vhost.VHost) (any, error) {
	content, ok := request.Content.(*amqp.BasicAckContent)
	if !ok || content == nil {
		return nil, fmt.Errorf("invalid basic ack content")
	}
	err := vh.HandleBasicAck(conn, request.Channel, content.DeliveryTag, content.Multiple)
	if err != nil {
		log.Error().Err(err).Msg("Failed to acknowledge message")
		return nil, err
	}
	log.Debug().Uint64("delivery_tag", content.DeliveryTag).Msg("Acknowledged message")
	return nil, nil
}

func (b *Broker) basicRejectHandler(request *amqp.RequestMethodMessage, conn net.Conn, vh *vhost.VHost) (any, error) {
	content, ok := request.Content.(*amqp.BasicRejectContent)
	if !ok || content == nil {
		return nil, fmt.Errorf("invalid basic reject content")
	}
	err := vh.HandleBasicReject(conn, request.Channel, content.DeliveryTag, content.Requeue)
	if err != nil {
		log.Error().Err(err).Msg("Failed to reject message")
		return nil, err
	}
	log.Debug().Uint64("delivery_tag", content.DeliveryTag).Msg("Rejected message")
	return nil, nil
}

func (b *Broker) basicRecoverHandler(request *amqp.RequestMethodMessage, conn net.Conn, vh *vhost.VHost, async bool) (any, error) {
	content, ok := request.Content.(*amqp.BasicRecoverContent)
	if !ok || content == nil {
		return nil, fmt.Errorf("invalid basic recover content")
	}
	err := vh.HandleBasicRecover(conn, request.Channel, content.Requeue)
	if err != nil {
		log.Error().Err(err).Msg("Failed to recover messages")
		return nil, err
	}
	log.Debug().Msg("Recovered messages")
	if async {
		return nil, nil
	}

	frame := b.framer.CreateBasicRecoverOkFrame(request.Channel)
	if err := b.framer.SendFrame(conn, frame); err != nil {
		log.Error().Err(err).Msg("Failed to send basic recover ok frame")
		return nil, err
	}
	log.Debug().Msg("Sent Basic.RecoverOk frame")
	return nil, nil
}

func (b *Broker) BasicReturn(conn net.Conn, channel uint16, exchange, routingKey string, msg *amqp.Message) (any, error) {
	// TODO: implement basic.return
	frame := b.framer.CreateBasicReturnFrame(channel, 312, "No route", exchange, routingKey)
	if err := b.framer.SendFrame(conn, frame); err != nil {
		log.Error().Err(err).Msg("Failed to send basic return frame")
		return nil, err
	}
	log.Debug().Str("exchange", exchange).Str("routing_key", routingKey).Msg("Sent Basic.Return frame")

	responseContent := amqp.ResponseContent{
		Channel: channel,
		ClassID: uint16(amqp.BASIC),
		Weight:  0,
		Message: *msg,
	}
	// Header
	frame = responseContent.FormatHeaderFrame()
	if err := b.framer.SendFrame(conn, frame); err != nil {
		log.Error().Err(err).Msg("Failed to send header frame")
	}
	// Body
	frame = responseContent.FormatBodyFrame()
	if err := b.framer.SendFrame(conn, frame); err != nil {
		log.Error().Err(err).Msg("Failed to send body frame")
	}
	return nil, nil
}

func (b *Broker) basicNackHandler(request *amqp.RequestMethodMessage, conn net.Conn, vh *vhost.VHost) (any, error) {
	content, ok := request.Content.(*amqp.BasicNackContent)
	if !ok || content == nil {
		return nil, fmt.Errorf("invalid basic nack content")
	}
	err := vh.HandleBasicNack(conn, request.Channel, content.DeliveryTag, content.Multiple, content.Requeue)
	if err != nil {
		log.Error().Err(err).Msg("Failed to reject message")
		return nil, err
	}
	log.Debug().Uint64("delivery_tag", content.DeliveryTag).Msg("Rejected message")
	return nil, nil
}
