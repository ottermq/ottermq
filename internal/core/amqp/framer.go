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

	CreateQueueDeclareOkFrame(channel uint16, queueName string, messageCount, consumerCount uint32) []byte
	CreateQueueBindOkFrame(channel uint16) []byte
	CreateQueueUnbindOkFrame(channel uint16) []byte
	CreateQueueDeleteOkFrame(channel uint16, messageCount uint32) []byte
	CreateQueuePurgeOkFrame(channel uint16, messageCount uint32) []byte

	// Exchange Methods

	CreateExchangeDeclareFrame(channel uint16) []byte
	CreateExchangeDeleteFrame(channel uint16) []byte

	// Channel Methods

	CreateChannelOpenOkFrame(channel uint16) []byte
	CreateChannelCloseFrame(channel, replyCode, classID, methodID uint16, replyText string) []byte
	CreateChannelCloseOkFrame(channel uint16) []byte

	// Connection Methods

	CreateConnectionCloseFrame(channel, replyCode, classID, methodID uint16, replyText string) []byte
	CreateConnectionCloseOkFrame(channel uint16) []byte
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

func (d *DefaultFramer) CreateQueueDeclareOkFrame(channel uint16, queueName string, messageCount, consumerCount uint32) []byte {
	return createQueueDeclareOkFrame(channel, queueName, messageCount, consumerCount)
}

func (d *DefaultFramer) CreateQueueBindOkFrame(channel uint16) []byte {
	return createQueueAckFrame(channel, uint16(QUEUE_BIND_OK))
}

func (d *DefaultFramer) CreateQueueUnbindOkFrame(channel uint16) []byte {
	return createQueueAckFrame(channel, uint16(QUEUE_UNBIND_OK))
}

func (d *DefaultFramer) CreateQueuePurgeOkFrame(channel uint16, messageCount uint32) []byte {
	return createQueueCountAckFrame(channel, uint16(QUEUE_PURGE_OK), messageCount)
}

func (d *DefaultFramer) CreateQueueDeleteOkFrame(channel uint16, messageCount uint32) []byte {
	return createQueueCountAckFrame(channel, uint16(QUEUE_DELETE_OK), messageCount)
}

// ENDREGION

// REGION Exchange Methods

func (d *DefaultFramer) CreateExchangeDeclareFrame(channel uint16) []byte {
	return createExchangeDeclareFrame(channel)
}

func (d *DefaultFramer) CreateExchangeDeleteFrame(channel uint16) []byte {
	return createExchangeDeleteFrame(channel)
}

// ENDREGION

// REGION Channel Methods

func (d *DefaultFramer) CreateChannelOpenOkFrame(channel uint16) []byte {
	return createChannelOpenOkFrame(channel)
}

func (d *DefaultFramer) CreateChannelCloseFrame(channel, replyCode, classID, methodID uint16, replyText string) []byte {
	return createChannelCloseFrame(channel, replyCode, classID, methodID, replyText)
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

func (d *DefaultFramer) CreateConnectionCloseOkFrame(channel uint16) []byte {
	return createConnectionCloseOkFrame(channel)
}
