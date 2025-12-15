package management

import (
	"fmt"
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/rs/zerolog/log"
)

func (s *Service) PublishMessage(vhostName, exchangeName string, req models.PublishMessageRequest) error {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return fmt.Errorf("vhost '%s' not found", vhostName)
	}
	exchange := vh.GetExchange(exchangeName)
	routingKey := req.RoutingKey

	payload := []byte(req.Payload)
	props := getPropsFromRequest(req)
	hasRouting, err := vh.HasRoutingForMessage(exchange.Name, routingKey)

	if err != nil {
		return err
	}
	if !hasRouting {
		log.Debug().Str("exchange", exchange.Name).Str("routing_key", routingKey).Msg("No route for message, message dropped")
		return nil
	}
	msgID := vhost.GenerateMessageId()
	msg := vhost.Message{
		ID:         msgID,
		EnqueuedAt: time.Now().UTC(),
		Body:       payload,
		Properties: props,
		Exchange:   exchange.Name,
		RoutingKey: routingKey,
	}
	_, err = vh.Publish(exchange.Name, routingKey, &msg)
	return err
}

func (s *Service) GetMessages(vhostName, queue string, count int, ackMode models.AckType) ([]models.MessageDTO, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}
	if queue == "" {
		return nil, fmt.Errorf("queue name cannot be empty")
	}
	if count <= 0 {
		return nil, fmt.Errorf("message count must be greater than zero")
	}

	noAck := ackMode == models.NoAck

	msgCount, err := vh.GetMessageCount(queue)
	if err != nil {
		return nil, err
	}
	log.Debug().Str("queue", queue).Int("requested_count", count).Int("available_count", msgCount).Msg("Getting messages from queue")
	msgs := make([]models.MessageDTO, 0, count)
	if count > msgCount {
		count = msgCount
	}

	for i := 0; i < count; i++ {
		msg := vh.GetMessage(queue, noAck)

		if msgCount == 0 || msg == nil {
			return []models.MessageDTO{}, nil
		}

		channelKey := vhost.ConnectionChannelKey{
			ConnectionID: vhost.MANAGEMENT_CONNECTION_ID,
			Channel:      0,
		}
		ch := vh.GetOrCreateChannelDelivery(channelKey)

		deliveryTag := ch.TrackDelivery(noAck, msg, queue)

		redelivered := vh.ShouldRedeliver(msg.ID)

		dto := models.MessageDTO{
			ID:          msg.ID,
			Payload:     msg.Body,
			Properties:  propertiesToMap(msg.Properties),
			DeliveryTag: deliveryTag,
			Redelivered: redelivered,
		}
		msgs = append(msgs, dto)
	}

	return msgs, nil
}

func propertiesToMap(props amqp.BasicProperties) map[string]any {
	result := make(map[string]any)
	result["ContentType"] = string(props.ContentType)
	result["ContentEncoding"] = string(props.ContentEncoding)
	result["Headers"] = props.Headers
	result["DeliveryMode"] = fmt.Sprintf("%d", props.DeliveryMode)
	result["Priority"] = fmt.Sprintf("%d", props.Priority)
	result["CorrelationId"] = props.CorrelationID
	result["ReplyTo"] = props.ReplyTo
	result["Expiration"] = props.Expiration
	result["MessageId"] = props.MessageID
	result["Timestamp"] = props.Timestamp.String()
	result["Type"] = props.Type
	result["UserId"] = props.UserID
	result["AppId"] = props.AppID
	return result
}

func getPropsFromRequest(req models.PublishMessageRequest) amqp.BasicProperties {
	if req.DeliveryMode < 1 || req.DeliveryMode > 2 {
		req.DeliveryMode = 1 // default to transient
	}
	deliveryMode := amqp.DeliveryMode(req.DeliveryMode)

	props := amqp.BasicProperties{
		ContentType:     amqp.ContentType(req.ContentType),
		ContentEncoding: req.ContentEncoding,
		Headers:         req.Headers,
		DeliveryMode:    deliveryMode,
		Priority:        req.Priority,
		CorrelationID:   req.CorrelationId,
		ReplyTo:         req.ReplyTo,
		Expiration:      req.Expiration,
		MessageID:       req.MessageID,
		Type:            req.Type,
		UserID:          req.UserId,
		AppID:           req.AppId,
	}
	if req.Timestamp != nil {
		props.Timestamp = *req.Timestamp
	}
	return props
}
