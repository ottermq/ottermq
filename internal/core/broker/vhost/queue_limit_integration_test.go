package vhost

import (
	"testing"
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
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
