package testutil

import (
	"context"
	"net"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
)

// MockFramer is a configurable test double for amqp.Framer
type MockFramer struct {
	SentFrames [][]byte // Records all sent frames
	SendError  error    // If non-nil, SendFrame returns this error
}

func (m *MockFramer) ReadFrame(conn net.Conn) ([]byte, error) { return nil, nil }

func (m *MockFramer) SendFrame(conn net.Conn, frame []byte) error {
	if m.SendError != nil {
		return m.SendError
	}
	m.SentFrames = append(m.SentFrames, frame)
	return nil
}

func (m *MockFramer) Handshake(configurations *map[string]any, conn net.Conn, connCtxt context.Context) (*amqp.ConnectionInfo, error) {
	return nil, nil
}

func (m *MockFramer) ParseFrame(frame []byte) (any, error) { return nil, nil }

func (m *MockFramer) CreateHeaderFrame(channel, classID uint16, msg amqp.Message) []byte {
	return []byte("header-frame")
}

func (m *MockFramer) CreateBodyFrame(channel uint16, content []byte) []byte {
	return []byte("body-frame")
}

func (m *MockFramer) CreateBasicReturnFrame(channel uint16, replyCode uint16, replyText, exchange, routingKey string) []byte {
	return []byte("basic-return")
}

func (m *MockFramer) CreateBasicQosOkFrame(channel uint16) []byte {
	return []byte("basic-qos-ok")
}

func (m *MockFramer) CreateBasicDeliverFrame(channel uint16, consumerTag, exchange, routingKey string, deliveryTag uint64, redelivered bool) []byte {
	return []byte("basic-deliver")
}

func (m *MockFramer) CreateBasicGetEmptyFrame(channel uint16) []byte {
	return []byte("basic-get-empty")
}

func (m *MockFramer) CreateBasicGetOkFrame(channel uint16, exchange, routingkey string, msgCount uint32, deliveryTag uint64, redelivered bool) []byte {
	return []byte("basic-get-ok")
}

func (m *MockFramer) CreateBasicConsumeOkFrame(channel uint16, consumerTag string) []byte {
	return []byte("basic-consume-ok:" + consumerTag)
}

func (m *MockFramer) CreateBasicCancelOkFrame(channel uint16, consumerTag string) []byte {
	return []byte("basic-cancel-ok:" + consumerTag)
}

func (m *MockFramer) CreateBasicRecoverOkFrame(channel uint16) []byte {
	return []byte("basic-recover-ok")
}

func (m *MockFramer) CreateQueueDeclareOkFrame(channel uint16, queueName string, messageCount, consumerCount uint32) []byte {
	return []byte("queue-declare")
}

func (m *MockFramer) CreateQueueBindOkFrame(channel uint16) []byte {
	return []byte("queue-bind-ok")
}

func (m *MockFramer) CreateQueueUnbindOkFrame(channel uint16) []byte {
	return []byte("queue-unbind-ok")
}

func (m *MockFramer) CreateQueueDeleteOkFrame(channel uint16, messageCount uint32) []byte {
	return []byte("queue-delete-ok")
}

func (m *MockFramer) CreateQueuePurgeOkFrame(channel uint16, messageCount uint32) []byte {
	return []byte("queue-purge-ok")
}

func (m *MockFramer) CreateExchangeDeclareFrameOk(channel uint16) []byte {
	return []byte("exchange-declare")
}

func (m *MockFramer) CreateExchangeDeleteFrame(channel uint16) []byte {
	return []byte("exchange-delete")
}

func (m *MockFramer) CreateChannelOpenOkFrame(channel uint16) []byte {
	return []byte("channel-open-ok")
}

func (m *MockFramer) CreateChannelFlowFrame(channel uint16, active bool) []byte {
	return []byte("channel-flow")
}

func (m *MockFramer) CreateChannelFlowOkFrame(channel uint16, active bool) []byte {
	return []byte("channel-flow-ok")
}

func (m *MockFramer) CreateChannelCloseFrame(channel, replyCode, classID, methodID uint16, replyText string) []byte {
	return []byte("channel-close")
}

func (m *MockFramer) CreateChannelCloseOkFrame(channel uint16) []byte {
	return []byte("channel-close-ok")
}

func (m *MockFramer) CreateConnectionCloseOkFrame(channel uint16) []byte {
	return []byte("connection-close-ok")
}

func (m *MockFramer) CreateConnectionCloseFrame(channel, replyCode, classID, methodID uint16, replyText string) []byte {
	return []byte("connection-close")
}

func (m *MockFramer) CreateTxSelectOkFrame(channel uint16) []byte {
	return []byte("tx-select-ok")
}

func (m *MockFramer) CreateTxCommitOkFrame(channel uint16) []byte {
	return []byte("tx-commit-ok")
}

func (m *MockFramer) CreateTxRollbackOkFrame(channel uint16) []byte {
	return []byte("tx-rollback-ok")
}

// MockFrameSender is a test double for vhost.FrameSender interface
type MockFrameSender struct {
	SentFrames []SentFrame
	SendError  error
}

type SentFrame struct {
	ConnID  string
	Channel uint16
	Frame   []byte
}

// SendFrame implements the FrameSender interface
// Uses interface{} for connID to avoid importing vhost package
func (m *MockFrameSender) SendFrame(connID interface{}, channel uint16, frame []byte) error {
	if m.SendError != nil {
		return m.SendError
	}
	// Convert connID to string for recording
	connIDStr := ""
	if connID != nil {
		// Use Sprintf to convert any type to string safely
		connIDStr = string(connID.(interface{ String() string }).String())
	}
	m.SentFrames = append(m.SentFrames, SentFrame{
		ConnID:  connIDStr,
		Channel: channel,
		Frame:   frame,
	})
	return nil
}
