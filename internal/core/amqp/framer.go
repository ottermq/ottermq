package amqp

import (
	"context"
	"net"
)

type Framer interface {
	ReadFrame(conn net.Conn) ([]byte, error)
	SendFrame(conn net.Conn, frame []byte) error
	Handshake(configurations *map[string]any, conn net.Conn, connCtxt context.Context) (*ConnectionInfo, error)
	ParseFrame(frame []byte) (any, error)
	CreateHeaderFrame(channel, classID uint16, msg Message) []byte
	CreateBodyFrame(channel uint16, content []byte) []byte

	// Basic Methods

	CreateBasicQosOkFrame(channel uint16) []byte
	CreateBasicDeliverFrame(channel uint16, consumerTag, exchange, routingKey string, deliveryTag uint64, redelivered bool) []byte
	CreateBasicReturnFrame(channel uint16, replyCode uint16, replyText, exchange, routingKey string) []byte
	CreateBasicGetEmptyFrame(channel uint16) []byte
	CreateBasicGetOkFrame(channel uint16, exchange, routingkey string, msgCount uint32) []byte
	CreateBasicConsumeOkFrame(channel uint16, consumerTag string) []byte
	CreateBasicCancelOkFrame(channel uint16, consumerTag string) []byte
	CreateBasicRecoverOkFrame(channel uint16) []byte

	// Queue Methods

	CreateQueueDeclareOkFrame(request *RequestMethodMessage, queueName string, messageCount, consumerCount uint32) []byte
	CreateQueueBindOkFrame(request *RequestMethodMessage) []byte
	CreateQueueDeleteOkFrame(request *RequestMethodMessage, messageCount uint32) []byte

	// Exchange Methods

	CreateExchangeDeclareFrame(request *RequestMethodMessage) []byte
	CreateExchangeDeleteFrame(request *RequestMethodMessage) []byte

	// Channel Methods

	CreateChannelOpenOkFrame(request *RequestMethodMessage) []byte
	CreateChannelCloseFrame(channel, replyCode, classID, methodID uint16, replyText string) []byte
	CreateChannelCloseOkFrame(channel uint16) []byte

	// Connection Methods

	CreateConnectionCloseFrame(channel, replyCode, classID, methodID uint16, replyText string) []byte
	CreateConnectionCloseOkFrame(request *RequestMethodMessage) []byte
}

type DefaultFramer struct{}

func (d *DefaultFramer) ReadFrame(conn net.Conn) ([]byte, error) {
	return readFrame(conn)
}

func (d *DefaultFramer) SendFrame(conn net.Conn, frame []byte) error {
	return sendFrame(conn, frame)
}

func (d *DefaultFramer) Handshake(configurations *map[string]any, conn net.Conn, connCtxt context.Context) (*ConnectionInfo, error) {
	return handshake(configurations, conn, connCtxt)
}

func (d *DefaultFramer) ParseFrame(frame []byte) (any, error) {
	return parseFrame(frame)
}

func (d *DefaultFramer) CreateHeaderFrame(channel, classID uint16, msg Message) []byte {
	return createHeaderFrame(channel, classID, msg)
}

func (d *DefaultFramer) CreateBodyFrame(channel uint16, content []byte) []byte {
	return createBodyFrame(channel, content)
}

// REGION Queue Methods

func (d *DefaultFramer) CreateQueueDeclareOkFrame(request *RequestMethodMessage, queueName string, messageCount, consumerCount uint32) []byte {
	return createQueueDeclareOkFrame(request, queueName, messageCount, consumerCount)
}

func (d *DefaultFramer) CreateQueueBindOkFrame(request *RequestMethodMessage) []byte {
	return createQueueBindOkFrame(request)
}

func (d *DefaultFramer) CreateQueueDeleteOkFrame(request *RequestMethodMessage, messageCount uint32) []byte {
	return createQueueDeleteOkFrame(request, messageCount)
}

// ENDREGION

// REGION Exchange Methods

func (d *DefaultFramer) CreateExchangeDeclareFrame(request *RequestMethodMessage) []byte {
	return createExchangeDeclareFrame(request)
}

func (d *DefaultFramer) CreateExchangeDeleteFrame(request *RequestMethodMessage) []byte {
	return createExchangeDeleteFrame(request)
}

// ENDREGION

// REGION Channel Methods

func (d *DefaultFramer) CreateChannelOpenOkFrame(request *RequestMethodMessage) []byte {
	return createChannelOpenOkFrame(request)
}

func (d *DefaultFramer) CreateChannelCloseOkFrame(channel uint16) []byte {
	return createChannelCloseOkFrame(channel)
}

// ENDREGION

// REGION Basic Methods

func (d *DefaultFramer) CreateBasicQosOkFrame(channel uint16) []byte {
	return createBasicQosOkFrame(channel)
}

func (d *DefaultFramer) CreateBasicDeliverFrame(channel uint16, consumerTag, exchange, routingKey string, deliveryTag uint64, redelivered bool) []byte {
	return createBasicDeliverFrame(channel, consumerTag, exchange, routingKey, deliveryTag, redelivered)
}

func (d *DefaultFramer) CreateBasicReturnFrame(channel uint16, replyCode uint16, replyText, exchange, routingKey string) []byte {
	return createBasicReturnFrame(channel, replyCode, replyText, exchange, routingKey)
}

func (d *DefaultFramer) CreateBasicConsumeOkFrame(channel uint16, consumerTag string) []byte {
	return createBasicConsumeOkFrame(channel, consumerTag)
}

func (d *DefaultFramer) CreateBasicCancelOkFrame(channel uint16, consumerTag string) []byte {
	return createBasicCancelOkFrame(channel, consumerTag)
}

func (d *DefaultFramer) CreateBasicGetEmptyFrame(channel uint16) []byte {
	return createBasicGetEmptyFrame(channel)
}

func (d *DefaultFramer) CreateBasicGetOkFrame(channel uint16, exchange, routingkey string, msgCount uint32) []byte {
	return createBasicGetOkFrame(channel, exchange, routingkey, msgCount)
}

func (d *DefaultFramer) CreateBasicRecoverOkFrame(channel uint16) []byte {
	return createBasicRecoverOkFrame(channel)
}

// ENDREGION

// REGION Connection Methods

func (d *DefaultFramer) CreateConnectionCloseFrame(channel, replyCode, classID, methodID uint16, replyText string) []byte {
	return createConnectionCloseFrame(channel, replyCode, classID, methodID, replyText)
}

func (d *DefaultFramer) CreateConnectionCloseOkFrame(request *RequestMethodMessage) []byte {
	return createConnectionCloseOkFrame(request)
}
