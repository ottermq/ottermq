package management

import (
	"fmt"
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/rs/zerolog/log"
)

func (s *Service) PublishMessage(req models.PublishMessageRequest) error {
	vh := s.broker.GetVHost(req.VHost)
	if vh == nil {
		return fmt.Errorf("vhost '%s' not found", req.VHost)
	}
	exchange := vh.GetExchange(req.ExchangeName)
	routingKey := req.RoutingKey
	mandatory := req.Mandatory

	payload := []byte(req.Payload)
	props := getPropsFromRequest(req)
	hasRouting, err := vh.HasRoutingForMessage(exchange.Name, routingKey)

	if err != nil {
		return err
	}
	if !hasRouting {
		if mandatory {
			log.Debug().Str("exchange", exchange.Name).Str("routing_key", routingKey).Msg("No route for message, returned to publisher")
			return fmt.Errorf("message returned: no route for exchange '%s' with routing key '%s'", exchange.Name, routingKey)
		}
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

func (s *Service) GetMessages(vhostName, queue string, count int, ackMode string) ([]models.MessageDTO, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}

	noAck := ackMode == "noack"

	msgCount, err := vh.GetMessageCount(queue)
	if err != nil {
		return nil, err
	}
	var msg *vhost.Message
	if msgCount > 0 {
		msg = vh.GetMessage(queue)
	}

	if msgCount == 0 || msg == nil {
		return []models.MessageDTO{}, nil
	}

	channelKey := vhost.ConnectionChannelKey{
		// ConnectionName: "management",
		Connection: nil,
		Channel:    0,
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

	return []models.MessageDTO{dto}, nil
}

func propertiesToMap(props amqp.BasicProperties) map[string]any {
	result := make(map[string]any)
	result["ContentType"] = props.ContentType
	result["ContentEncoding"] = props.ContentEncoding
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
