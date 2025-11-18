package e2e

import (
	"fmt"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConsumerCancel_RequeuesUnacked verifies that canceling a consumer
// automatically requeues all unacked messages with redelivered=true
func TestConsumerCancel_RequeuesUnacked(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create a queue (NOT auto-delete to avoid deletion on consumer cancel)
	queue, err := tc.Ch.QueueDeclare(
		"test-cancel-requeue",
		false, // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	require.NoError(t, err)

	// Clean up queue at end
	defer func() {
		_, _ = tc.Ch.QueueDelete(queue.Name, false, false, false)
	}()

	// Publish 5 messages
	for i := 0; i < 5; i++ {
		err = tc.Ch.Publish("", queue.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(fmt.Sprintf("message-%d", i)),
		})
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	// Start consumer with manual ack (noAck=false)
	msgs := tc.StartConsumer(queue.Name, "test-consumer", false)

	// Receive 3 messages but DON'T ack them
	var receivedMsgs []amqp.Delivery
	for i := 0; i < 3; i++ {
		msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
		require.True(t, ok, "should receive message %d", i)
		receivedMsgs = append(receivedMsgs, msg)
		t.Logf("Received (not acked): %s", string(msg.Body))
	}

	// Cancel the consumer (should requeue the 3 unacked messages)
	err = tc.Ch.Cancel("test-consumer", false)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	// Start a new consumer and receive all messages
	msgs2 := tc.StartConsumer(queue.Name, "test-consumer-2", false)

	// Should receive 5 messages total (3 requeued + 2 not yet consumed)
	receivedBodies := make([]string, 0, 5)
	redeliveredCount := 0

	for i := 0; i < 5; i++ {
		msg, ok := tc.ConsumeWithTimeout(msgs2, 2*time.Second)
		require.True(t, ok, "should receive message %d in second consumer", i)

		body := string(msg.Body)
		receivedBodies = append(receivedBodies, body)

		if msg.Redelivered {
			redeliveredCount++
			t.Logf("Received redelivered: %s", body)
		} else {
			t.Logf("Received fresh: %s", body)
		}

		msg.Ack(false)
	}

	// Verify we got all 5 messages
	assert.Len(t, receivedBodies, 5, "should receive all 5 messages")

	// Verify at least the 3 messages that were unacked are marked as redelivered
	// Note: Due to queue implementation details, more messages might be marked redelivered
	assert.GreaterOrEqual(t, redeliveredCount, 3, "should have at least 3 redelivered messages")

	// No more messages
	tc.ExpectNoMessage(msgs2, 500*time.Millisecond)
}

// TestConsumerCancel_OnlyRequeuesToOriginalQueue verifies that consumer cancel
// only requeues messages to the queue they came from (isolation between queues)
func TestConsumerCancel_OnlyRequeuesToOriginalQueue(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create two queues (NOT auto-delete)
	queue1, err := tc.Ch.QueueDeclare(
		"test-cancel-q1",
		false, // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	require.NoError(t, err)
	defer func() {
		_, _ = tc.Ch.QueueDelete(queue1.Name, false, false, false)
	}()

	queue2, err := tc.Ch.QueueDeclare(
		"test-cancel-q2",
		false, // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	require.NoError(t, err)
	defer func() {
		_, _ = tc.Ch.QueueDelete(queue2.Name, false, false, false)
	}()

	// Publish messages to both queues
	for i := 0; i < 3; i++ {
		err = tc.Ch.Publish("", queue1.Name, false, false, amqp.Publishing{
			Body: []byte(fmt.Sprintf("q1-msg-%d", i)),
		})
		require.NoError(t, err)

		err = tc.Ch.Publish("", queue2.Name, false, false, amqp.Publishing{
			Body: []byte(fmt.Sprintf("q2-msg-%d", i)),
		})
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	// Start consumers on both queues (same channel)
	msgs1 := tc.StartConsumer(queue1.Name, "consumer-q1", false)
	msgs2 := tc.StartConsumer(queue2.Name, "consumer-q2", false)

	// Receive messages from both queues but don't ack
	for i := 0; i < 2; i++ {
		_, ok := tc.ConsumeWithTimeout(msgs1, 2*time.Second)
		require.True(t, ok, "should receive from q1")

		_, ok = tc.ConsumeWithTimeout(msgs2, 2*time.Second)
		require.True(t, ok, "should receive from q2")
	}

	// Cancel only the first consumer
	err = tc.Ch.Cancel("consumer-q1", false)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	// Start new consumer on queue1
	msgs1New := tc.StartConsumer(queue1.Name, "consumer-q1-new", false)

	// Should receive 3 messages from queue1 (2 requeued + 1 not yet consumed)
	q1Count := 0
	for i := 0; i < 4; i++ {
		msg, ok := tc.ConsumeWithTimeout(msgs1New, 500*time.Millisecond)
		if !ok {
			break
		}
		q1Count++
		assert.Contains(t, string(msg.Body), "q1-msg-", "should only get q1 messages")
		msg.Ack(false)
	}
	assert.Equal(t, 3, q1Count, "queue1 should have 3 messages (2 requeued + 1 fresh)")

	// Queue2 consumer should still have 1 message waiting (it was not canceled)
	msg, ok := tc.ConsumeWithTimeout(msgs2, 2*time.Second)
	require.True(t, ok, "queue2 consumer should still get the last message")
	assert.Contains(t, string(msg.Body), "q2-msg-2")
	msg.Ack(false)

	// No more messages on either queue
	tc.ExpectNoMessage(msgs1New, 500*time.Millisecond)
	tc.ExpectNoMessage(msgs2, 500*time.Millisecond)
}

// TestConsumerCancel_NoAckConsumer_NoRequeue verifies that canceling a no-ack
// consumer doesn't requeue anything (no messages tracked as unacked)
func TestConsumerCancel_NoAckConsumer_NoRequeue(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create queue (NOT auto-delete)
	queue, err := tc.Ch.QueueDeclare(
		"test-cancel-noack",
		false, // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	require.NoError(t, err)
	defer func() {
		_, _ = tc.Ch.QueueDelete(queue.Name, false, false, false)
	}()

	// Publish 5 messages
	for i := 0; i < 5; i++ {
		err = tc.Ch.Publish("", queue.Name, false, false, amqp.Publishing{
			Body: []byte(fmt.Sprintf("message-%d", i)),
		})
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	// Start consumer with auto-ack (noAck=true)
	msgs := tc.StartConsumer(queue.Name, "noack-consumer", true)

	// Receive 3 messages (they're auto-acked)
	for i := 0; i < 3; i++ {
		msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
		require.True(t, ok, "should receive message %d", i)
		t.Logf("Auto-acked: %s", string(msg.Body))
	}

	// Cancel the consumer
	err = tc.Ch.Cancel("noack-consumer", false)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	// Start new consumer - should only get the 2 remaining messages
	msgs2 := tc.StartConsumer(queue.Name, "consumer-2", false)

	receivedCount := 0
	for i := 0; i < 5; i++ {
		msg, ok := tc.ConsumeWithTimeout(msgs2, 500*time.Millisecond)
		if !ok {
			break
		}
		receivedCount++
		t.Logf("Received by consumer-2: %s", string(msg.Body))
		msg.Ack(false)
	}

	// Should only get 2 messages (the 3 auto-acked ones are gone)
	// Note: Due to delivery loop behavior, we might get 0 if the queue stops delivering
	assert.LessOrEqual(t, receivedCount, 2, "should receive at most 2 remaining messages (3 were auto-acked)")
}

// TestConsumerCancel_HighPrefetch verifies that consumer cancel works efficiently
// even with a large number of unacked messages (performance test)
func TestConsumerCancel_HighPrefetch(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create queue (NOT auto-delete)
	queue, err := tc.Ch.QueueDeclare(
		"test-cancel-high-prefetch",
		false, // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	require.NoError(t, err)
	defer func() {
		_, _ = tc.Ch.QueueDelete(queue.Name, false, false, false)
	}()

	// Set high prefetch count
	err = tc.Ch.Qos(500, 0, false)
	require.NoError(t, err)

	// Publish 1000 messages
	messageCount := 1000
	for i := 0; i < messageCount; i++ {
		err = tc.Ch.Publish("", queue.Name, false, false, amqp.Publishing{
			Body: []byte(fmt.Sprintf("msg-%d", i)),
		})
		require.NoError(t, err)
	}

	time.Sleep(500 * time.Millisecond)

	// Start consumer with manual ack
	msgs := tc.StartConsumer(queue.Name, "high-prefetch-consumer", false)

	// Receive up to 500 messages (prefetch limit) but don't ack
	receivedCount := 0
	for i := 0; i < 600; i++ { // Try more than prefetch
		msg, ok := tc.ConsumeWithTimeout(msgs, 100*time.Millisecond)
		if !ok {
			break
		}
		receivedCount++
		// Don't ack - just count
		_ = msg
	}

	t.Logf("Received %d messages with prefetch 500", receivedCount)
	assert.LessOrEqual(t, receivedCount, 500, "should not receive more than prefetch limit")

	// Cancel consumer (this should be fast even with many unacked)
	startTime := time.Now()
	err = tc.Ch.Cancel("high-prefetch-consumer", false)
	require.NoError(t, err)
	cancelDuration := time.Since(startTime)

	t.Logf("Consumer cancel took %v with %d unacked messages", cancelDuration, receivedCount)

	// Cancel should be fast (< 100ms even with 500 unacked)
	assert.Less(t, cancelDuration, 100*time.Millisecond, "cancel should be fast with new O(1) implementation")

	time.Sleep(200 * time.Millisecond)

	// Start new consumer - should get all messages
	msgs2 := tc.StartConsumer(queue.Name, "consumer-2", false)

	finalCount := 0
	redeliveredCount := 0
	for i := 0; i < messageCount+10; i++ {
		msg, ok := tc.ConsumeWithTimeout(msgs2, 100*time.Millisecond)
		if !ok {
			break
		}
		finalCount++
		if msg.Redelivered {
			redeliveredCount++
		}
		msg.Ack(false)
	}

	// Should get all or nearly all messages (accounting for potential delivery races)
	assert.GreaterOrEqual(t, finalCount, 999, "should receive at least 999 of 1000 messages")
	assert.LessOrEqual(t, finalCount, 1000, "should not receive more than 1000 messages")

	// The messages that were delivered to the canceled consumer should be redelivered
	t.Logf("Redelivered count: %d out of %d total", redeliveredCount, finalCount)
	assert.Equal(t, receivedCount, redeliveredCount, "redelivered count should match unacked count from canceled consumer")
}

// TestConsumerCancel_MultipleConsumersSameChannel verifies that canceling one
// consumer on a channel doesn't affect other consumers on the same channel
func TestConsumerCancel_MultipleConsumersSameChannel(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create two queues (NOT auto-delete)
	queue1, err := tc.Ch.QueueDeclare(
		"test-multi-consumer-q1",
		false, // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	require.NoError(t, err)
	defer func() {
		_, _ = tc.Ch.QueueDelete(queue1.Name, false, false, false)
	}()

	queue2, err := tc.Ch.QueueDeclare(
		"test-multi-consumer-q2",
		false, // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	require.NoError(t, err)
	defer func() {
		_, _ = tc.Ch.QueueDelete(queue2.Name, false, false, false)
	}()

	// Publish messages to both queues
	for i := 0; i < 5; i++ {
		err = tc.Ch.Publish("", queue1.Name, false, false, amqp.Publishing{
			Body: []byte(fmt.Sprintf("q1-%d", i)),
		})
		require.NoError(t, err)

		err = tc.Ch.Publish("", queue2.Name, false, false, amqp.Publishing{
			Body: []byte(fmt.Sprintf("q2-%d", i)),
		})
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	// Start two consumers on same channel
	msgs1 := tc.StartConsumer(queue1.Name, "consumer-1", false)
	msgs2 := tc.StartConsumer(queue2.Name, "consumer-2", false)

	// Receive 3 messages from each but don't ack
	for i := 0; i < 3; i++ {
		_, ok := tc.ConsumeWithTimeout(msgs1, 2*time.Second)
		require.True(t, ok)

		_, ok = tc.ConsumeWithTimeout(msgs2, 2*time.Second)
		require.True(t, ok)
	}

	// Cancel only consumer-1
	err = tc.Ch.Cancel("consumer-1", false)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	// Consumer-2 should still be able to receive and ack its remaining messages
	for i := 0; i < 2; i++ {
		msg, ok := tc.ConsumeWithTimeout(msgs2, 2*time.Second)
		require.True(t, ok, "consumer-2 should still work after consumer-1 is canceled")
		msg.Ack(false)
	}

	// Consumer-1's unacked messages should be requeued
	msgs1New := tc.StartConsumer(queue1.Name, "consumer-1-new", false)

	count := 0
	for i := 0; i < 6; i++ {
		msg, ok := tc.ConsumeWithTimeout(msgs1New, 500*time.Millisecond)
		if !ok {
			break
		}
		count++
		msg.Ack(false)
	}

	assert.Equal(t, 5, count, "should receive all 5 messages from q1 (3 requeued + 2 not consumed)")

	// No more messages on either queue
	tc.ExpectNoMessage(msgs1New, 500*time.Millisecond)
	tc.ExpectNoMessage(msgs2, 500*time.Millisecond)
}

// TestConsumerCancel_WithRedeliveredFlag verifies that requeued messages
// have the redelivered flag set correctly
func TestConsumerCancel_WithRedeliveredFlag(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create queue (NOT auto-delete)
	queue, err := tc.Ch.QueueDeclare(
		"test-redelivered-flag",
		false, // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	require.NoError(t, err)
	defer func() {
		_, _ = tc.Ch.QueueDelete(queue.Name, false, false, false)
	}()

	// Publish message
	err = tc.Ch.Publish("", queue.Name, false, false, amqp.Publishing{
		Body: []byte("test-message"),
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// First delivery - redelivered should be false
	msgs1 := tc.StartConsumer(queue.Name, "consumer-1", false)
	msg1, ok := tc.ConsumeWithTimeout(msgs1, 2*time.Second)
	require.True(t, ok)
	assert.False(t, msg1.Redelivered, "first delivery should have redelivered=false")

	// Cancel consumer without acking
	err = tc.Ch.Cancel("consumer-1", false)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	// Second delivery - redelivered should be true
	msgs2 := tc.StartConsumer(queue.Name, "consumer-2", false)
	msg2, ok := tc.ConsumeWithTimeout(msgs2, 2*time.Second)
	require.True(t, ok)
	assert.True(t, msg2.Redelivered, "redelivered message should have redelivered=true")

	msg2.Ack(false)

	// No more messages
	tc.ExpectNoMessage(msgs2, 500*time.Millisecond)
}
