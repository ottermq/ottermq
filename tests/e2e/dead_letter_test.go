package e2e

import (
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDLX_BasicRejection verifies that a rejected message is routed to the configured dead letter exchange
func TestDLX_BasicRejection(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()
	dlx := "test-dlx-basic"
	dlk := "dead"
	// Create DLX and dead letter queue
	err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("test-dlq-basic", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, dlk, dlx, false, nil)
	require.NoError(t, err)

	// Create main queue with DLX configuration
	mainQueue, err := tc.Ch.QueueDeclare(
		"test-main-queue-basic",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": dlk,
		},
	)
	require.NoError(t, err)

	// Consume and reject the message
	msgs := tc.StartConsumer(mainQueue.Name, "", false)

	// Publish a message to main queue
	err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("test message for rejection"),
	})
	require.NoError(t, err)

	msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for message")
	}

	dlqMsgs := tc.StartConsumer(dlq.Name, "", true)

	// Reject without requeue
	err = msg.Reject(false)
	require.NoError(t, err)

	// Verify message was routed to DLQ
	time.Sleep(100 * time.Millisecond) // Give time for dead-lettering

	deadMsg, ok := tc.ConsumeWithTimeout(dlqMsgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for dead-lettered message")
	}

	assert.Equal(t, []byte("test message for rejection"), deadMsg.Body)

	// Verify x-death headers
	headers := deadMsg.Headers
	xDeath, ok := headers["x-death"].([]interface{})
	require.True(t, ok, "x-death header should be an array")
	require.Len(t, xDeath, 1, "should have one death entry")

	deathEntry := xDeath[0].(amqp.Table)
	assert.Equal(t, "rejected", deathEntry["reason"])
	assert.Equal(t, mainQueue.Name, deathEntry["queue"])
	assert.Equal(t, int64(1), deathEntry["count"]) // Count is int64 in amqp.Table

	// Verify x-first-death headers
	assert.Equal(t, mainQueue.Name, deadMsg.Headers["x-first-death-queue"])
	assert.Equal(t, "rejected", deadMsg.Headers["x-first-death-reason"])

}

// TestDLX_Nack verifies that a nacked message (requeue=false) is routed to DLX
func TestDLX_Nack(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX infrastructure
	err := tc.Ch.ExchangeDeclare("dlx-nack", "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("dlq-nack", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, "error", "dlx-nack", false, nil)
	require.NoError(t, err)

	// Main queue with DLX
	mainQueue, err := tc.Ch.QueueDeclare(
		"main-nack",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    "dlx-nack",
			"x-dead-letter-routing-key": "error",
		},
	)
	require.NoError(t, err)

	// Publish message
	err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
		Body: []byte("nack test message"),
	})
	require.NoError(t, err)

	// Consume and nack
	msgs := tc.StartConsumer(mainQueue.Name, "", false)

	msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for message")
	}

	err = msg.Nack(false, false) // multiple=false, requeue=false
	require.NoError(t, err)

	// Verify in DLQ
	time.Sleep(100 * time.Millisecond)

	dlqMsgs := tc.StartConsumer(dlq.Name, "", true)

	deadMsg, ok := tc.ConsumeWithTimeout(dlqMsgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for dead-lettered message")
	}

	assert.Equal(t, []byte("nack test message"), deadMsg.Body)
	assert.Equal(t, "rejected", deadMsg.Headers["x-first-death-reason"])
}

// TestDLX_MultipleDeaths verifies that messages can be dead-lettered multiple times
func TestDLX_MultipleDeaths(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create DLX chain: queue1 -> dlx1 -> queue2 -> dlx2 -> queue3
	err := tc.Ch.ExchangeDeclare("dlx1", "direct", false, true, false, false, nil)
	require.NoError(t, err)
	err = tc.Ch.ExchangeDeclare("dlx2", "direct", false, true, false, false, nil)
	require.NoError(t, err)

	// Queue 1 with DLX -> dlx1
	q1, err := tc.Ch.QueueDeclare("dlx-q1", false, true, false, false, amqp.Table{
		"x-dead-letter-exchange":    "dlx1",
		"x-dead-letter-routing-key": "to-q2",
	})
	require.NoError(t, err)

	// Queue 2 with DLX -> dlx2
	q2, err := tc.Ch.QueueDeclare("dlx-q2", false, true, false, false, amqp.Table{
		"x-dead-letter-exchange":    "dlx2",
		"x-dead-letter-routing-key": "to-q3",
	})
	require.NoError(t, err)

	// Queue 3 (no DLX)
	q3, err := tc.Ch.QueueDeclare("dlx-q3", false, true, false, false, nil)
	require.NoError(t, err)

	// Bind queues
	err = tc.Ch.QueueBind(q2.Name, "to-q2", "dlx1", false, nil)
	require.NoError(t, err)
	err = tc.Ch.QueueBind(q3.Name, "to-q3", "dlx2", false, nil)
	require.NoError(t, err)

	// Publish message to q1
	err = tc.Ch.Publish("", q1.Name, false, false, amqp.Publishing{
		Body: []byte("multi-death message"),
	})
	require.NoError(t, err)

	// First death: reject from q1
	msgs1 := tc.StartConsumer(q1.Name, "", false)
	msg1, ok := tc.ConsumeWithTimeout(msgs1, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for message in q1")
	}
	err = msg1.Reject(false)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Second death: reject from q2
	msgs2 := tc.StartConsumer(q2.Name, "", false)
	msg2, ok := tc.ConsumeWithTimeout(msgs2, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for message in q2")
	}

	// Verify x-death has 1 entry
	xDeath, _ := msg2.Headers["x-death"].([]interface{})
	require.Len(t, xDeath, 1, "should have one death entry after first rejection")

	err = msg2.Reject(false)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Verify final message in q3
	msgs3 := tc.StartConsumer(q3.Name, "", true)
	msg3, ok := tc.ConsumeWithTimeout(msgs3, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for message in q3")
	}

	assert.Equal(t, []byte("multi-death message"), msg3.Body)

	// Verify x-death has 2 entries
	xDeath2, ok := msg3.Headers["x-death"].([]interface{})
	require.True(t, ok)
	require.Len(t, xDeath2, 2, "should have two death entries")

	// Verify newest death is first (q2)
	death1 := xDeath2[0].(amqp.Table)
	assert.Equal(t, "dlx-q2", death1["queue"])
	assert.Equal(t, int64(2), death1["count"])

	// Verify oldest death is last (q1)
	death2 := xDeath2[1].(amqp.Table)
	assert.Equal(t, "dlx-q1", death2["queue"])
	assert.Equal(t, int64(1), death2["count"])

	// Verify x-first-death points to q1 (original)
	assert.Equal(t, "dlx-q1", msg3.Headers["x-first-death-queue"])

	// Verify x-last-death points to q2 (previous)
	assert.Equal(t, "dlx-q2", msg3.Headers["x-last-death-queue"])
}

// TestDLX_RoutingKeyOverride verifies x-dead-letter-routing-key overrides original routing key
func TestDLX_RoutingKeyOverride(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create DLX with topic exchange to test routing
	err := tc.Ch.ExchangeDeclare("dlx-topic", "topic", false, true, false, false, nil)
	require.NoError(t, err)

	// Create two DLQs with different bindings
	dlqErrors, err := tc.Ch.QueueDeclare("dlq-errors", false, true, false, false, nil)
	require.NoError(t, err)
	dlqAll, err := tc.Ch.QueueDeclare("dlq-all", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlqErrors.Name, "error.#", "dlx-topic", false, nil)
	require.NoError(t, err)
	err = tc.Ch.QueueBind(dlqAll.Name, "#", "dlx-topic", false, nil)
	require.NoError(t, err)

	// Main queue with routing key override
	mainQueue, err := tc.Ch.QueueDeclare(
		"main-override",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    "dlx-topic",
			"x-dead-letter-routing-key": "error.critical.timeout",
		},
	)
	require.NoError(t, err)

	// Publish with original routing key
	err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
		Body: []byte("override test"),
	})
	require.NoError(t, err)

	// Reject message
	msgs := tc.StartConsumer(mainQueue.Name, "", false)
	msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for message")
	}
	err = msg.Reject(false)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Both DLQs should receive the message (topic routing)
	errMsgs := tc.StartConsumer(dlqErrors.Name, "", true)
	allMsgs := tc.StartConsumer(dlqAll.Name, "", true)

	errMsg, ok := tc.ConsumeWithTimeout(errMsgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for message in dlq-errors")
	}
	allMsg, ok := tc.ConsumeWithTimeout(allMsgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for message in dlq-all")
	}

	assert.Equal(t, []byte("override test"), errMsg.Body)
	assert.Equal(t, []byte("override test"), allMsg.Body)
}

// TestDLX_WithoutConfiguration verifies messages are discarded when DLX is not configured
func TestDLX_WithoutConfiguration(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create queue WITHOUT DLX configuration (use unique name)
	// NOTE: Using auto-delete=false so queue persists for inspection
	queueName := tc.UniqueQueueName("no-dlx-queue")
	q, err := tc.Ch.QueueDeclare(queueName, false, false, false, false, nil)
	require.NoError(t, err)

	// Ensure cleanup at end
	defer func() {
		_, _ = tc.Ch.QueueDelete(q.Name, false, false, false)
	}()

	// Publish message
	err = tc.Ch.Publish("", q.Name, false, false, amqp.Publishing{
		Body: []byte("will be discarded"),
	})
	require.NoError(t, err)

	// Consume and reject
	consumerTag := tc.UniqueConsumerTag("consumer")
	msgs := tc.StartConsumer(q.Name, consumerTag, false)
	msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for message")
	}
	err = msg.Reject(false)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Queue should be empty (message discarded)
	inspected, err := tc.Ch.QueueInspect(q.Name)
	require.NoError(t, err)
	assert.Equal(t, 0, inspected.Messages, "queue should be empty after rejection without DLX")
}

// TestDLX_MultipleNack verifies that multiple messages can be nacked at once
func TestDLX_MultipleNack(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX
	err := tc.Ch.ExchangeDeclare("dlx-multi", "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("dlq-multi", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, "dead", "dlx-multi", false, nil)
	require.NoError(t, err)

	mainQueue, err := tc.Ch.QueueDeclare(
		"main-multi",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    "dlx-multi",
			"x-dead-letter-routing-key": "dead",
		},
	)
	require.NoError(t, err)

	// Publish 3 messages
	for i := 1; i <= 3; i++ {
		err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
			Body: []byte("message " + string(rune('0'+i))),
		})
		require.NoError(t, err)
	}

	// Consume messages without ack
	msgs := tc.StartConsumer(mainQueue.Name, "", false)

	msg1, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for msg1")
	}
	_ = msg1

	msg2, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for msg2")
	}

	msg3, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for msg3")
	}

	// Nack multiple (up to msg2)
	err = msg2.Nack(true, false) // multiple=true, requeue=false
	require.NoError(t, err)

	// msg1 and msg2 should be in DLQ, msg3 is still unacked (delivered but not ready)
	time.Sleep(100 * time.Millisecond)

	// Check DLQ has 2 messages
	dlqInspected, err := tc.Ch.QueueInspect(dlq.Name)
	require.NoError(t, err)
	assert.Equal(t, 2, dlqInspected.Messages, "two messages should be in DLQ")

	// Now ack msg3 to clear it
	err = msg3.Ack(false)
	require.NoError(t, err)

	// After acking msg3, the main queue should be completely empty
	time.Sleep(50 * time.Millisecond)
	inspected, err := tc.Ch.QueueInspect(mainQueue.Name)
	require.NoError(t, err)
	assert.Equal(t, 0, inspected.Messages, "main queue should be empty after all messages processed")
}

// TestDLX_PersistentMessage verifies that persistent messages retain their delivery mode
func TestDLX_PersistentMessage(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX
	err := tc.Ch.ExchangeDeclare("dlx-persist", "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("dlq-persist", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, "dead", "dlx-persist", false, nil)
	require.NoError(t, err)

	mainQueue, err := tc.Ch.QueueDeclare(
		"main-persist",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    "dlx-persist",
			"x-dead-letter-routing-key": "dead",
		},
	)
	require.NoError(t, err)

	// Publish persistent message
	err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Body:         []byte("persistent message"),
	})
	require.NoError(t, err)

	// Reject
	msgs := tc.StartConsumer(mainQueue.Name, "", false)
	msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for message")
	}
	err = msg.Reject(false)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Verify in DLQ with persistent delivery mode
	dlqMsgs := tc.StartConsumer(dlq.Name, "", true)
	deadMsg, ok := tc.ConsumeWithTimeout(dlqMsgs, 2*time.Second)
	if !ok {
		t.Fatal("timeout waiting for dead-lettered message")
	}

	assert.Equal(t, []byte("persistent message"), deadMsg.Body)
	assert.Equal(t, uint8(amqp.Persistent), deadMsg.DeliveryMode, "message should remain persistent")
}
