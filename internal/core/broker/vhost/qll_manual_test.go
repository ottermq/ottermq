package vhost

import (
	"fmt"
	"testing"
)

// Run with: go test -tags=manual -v -run TestManualQueueLengthLimit
func TestManualQueueLengthLimit(t *testing.T) {
	// Create a vhost with mock persistence
	vh := &VHost{
		Name:               "test",
		Queues:             make(map[string]*Queue),
		queueBufferSize:    100,
		QueueLengthLimiter: &DefaultQueueLengthLimiter{},
	}
	vh.QueueLengthLimiter.(*DefaultQueueLengthLimiter).vh = vh

	// Create a queue with max-length=5
	queueProps := &QueueProperties{}
	queueProps.Arguments = map[string]interface{}{
		"x-max-length": int32(5),
	}

	queue := NewQueue("test-queue", 100, vh)
	queue.Props = queueProps
	setQueueMaxLength(queueProps, queue, "test-queue")

	fmt.Printf("Queue created: %s, maxLength=%d, hasVH=%v\n", queue.Name, queue.maxLength, queue.vh != nil)

	// Publish 10 messages
	for i := range 10 {
		msg := Message{
			ID:   fmt.Sprintf("msg-%d", i),
			Body: []byte(fmt.Sprintf("message-%d", i)),
		}
		fmt.Printf("Before push %d: count=%d\n", i, queue.count)
		queue.Push(msg)
		fmt.Printf("After push %d: count=%d\n", i, queue.count)
	}

	// Check how many messages remain
	finalCount := queue.count
	fmt.Printf("\nFinal count: %d (expected: 5)\n", finalCount)

	// Pop and print all messages
	fmt.Println("\nMessages in queue:")
	for {
		msg := queue.Pop()
		if msg == nil {
			break
		}
		fmt.Printf("  %s\n", string(msg.Body))
	}

	if finalCount != 5 {
		t.Fatalf("Expected 5 messages, got %d", finalCount)
	}
}
