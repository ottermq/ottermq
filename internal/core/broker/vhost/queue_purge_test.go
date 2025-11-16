package vhost

import (
	"net"
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	amqperr "github.com/andrelcunha/ottermq/internal/core/amqp/errors"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
)

func TestPurgeQueue_DeletesPersistentMessagesFromPersistence(t *testing.T) {
	fp := &dummy.DummyPersistence{
		DeletedMessages: []string{}, // Enable simple tracking
	}
	var options = VHostOptions{
		QueueBufferSize: 100,
		Persistence:     fp,
	}
	vh := NewVhost("/", options)

	// Create a durable queue
	_, err := vh.CreateQueue("q1", &QueueProperties{Durable: true}, nil)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}

	// Enqueue 3 messages: 2 persistent, 1 non-persistent
	vh.Queues["q1"].Push(Message{ID: "m1", Body: []byte("a"), Properties: amqp.BasicProperties{DeliveryMode: amqp.PERSISTENT}})
	vh.Queues["q1"].Push(Message{ID: "m2", Body: []byte("b"), Properties: amqp.BasicProperties{DeliveryMode: amqp.NON_PERSISTENT}})
	vh.Queues["q1"].Push(Message{ID: "m3", Body: []byte("c"), Properties: amqp.BasicProperties{DeliveryMode: amqp.PERSISTENT}})

	if got := vh.Queues["q1"].Len(); got != 3 {
		t.Fatalf("precondition: expected 3 messages in queue, got %d", got)
	}

	purged, err := vh.PurgeQueue("q1", nil)
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
	if len(fp.DeletedMessages) != 2 {
		t.Fatalf("expected 2 persisted deletions, got %d (%v)", len(fp.DeletedMessages), fp.DeletedMessages)
	}
	// Order is FIFO; expect m1 then m3
	if fp.DeletedMessages[0] != "m1" || fp.DeletedMessages[1] != "m3" {
		t.Fatalf("unexpected deletion order/ids: %v", fp.DeletedMessages)
	}
}

func TestPurgeQueue_QueueNotFoundReturnsAMQPError(t *testing.T) {
	options := VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := NewVhost("/", options)

	_, err := vh.PurgeQueue("does-not-exist", nil)
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

func TestPurgeQueue_ExclusiveQueueWrongConnectionReturnsAccessRefused(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := NewVhost("/", options)

	// Mock connections
	ownerConn := &mockConn{}
	otherConn := &mockConn{}

	// Create exclusive queue with owner connection
	_, err := vh.CreateQueue("exclusive-q", &QueueProperties{Exclusive: true}, ownerConn)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}

	// Try to purge from different connection
	_, err = vh.PurgeQueue("exclusive-q", otherConn)
	if err == nil {
		t.Fatalf("expected error when purging exclusive queue from wrong connection")
	}

	amqpErr, ok := err.(amqperr.AMQPError)
	if !ok {
		t.Fatalf("expected AMQPError, got %T (%v)", err, err)
	}
	if amqpErr.ReplyCode() != uint16(amqp.ACCESS_REFUSED) {
		t.Errorf("expected ACCESS_REFUSED, got %d", amqpErr.ReplyCode())
	}
	if amqpErr.ClassID() != uint16(amqp.QUEUE) {
		t.Errorf("expected class QUEUE, got %d", amqpErr.ClassID())
	}
	if amqpErr.MethodID() != uint16(amqp.QUEUE_PURGE) {
		t.Errorf("expected method QUEUE_PURGE, got %d", amqpErr.MethodID())
	}

	// Owner should be able to purge
	_, err = vh.PurgeQueue("exclusive-q", ownerConn)
	if err != nil {
		t.Fatalf("owner connection should be able to purge: %v", err)
	}
}

// mockConn for testing
type mockConn struct {
	net.Conn
}
