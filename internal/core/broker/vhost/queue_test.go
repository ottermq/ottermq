package vhost

import (
	"fmt"
	"net"
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
)

func TestQueueCapacityExceeds1000(t *testing.T) {
	// Create a test queue with a buffer size of 100000
	queue := NewQueue("testQueue", 100000)

	// Try to push more than 1000 messages
	t.Logf("Starting to push 2000 messages to queue...")

	for i := range 2000 {
		msg := amqp.Message{
			ID:   fmt.Sprintf("msg-%d", i),
			Body: []byte(fmt.Sprintf("Test message %d", i)),
		}

		// Push the message
		queue.Push(msg)
	}

	finalCount := queue.Len()
	t.Logf("Final queue length: %d", finalCount)

	if finalCount <= 1000 {
		t.Errorf("FAIL: Queue is still limited to 1000 messages (actual: %d)", finalCount)
	} else {
		t.Logf("SUCCESS: Queue can now hold more than 1000 messages! (actual: %d)", finalCount)
	}

	// Verify we can hold at least 2000 messages
	if finalCount < 2000 {
		t.Errorf("Expected queue to hold 2000 messages, got %d", finalCount)
	}
}

func TestDeleteQueue_AutoDeleteDirectExchange(t *testing.T) {
	vh := &VHost{
		Exchanges: make(map[string]*Exchange),
		Queues:    make(map[string]*Queue),
		persist:   &dummy.DummyPersistence{},
	}
	// Create auto-delete direct exchange
	ex := &Exchange{
		Name:     "ex_direct",
		Typ:      DIRECT,
		Bindings: make(map[string][]*Binding),
		Props:    &ExchangeProperties{AutoDelete: true},
	}
	vh.Exchanges["ex_direct"] = ex
	// Create queue and bind (use NewQueue to avoid nil channel)
	q := NewQueue("q_direct", 10)
	vh.Queues["q_direct"] = q
	ex.Bindings["rk"] = []*Binding{{Queue: q}}

	// Delete the queue
	err := vh.DeleteQueuebyName("q_direct")
	if err != nil {
		t.Fatalf("DeleteQueue failed: %v", err)
	}
	// Exchange should be auto-deleted
	if _, exists := vh.Exchanges["ex_direct"]; exists {
		t.Errorf("Expected direct exchange to be auto-deleted, but it still exists")
	}
}

func TestDeleteQueue_AutoDeleteFanoutExchange(t *testing.T) {
	vh := &VHost{
		Exchanges: make(map[string]*Exchange),
		Queues:    make(map[string]*Queue),
		persist:   &dummy.DummyPersistence{},
	}
	// Create auto-delete fanout exchange
	ex := &Exchange{
		Name:     "ex_fanout",
		Typ:      FANOUT,
		Bindings: make(map[string][]*Binding),
		Props:    &ExchangeProperties{AutoDelete: true},
	}
	vh.Exchanges["ex_fanout"] = ex
	// Create queue and bind (use NewQueue to avoid nil channel)
	q := NewQueue("q_fanout", 10)
	vh.Queues["q_fanout"] = q
	ex.Bindings[""] = []*Binding{{Queue: q}}

	// Delete the queue
	err := vh.DeleteQueuebyName("q_fanout")
	if err != nil {
		t.Fatalf("DeleteQueue failed: %v", err)
	}
	// Exchange should be auto-deleted
	if _, exists := vh.Exchanges["ex_fanout"]; exists {
		t.Errorf("Expected fanout exchange to be auto-deleted, but it still exists")
	}
}

func TestExclusiveQueueDeletedOnConnectionClose(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := NewVhost("/", options)
	// conn := &mockConn{}
	conn := net.Conn(nil)

	// Create exclusive queue
	queue, err := vh.CreateQueue("exclusive-q", &QueueProperties{
		Exclusive: true,
	}, conn)
	if err != nil {
		t.Fatalf("Failed to create exclusive queue: %v", err)
	}

	// Verify owner is set
	if queue.OwnerConn != conn {
		t.Error("Expected OwnerConn to be set")
	}

	// Close connection
	vh.CleanupConnection(conn)

	// Verify queue was deleted
	if _, exists := vh.Queues["exclusive-q"]; exists {
		t.Error("Expected exclusive queue to be deleted on connection close")
	}
}
