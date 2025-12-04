package vhost

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
)

func TestPriorityQueue_PushPop_Order(t *testing.T) {
	vh := createTestVHostWithPriority(10)
	queue := createTestQueueWithMaxPriority(vh, "test-priority-queue", 10)

	// Push messages with different priorities
	pushMessageWithPriority(queue, "low", 1)
	pushMessageWithPriority(queue, "high", 9)
	pushMessageWithPriority(queue, "medium", 5)

	// Pop should return in priority order
	assertNextMessage(t, queue, "high")
	assertNextMessage(t, queue, "medium")
	assertNextMessage(t, queue, "low")
}

func TestPriorityQueue_FIFO_WithinPriority(t *testing.T) {
	vh := createTestVHostWithPriority(10)
	queue := createTestQueueWithMaxPriority(vh, "test-queue", 10)

	// Push multiple messages with same priority
	pushMessageWithPriority(queue, "first", 5)
	pushMessageWithPriority(queue, "second", 5)
	pushMessageWithPriority(queue, "third", 5)

	// Should maintain FIFO within priority
	assertNextMessage(t, queue, "first")
	assertNextMessage(t, queue, "second")
	assertNextMessage(t, queue, "third")
}

func TestPriorityQueue_PriorityClamp(t *testing.T) {
	vh := createTestVHostWithPriority(10)
	queue := createTestQueueWithMaxPriority(vh, "test-queue", 5)

	// Push with priority > max
	pushMessageWithPriority(queue, "clamped", 10) // Should clamp to 5
	pushMessageWithPriority(queue, "normal", 3)

	// Clamped message should be delivered first (priority 5)
	assertNextMessage(t, queue, "clamped")
	assertNextMessage(t, queue, "normal")
}

func TestPriorityQueue_NoPriorityDefaultsToZero(t *testing.T) {
	vh := createTestVHostWithPriority(10)
	queue := createTestQueueWithMaxPriority(vh, "test-queue", 10)

	pushMessageWithPriority(queue, "priority-5", 5)
	pushMessageNoPriority(queue, "no-priority") // Should be priority 0

	assertNextMessage(t, queue, "priority-5")
	assertNextMessage(t, queue, "no-priority")
}

func pushMessageNoPriority(queue *Queue, s string) {
	msg := Message{
		ID:         generateID(),
		Body:       []byte(s),
		Properties: amqp.BasicProperties{
			// No priority set
		},
	}
	queue.Push(msg)
}

func TestPriorityQueue_LazyChannelAllocation(t *testing.T) {
	vh := createTestVHostWithPriority(10)
	queue := createTestQueueWithMaxPriority(vh, "test-queue", 10)

	// Initially no channels allocated
	if len(queue.priorityMessages) != 0 {
		t.Errorf("Expected 0 allocated channels, got %d", len(queue.priorityMessages))
	}

	// Push to priority 5
	pushMessageWithPriority(queue, "msg", 5)

	// Should allocate only priority 5 channel
	if len(queue.priorityMessages) != 1 {
		t.Errorf("Expected 1 allocated channel, got %d", len(queue.priorityMessages))
	}
	if _, exists := queue.priorityMessages[5]; !exists {
		t.Error("Expected priority 5 channel to be allocated")
	}
}

func TestPriorityQueue_Purge(t *testing.T) {
	vh := createTestVHostWithPriority(10)
	queue := createTestQueueWithMaxPriority(vh, "test-queue", 10)

	// Push messages at different priorities
	pushMessageWithPriority(queue, "p9", 9)
	pushMessageWithPriority(queue, "p5", 5)
	pushMessageWithPriority(queue, "p1", 1)

	if queue.Len() != 3 {
		t.Errorf("Expected queue length 3, got %d", queue.Len())
	}

	// Purge
	purged := queue.StreamPurge(func(msg *Message) {})

	if purged != 3 {
		t.Errorf("Expected 3 purged messages, got %d", purged)
	}
	if queue.Len() != 0 {
		t.Errorf("Expected queue length 0 after purge, got %d", queue.Len())
	}
}

func TestNonPriorityQueue_BackwardCompatible(t *testing.T) {
	vh := createTestVHostWithPriority(10)
	queue := createTestQueue(vh, "fifo-queue") // No x-max-priority

	// Should use FIFO channel
	if queue.maxPriority != 0 {
		t.Errorf("Expected maxPriority=0, got %d", queue.maxPriority)
	}
	if queue.priorityMessages != nil {
		t.Error("Expected nil priorityMessages for FIFO queue")
	}

	// Should work as before

	queue.push(Message{Body: []byte("first")})
	queue.push(Message{Body: []byte("second")})
	assertNextMessage(t, queue, "first")
	assertNextMessage(t, queue, "second")
}

func createTestQueue(vh *VHost, s string) *Queue {
	props := &QueueProperties{}
	queue, _ := vh.CreateQueue(s, props, MANAGEMENT_CONNECTION_ID)
	return queue
}

// Helper functions
func createTestVHostWithPriority(maxPri uint8) *VHost {
	options := VHostOptions{
		QueueBufferSize: 1000,
		MaxPriority:     maxPri,
	}
	return NewVhost("test-vhost", options)
}

func createTestQueueWithMaxPriority(vh *VHost, name string, maxPri uint8) *Queue {
	props := &QueueProperties{
		Arguments: map[string]any{
			"x-max-priority": int(maxPri),
		},
	}
	queue, _ := vh.CreateQueue(name, props, MANAGEMENT_CONNECTION_ID)
	return queue
}

func pushMessageWithPriority(queue *Queue, body string, priority uint8) {
	msg := Message{
		ID:   generateID(),
		Body: []byte(body),
		Properties: amqp.BasicProperties{
			Priority: priority,
		},
	}
	queue.Push(msg)
}

func generateID() string {
	return fmt.Sprintf("msg-%d", rand.Int63())
}

func assertNextMessage(t *testing.T, queue *Queue, expectedBody string) {
	msg := queue.Pop()
	if msg == nil {
		t.Fatalf("Expected message '%s', got nil", expectedBody)
	}
	if string(msg.Body) != expectedBody {
		t.Errorf("Expected message body '%s', got '%s'", expectedBody, string(msg.Body))
	}
}
