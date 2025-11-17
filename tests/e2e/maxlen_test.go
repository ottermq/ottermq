package e2e

import (
	"fmt"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMaxLen_BasicOverflow verifies basic queue length limit enforcement
func TestMaxLen_BasicOverflow(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create queue with max-length=5
	queue, err := tc.Ch.QueueDeclare(
		"test-maxlen-basic",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-max-length": int32(5),
		},
	)
	require.NoError(t, err)

	// Publish 10 messages
	for i := 0; i < 10; i++ {
		err = tc.Ch.Publish("", queue.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(fmt.Sprintf("message-%d", i)),
		})
		require.NoError(t, err)
	}

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// Queue should have exactly 5 messages (newest ones)
	msgs := tc.StartConsumer(queue.Name, "", false)

	// Should receive exactly 5 messages
	for i := 5; i < 10; i++ {
		msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
		require.True(t, ok, "should receive message %d", i)
		assert.Equal(t, []byte(fmt.Sprintf("message-%d", i)), msg.Body)
		msg.Ack(false)
	}

	// No more messages
	tc.ExpectNoMessage(msgs, 500*time.Millisecond)
}

// TestMaxLen_WithDeadLetter verifies evicted messages are dead-lettered
func TestMaxLen_WithDeadLetter(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX infrastructure
	dlx := "test-dlx-maxlen"
	err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("test-dlq-maxlen", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, "evicted", dlx, false, nil)
	require.NoError(t, err)

	// Create main queue with max-length=3 and DLX
	mainQueue, err := tc.Ch.QueueDeclare(
		"test-queue-maxlen-dlx",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-max-length":              int32(3),
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": "evicted",
		},
	)
	require.NoError(t, err)

	// Publish 6 messages
	for i := 0; i < 6; i++ {
		err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(fmt.Sprintf("message-%d", i)),
		})
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	// Main queue should have 3 newest messages
	mainMsgs := tc.StartConsumer(mainQueue.Name, "", false)
	for i := 3; i < 6; i++ {
		msg, ok := tc.ConsumeWithTimeout(mainMsgs, 2*time.Second)
		require.True(t, ok, "should receive message %d from main queue", i)
		assert.Equal(t, []byte(fmt.Sprintf("message-%d", i)), msg.Body)
		msg.Ack(false)
	}
	tc.ExpectNoMessage(mainMsgs, 500*time.Millisecond)

	// DLQ should have 3 oldest evicted messages
	dlqMsgs := tc.StartConsumer(dlq.Name, "", false)
	for i := 0; i < 3; i++ {
		deadMsg, ok := tc.ConsumeWithTimeout(dlqMsgs, 2*time.Second)
		require.True(t, ok, "should receive evicted message %d from DLQ", i)
		assert.Equal(t, []byte(fmt.Sprintf("message-%d", i)), deadMsg.Body)

		// Verify x-death headers
		xDeath, ok := deadMsg.Headers["x-death"].([]interface{})
		require.True(t, ok, "x-death header should be an array")
		require.Len(t, xDeath, 1, "should have one death entry")

		deathEntry := xDeath[0].(amqp.Table)
		assert.Equal(t, "maxlen", deathEntry["reason"], "death reason should be 'maxlen'")
		assert.Equal(t, mainQueue.Name, deathEntry["queue"])
		assert.Equal(t, int64(1), deathEntry["count"])

		deadMsg.Ack(false)
	}
	tc.ExpectNoMessage(dlqMsgs, 500*time.Millisecond)
}

// TestMaxLen_TransactionCommit verifies max-length enforcement during transaction commits
func TestMaxLen_TransactionCommit(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Enable transaction mode
	err := tc.Ch.Tx()
	require.NoError(t, err)

	// Create queue with max-length=5
	queue, err := tc.Ch.QueueDeclare(
		"test-maxlen-tx",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-max-length": int32(5),
		},
	)
	require.NoError(t, err)

	// Publish 20 messages in transaction
	for i := 0; i < 20; i++ {
		err = tc.Ch.Publish("", queue.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(fmt.Sprintf("tx-message-%d", i)),
		})
		require.NoError(t, err)
	}

	// Commit transaction (all 20 messages committed at once)
	err = tc.Ch.TxCommit()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Queue should have exactly 5 newest messages
	msgs := tc.StartConsumer(queue.Name, "", false)
	for i := 15; i < 20; i++ {
		msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
		require.True(t, ok, "should receive message %d", i)
		assert.Equal(t, []byte(fmt.Sprintf("tx-message-%d", i)), msg.Body)
		msg.Ack(false)
	}
	tc.ExpectNoMessage(msgs, 500*time.Millisecond)
}

// TestMaxLen_RequeueRespected verifies requeued messages respect max-length
func TestMaxLen_RequeueRespected(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX for overflow detection
	dlx := "test-dlx-requeue"
	err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("test-dlq-requeue", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, "evicted", dlx, false, nil)
	require.NoError(t, err)

	// Create queue with max-length=3
	queue, err := tc.Ch.QueueDeclare(
		"test-maxlen-requeue",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-max-length":              int32(3),
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": "evicted",
		},
	)
	require.NoError(t, err)

	// Publish 3 messages (fills queue)
	for i := 0; i < 3; i++ {
		err = tc.Ch.Publish("", queue.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(fmt.Sprintf("initial-%d", i)),
		})
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	// Consume and nack with requeue=true
	msgs := tc.StartConsumer(queue.Name, "", false)
	msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	require.True(t, ok, "should receive first message")
	assert.Equal(t, []byte("initial-0"), msg.Body)

	// Nack with requeue while queue is at limit
	err = msg.Nack(false, true)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Try to publish one more message - should trigger eviction
	err = tc.Ch.Publish("", queue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("new-message"),
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Should have 3 messages in queue (requeued one was evicted or new one replaced oldest)
	mainMsgs := tc.StartConsumer(queue.Name, "", true)
	receivedCount := 0
	for i := 0; i < 4; i++ { // Try to get up to 4
		msg, ok := tc.ConsumeWithTimeout(mainMsgs, 500*time.Millisecond)
		if !ok {
			break
		}
		receivedCount++
		t.Logf("Received: %s", msg.Body)
	}
	assert.Equal(t, 3, receivedCount, "should have exactly 3 messages in queue")
}

// TestMaxLen_FanoutIndependence verifies each queue independently enforces its max-length
func TestMaxLen_FanoutIndependence(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create fanout exchange
	exchange := "test-fanout-maxlen"
	err := tc.Ch.ExchangeDeclare(exchange, "fanout", false, true, false, false, nil)
	require.NoError(t, err)

	// Create three queues with different max-lengths
	queue1, err := tc.Ch.QueueDeclare(
		"test-q1-max2",
		false,
		true,
		false,
		false,
		amqp.Table{"x-max-length": int32(2)},
	)
	require.NoError(t, err)

	queue2, err := tc.Ch.QueueDeclare(
		"test-q2-max5",
		false,
		true,
		false,
		false,
		amqp.Table{"x-max-length": int32(5)},
	)
	require.NoError(t, err)

	queue3, err := tc.Ch.QueueDeclare(
		"test-q3-unlimited",
		false,
		true,
		false,
		false,
		nil, // No max-length
	)
	require.NoError(t, err)

	// Bind all queues to fanout exchange
	err = tc.Ch.QueueBind(queue1.Name, "", exchange, false, nil)
	require.NoError(t, err)
	err = tc.Ch.QueueBind(queue2.Name, "", exchange, false, nil)
	require.NoError(t, err)
	err = tc.Ch.QueueBind(queue3.Name, "", exchange, false, nil)
	require.NoError(t, err)

	// Publish 10 messages to fanout exchange
	for i := 0; i < 10; i++ {
		err = tc.Ch.Publish(exchange, "", false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(fmt.Sprintf("fanout-%d", i)),
		})
		require.NoError(t, err)
	}

	time.Sleep(200 * time.Millisecond)

	// Queue1 should have 2 messages
	msgs1 := tc.StartConsumer(queue1.Name, "", true)
	count1 := 0
	for i := 0; i < 3; i++ {
		if _, ok := tc.ConsumeWithTimeout(msgs1, 500*time.Millisecond); ok {
			count1++
		}
	}
	assert.Equal(t, 2, count1, "queue1 should have 2 messages")

	// Queue2 should have 5 messages
	msgs2 := tc.StartConsumer(queue2.Name, "", true)
	count2 := 0
	for i := 0; i < 6; i++ {
		if _, ok := tc.ConsumeWithTimeout(msgs2, 500*time.Millisecond); ok {
			count2++
		}
	}
	assert.Equal(t, 5, count2, "queue2 should have 5 messages")

	// Queue3 should have all 10 messages
	msgs3 := tc.StartConsumer(queue3.Name, "", true)
	count3 := 0
	for i := 0; i < 11; i++ {
		if _, ok := tc.ConsumeWithTimeout(msgs3, 500*time.Millisecond); ok {
			count3++
		}
	}
	assert.Equal(t, 10, count3, "queue3 should have all 10 messages")
}

// TestMaxLen_NoLimitQueue verifies queues without x-max-length are unaffected
func TestMaxLen_NoLimitQueue(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create queue without max-length
	queue, err := tc.Ch.QueueDeclare(
		"test-no-maxlen",
		false,
		true,
		false,
		false,
		nil, // No arguments
	)
	require.NoError(t, err)

	// Publish many messages
	for i := 0; i < 100; i++ {
		err = tc.Ch.Publish("", queue.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(fmt.Sprintf("message-%d", i)),
		})
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	// Should have all 100 messages
	msgs := tc.StartConsumer(queue.Name, "", true)
	count := 0
	for i := 0; i < 101; i++ {
		if _, ok := tc.ConsumeWithTimeout(msgs, 100*time.Millisecond); ok {
			count++
		} else {
			break
		}
	}
	assert.Equal(t, 100, count, "should have all 100 messages")
}

// TestMaxLen_PropertyPreservation verifies message properties are preserved for kept messages
func TestMaxLen_PropertyPreservation(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create queue with max-length=2
	queue, err := tc.Ch.QueueDeclare(
		"test-maxlen-props",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-max-length": int32(2),
		},
	)
	require.NoError(t, err)

	// Publish 5 messages with different properties
	for i := 0; i < 5; i++ {
		err = tc.Ch.Publish("", queue.Name, false, false, amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Priority:     uint8(i),
			Headers: amqp.Table{
				"index": int32(i),
			},
			Body: []byte(fmt.Sprintf(`{"id":%d}`, i)),
		})
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	// Should have last 2 messages with properties intact
	msgs := tc.StartConsumer(queue.Name, "", false)
	for i := 3; i < 5; i++ {
		msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
		require.True(t, ok, "should receive message %d", i)
		assert.Equal(t, []byte(fmt.Sprintf(`{"id":%d}`, i)), msg.Body)
		assert.Equal(t, "application/json", msg.ContentType)
		assert.Equal(t, uint8(2), msg.DeliveryMode) // Persistent
		assert.Equal(t, uint8(i), msg.Priority)
		assert.Equal(t, int32(i), msg.Headers["index"])
		msg.Ack(false)
	}
	tc.ExpectNoMessage(msgs, 500*time.Millisecond)
}
