package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
)

func TestPriorityQueue_DeliveryOrder(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Declare queue with x-max-priority
	queue, err := tc.Ch.QueueDeclare(
		"test-priority-queue",
		false, false, false, false,
		amqp091.Table{
			"x-max-priority": int32(10),
		},
	)
	require.NoError(t, err)

	// Publish messages with different priorities
	publishWithPriority(t, tc.Ch, queue.Name, "low", 1)
	publishWithPriority(t, tc.Ch, queue.Name, "high", 9)
	publishWithPriority(t, tc.Ch, queue.Name, "medium", 5)

	// Consume and verify order
	msgs := consumeMessages(t, tc.Ch, queue.Name, 3)
	require.Equal(t, "high", string(msgs[0].Body))
	require.Equal(t, "medium", string(msgs[1].Body))
	require.Equal(t, "low", string(msgs[2].Body))
}

func TestPriorityQueue_FIFO_WithinPriority(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	queue, err := tc.Ch.QueueDeclare(
		"test-fifo-priority",
		false, false, false, false,
		amqp091.Table{"x-max-priority": int32(10)},
	)
	require.NoError(t, err)

	// Publish multiple messages with same priority
	for i := 1; i <= 5; i++ {
		publishWithPriority(t, tc.Ch, queue.Name, fmt.Sprintf("msg-%d", i), 5)
	}

	// Should maintain FIFO within priority 5
	msgs := consumeMessages(t, tc.Ch, queue.Name, 5)
	for i := 0; i < 5; i++ {
		expected := fmt.Sprintf("msg-%d", i+1)
		require.Equal(t, expected, string(msgs[i].Body))
	}
}

func TestPriorityQueue_MixedPriorities(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	queue, err := tc.Ch.QueueDeclare(
		"test-mixed-priority",
		false, false, false, false,
		amqp091.Table{"x-max-priority": int32(10)},
	)
	require.NoError(t, err)

	// Publish in random order
	publishWithPriority(t, tc.Ch, queue.Name, "p5-first", 5)
	publishWithPriority(t, tc.Ch, queue.Name, "p1", 1)
	publishWithPriority(t, tc.Ch, queue.Name, "p9", 9)
	publishWithPriority(t, tc.Ch, queue.Name, "p5-second", 5)
	publishWithPriority(t, tc.Ch, queue.Name, "p3", 3)

	// Expected order: p9, p5-first, p5-second, p3, p1
	msgs := consumeMessages(t, tc.Ch, queue.Name, 5)
	require.Equal(t, "p9", string(msgs[0].Body))
	require.Equal(t, "p5-first", string(msgs[1].Body))
	require.Equal(t, "p5-second", string(msgs[2].Body))
	require.Equal(t, "p3", string(msgs[3].Body))
	require.Equal(t, "p1", string(msgs[4].Body))
}

func TestPriorityQueue_NoPriorityIsZero(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	queue, err := tc.Ch.QueueDeclare(
		"test-default-priority",
		false, false, false, false,
		amqp091.Table{"x-max-priority": int32(10)},
	)
	require.NoError(t, err)

	// Publish without priority (defaults to 0)
	err = tc.Ch.Publish("", queue.Name, false, false, amqp091.Publishing{
		Body: []byte("no-priority"),
	})
	require.NoError(t, err)

	publishWithPriority(t, tc.Ch, queue.Name, "priority-5", 5)

	// Priority 5 should come first
	msgs := consumeMessages(t, tc.Ch, queue.Name, 2)
	require.Equal(t, "priority-5", string(msgs[0].Body))
	require.Equal(t, "no-priority", string(msgs[1].Body))
}

func TestPriorityQueue_WithDLX(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX
	dlx := "test-dlx-priority"
	err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("test-dlq-priority", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, "dlx-key", dlx, false, nil)
	require.NoError(t, err)

	// Main priority queue with DLX
	queue, err := tc.Ch.QueueDeclare(
		"test-priority-dlx",
		false, false, false, false,
		amqp091.Table{
			"x-max-priority":            int32(10),
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": "dlx-key",
		},
	)
	require.NoError(t, err)

	// Publish with priority
	publishWithPriority(t, tc.Ch, queue.Name, "high-rejected", 9)

	// Consume WITHOUT auto-ack, then reject manually
	msgs, err := tc.Ch.Consume(queue.Name, "", false, false, false, false, nil)
	require.NoError(t, err)

	var msg amqp091.Delivery
	select {
	case msg = <-msgs:
		// Reject without requeue - should go to DLX
		err = msg.Nack(false, false)
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message to consume")
	}

	// Give DLX routing a moment to complete
	time.Sleep(100 * time.Millisecond)

	// Check DLQ received message with priority preserved
	dlqMsgs := consumeMessages(t, tc.Ch, dlq.Name, 1)
	require.Equal(t, "high-rejected", string(dlqMsgs[0].Body))
	require.Equal(t, uint8(9), dlqMsgs[0].Priority)
}

// Helper functions
func publishWithPriority(t *testing.T, ch *amqp091.Channel, queue, body string, priority uint8) {
	err := ch.Publish("", queue, false, false, amqp091.Publishing{
		Body:     []byte(body),
		Priority: priority,
	})
	require.NoError(t, err)
}

func consumeMessages(t *testing.T, ch *amqp091.Channel, queue string, count int) []amqp091.Delivery {
	msgs, err := ch.Consume(queue, "", false, false, false, false, nil)
	require.NoError(t, err)

	result := make([]amqp091.Delivery, 0, count)
	for i := 0; i < count; i++ {
		select {
		case msg := <-msgs:
			result = append(result, msg)
			msg.Ack(false)
		case <-time.After(2 * time.Second):
			t.Fatalf("Timeout waiting for message %d/%d", i+1, count)
		}
	}
	return result
}
