package vhost

import (
	"net"
	"sync"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
)

type ChannelTransactionState struct {
	mu                sync.Mutex
	InTransaction     bool
	BufferedPublishes []BufferedPublish
	BufferedAcks      []BufferedAck
}

type BufferedPublish struct {
	ExchangeName string
	RoutingKey   string
	Message      amqp.Message
	Mandatory    bool
}

type BufferedAck struct {
	Operation   AckOperation // "ACK", "NACK", or "REJECT"
	DeliveryTag uint64
	Multiple    bool
	Requeue     bool // Only for NACK and REJECT
}

type AckOperation string

const (
	AckOperationAck    AckOperation = "ACK"
	AckOperationNack   AckOperation = "NACK"
	AckOperationReject AckOperation = "REJECT"
)

func (ct *ChannelTransactionState) Lock() {
	ct.mu.Lock()
}

func (ct *ChannelTransactionState) Unlock() {
	ct.mu.Unlock()
}

func (vh *VHost) GetOrCreateTransactionState(channel uint16, conn net.Conn) *ChannelTransactionState {
	key := ConnectionChannelKey{
		Connection: conn,
		Channel:    channel,
	}

	vh.mu.Lock()
	defer vh.mu.Unlock()

	txState, exists := vh.ChannelTransactions[key]
	if !exists {
		txState = &ChannelTransactionState{}
		vh.ChannelTransactions[key] = txState
	}
	return txState
}

func (vh *VHost) GetTransactionState(channel uint16, conn net.Conn) *ChannelTransactionState {
	key := ConnectionChannelKey{
		Connection: conn,
		Channel:    channel,
	}

	vh.mu.Lock()
	defer vh.mu.Unlock()

	txState, exists := vh.ChannelTransactions[key]
	if !exists {
		return nil
	}
	return txState
}
