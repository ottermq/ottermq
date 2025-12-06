package vhost

import (
	"testing"
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/pkg/metrics"
)

// TestMaxLengthIntegration tests the full flow of enforcing max-length before pushing
func TestMaxLengthIntegration(t *testing.T) {
	// Create VHost with max-length enabled
	vh := &VHost{
		Name:    "test",
		Queues:  make(map[string]*Queue),
		persist: &MockPersistence{},
		ActiveExtensions: map[string]bool{
			"qll": true,
		},
		collector: metrics.NewMockCollector(nil),
	}
	vh.QueueLengthLimiter = &DefaultQueueLengthLimiter{}
	vh.QueueLengthLimiter.(*DefaultQueueLengthLimiter).vh = vh

	// Create queue with max-length=5
	queue := NewQueue("test-queue", 100, vh)
	queue.maxLength = 5
	queue.vh = vh
	queue.Props = &QueueProperties{Arguments: make(QueueArgs)}
	vh.Queues["test-queue"] = queue

	// Push 10 messages using the Push method (which calls EnforceMaxLength)
	for i := 0; i < 10; i++ {
		msg := Message{
			ID:         string(rune('a' + i)),
			Body:       []byte{byte('0' + i)},
			EnqueuedAt: time.Now(),
			Properties: amqp.BasicProperties{},
		}
		queue.Push(msg)
	}

	// Queue should have exactly 5 messages (the newest ones)
	if queue.Len() != 5 {
		t.Errorf("Expected queue length 5, got %d", queue.Len())
	}

	// Verify we have the correct messages ('f' through 'j')
	for i := 0; i < 5; i++ {
		msg := queue.Pop()
		if msg == nil {
			t.Fatalf("Expected message at position %d, got nil", i)
		}
		expectedID := string(rune('f' + i))
		if msg.ID != expectedID {
			t.Errorf("Expected message ID '%s', got '%s'", expectedID, msg.ID)
		}
	}

	// Queue should be empty now
	if queue.Len() != 0 {
		t.Errorf("Expected queue to be empty, got length %d", queue.Len())
	}
}

// TestMaxLengthWithDLX tests max-length with dead letter exchange
func TestMaxLengthWithDLX(t *testing.T) {
	// Create VHost
	vh := &VHost{
		Name:      "test",
		Queues:    make(map[string]*Queue),
		Exchanges: make(map[string]*Exchange),
		persist:   &MockPersistence{},
		ActiveExtensions: map[string]bool{
			"qll": true,
			"dlx": true,
		},
		collector: metrics.NewMockCollector(nil),
	}
	vh.QueueLengthLimiter = &DefaultQueueLengthLimiter{vh: vh}
	vh.DeadLetterer = &DeadLetter{vh: vh}

	// Create DLX exchange and queue
	dlx := &Exchange{
		Name: "dlx",
		Typ:  DIRECT,
		Props: &ExchangeProperties{
			Internal: false,
		},
		Bindings: map[string][]*Binding{
			"evicted": {},
		},
	}
	vh.Exchanges["dlx"] = dlx

	dlq := NewQueue("dlq", 100, vh)
	dlq.vh = vh
	dlq.Props = &QueueProperties{Arguments: make(QueueArgs)}
	vh.Queues["dlq"] = dlq

	// Bind DLQ to DLX
	dlx.Bindings["evicted"] = []*Binding{{Queue: dlq, RoutingKey: "evicted"}}

	// Create main queue with max-length=3 and DLX
	mainQueue := NewQueue("main-queue", 100, vh)
	mainQueue.maxLength = 3
	mainQueue.vh = vh
	mainQueue.Props = &QueueProperties{
		Arguments: QueueArgs{
			"x-dead-letter-exchange":    "dlx",
			"x-dead-letter-routing-key": "evicted",
			"x-max-length":              int32(3),
		},
	}
	vh.Queues["main-queue"] = mainQueue

	// Push 6 messages
	for i := range 6 {
		msg := Message{
			ID:         string(rune('a' + i)),
			Body:       []byte{byte('0' + i)},
			EnqueuedAt: time.Now(),
			Properties: amqp.BasicProperties{},
		}
		mainQueue.Push(msg)
	}

	// Main queue should have 3 newest messages
	if mainQueue.Len() != 3 {
		t.Errorf("Expected main queue length 3, got %d", mainQueue.Len())
	}

	// DLQ should have 3 oldest evicted messages
	if dlq.Len() != 3 {
		t.Errorf("Expected DLQ length 3, got %d", dlq.Len())
	}

	// Verify main queue has newest messages ('d', 'e', 'f')
	for i := range 3 {
		msg := mainQueue.Pop()
		if msg == nil {
			t.Fatalf("Expected message in main queue at position %d, got nil", i)
		}
		expectedID := string(rune('d' + i))
		if msg.ID != expectedID {
			t.Errorf("Expected main queue message ID '%s', got '%s'", expectedID, msg.ID)
		}
	}

	// Verify DLQ has oldest messages ('a', 'b', 'c')
	for i := range 3 {
		msg := dlq.Pop()
		if msg == nil {
			t.Fatalf("Expected message in DLQ at position %d, got nil", i)
		}
		expectedID := string(rune('a' + i))
		if msg.ID != expectedID {
			t.Errorf("Expected DLQ message ID '%s', got '%s'", expectedID, msg.ID)
		}
		// Verify x-death headers
		if msg.Properties.Headers == nil {
			t.Error("Expected x-death headers in DLQ message")
		}
	}
}
