package vhost

import (
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	amqperr "github.com/andrelcunha/ottermq/internal/core/amqp/errors"
	"github.com/andrelcunha/ottermq/pkg/persistence"
)

// fakePersistence records DeleteMessage calls for assertions
type fakePersistence struct {
	deleted []string
}

func (f *fakePersistence) SaveQueueMetadata(vhost, name string, props persistence.QueueProperties) error {
	return nil
}
func (f *fakePersistence) LoadQueueMetadata(vhost, name string) (persistence.QueueProperties, error) {
	return persistence.QueueProperties{}, nil
}
func (f *fakePersistence) DeleteQueueMetadata(vhost, name string) error { return nil }
func (f *fakePersistence) SaveExchangeMetadata(vhost, name, exchangeType string, props persistence.ExchangeProperties) error {
	return nil
}
func (f *fakePersistence) LoadExchangeMetadata(vhost, name string) (string, persistence.ExchangeProperties, error) {
	return "", persistence.ExchangeProperties{}, nil
}
func (f *fakePersistence) DeleteExchangeMetadata(vhost, name string) error { return nil }
func (f *fakePersistence) SaveBindingState(vhost, exchange, queue, routingKey string, arguments map[string]any) error {
	return nil
}
func (f *fakePersistence) LoadExchangeBindings(vhost, exchange string) ([]persistence.BindingData, error) {
	return nil, nil
}
func (f *fakePersistence) DeleteBindingState(vhost, exchange, queue, routingKey string, arguments map[string]any) error {
	return nil
}
func (f *fakePersistence) SaveMessage(vhost, queue, msgId string, msgBody []byte, msgProps persistence.MessageProperties) error {
	return nil
}
func (f *fakePersistence) LoadMessages(vhostName, queueName string) ([]persistence.Message, error) {
	return nil, nil
}
func (f *fakePersistence) DeleteMessage(vhost, queue, msgId string) error {
	f.deleted = append(f.deleted, msgId)
	return nil
}
func (f *fakePersistence) LoadAllExchanges(vhost string) ([]persistence.ExchangeSnapshot, error) {
	return nil, nil
}
func (f *fakePersistence) LoadAllQueues(vhost string) ([]persistence.QueueSnapshot, error) {
	return nil, nil
}
func (f *fakePersistence) Initialize() error { return nil }
func (f *fakePersistence) Close() error      { return nil }

func TestPurgeQueue_DeletesPersistentMessagesFromPersistence(t *testing.T) {
	fp := &fakePersistence{}
	vh := NewVhost("/", 100, fp)

	// Create a durable queue
	_, err := vh.CreateQueue("q1", &QueueProperties{Durable: true}, nil)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}

	// Enqueue 3 messages: 2 persistent, 1 non-persistent
	vh.Queues["q1"].Push(amqp.Message{ID: "m1", Body: []byte("a"), Properties: amqp.BasicProperties{DeliveryMode: amqp.PERSISTENT}})
	vh.Queues["q1"].Push(amqp.Message{ID: "m2", Body: []byte("b"), Properties: amqp.BasicProperties{DeliveryMode: amqp.NON_PERSISTENT}})
	vh.Queues["q1"].Push(amqp.Message{ID: "m3", Body: []byte("c"), Properties: amqp.BasicProperties{DeliveryMode: amqp.PERSISTENT}})

	if got := vh.Queues["q1"].Len(); got != 3 {
		t.Fatalf("precondition: expected 3 messages in queue, got %d", got)
	}

	purged, err := vh.PurgeQueue("q1")
	if err != nil {
		t.Fatalf("PurgeQueue returned error: %v", err)
	}
	if purged != 3 {
		t.Fatalf("expected purged count 3, got %d", purged)
	}

	if got := vh.Queues["q1"].Len(); got != 0 {
		t.Fatalf("expected queue to be empty after purge, got len=%d", got)
	}

	// Only persistent messages should trigger DeleteMessage
	if len(fp.deleted) != 2 {
		t.Fatalf("expected 2 persisted deletions, got %d (%v)", len(fp.deleted), fp.deleted)
	}
	// Order is FIFO; expect m1 then m3
	if fp.deleted[0] != "m1" || fp.deleted[1] != "m3" {
		t.Fatalf("unexpected deletion order/ids: %v", fp.deleted)
	}
}

func TestPurgeQueue_QueueNotFoundReturnsAMQPError(t *testing.T) {
	vh := NewVhost("/", 100, &fakePersistence{})

	_, err := vh.PurgeQueue("does-not-exist")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	amqpErr, ok := err.(amqperr.AMQPError)
	if !ok {
		t.Fatalf("expected AMQPError, got %T (%v)", err, err)
	}
	if amqpErr.ReplyCode() != uint16(amqp.NOT_FOUND) {
		t.Errorf("expected reply code NOT_FOUND, got %d", amqpErr.ReplyCode())
	}
	if amqpErr.ClassID() != uint16(amqp.QUEUE) {
		t.Errorf("expected class QUEUE, got %d", amqpErr.ClassID())
	}
	if amqpErr.MethodID() != uint16(amqp.QUEUE_PURGE) {
		t.Errorf("expected method QUEUE_PURGE, got %d", amqpErr.MethodID())
	}
}
