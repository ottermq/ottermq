package e2e

import (
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTTL_PerMessageExpiration_Discard verifies that expired messages are discarded when no DLX configured
func TestTTL_PerMessageExpiration_Discard(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create queue without DLX
	queue, err := tc.Ch.QueueDeclare("test-ttl-discard", false, true, false, false, nil)
	require.NoError(t, err)

	// Publish message with 100ms TTL
	err = tc.Ch.Publish("", queue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("will expire soon"),
		Expiration:  "100", // TTL in milliseconds (relative)
	})
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Try to consume - should get nothing
	msgs := tc.StartConsumer(queue.Name, "", true)
	tc.ExpectNoMessage(msgs, 1*time.Second)
}

// TestTTL_PerMessageExpiration_DeadLetter verifies that expired messages are dead-lettered when DLX configured
func TestTTL_PerMessageExpiration_DeadLetter(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX infrastructure
	dlx := "test-dlx-ttl"
	dlk := "expired"
	err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("test-dlq-ttl", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, dlk, dlx, false, nil)
	require.NoError(t, err)

	// Create main queue with DLX
	mainQueue, err := tc.Ch.QueueDeclare(
		"test-main-queue-ttl",
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

	// Publish message with 100ms TTL
	err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("will expire and go to DLX"),
		Expiration:  "100", // TTL in milliseconds (relative)
	})
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Start consumer on main queue - should get nothing
	mainMsgs := tc.StartConsumer(mainQueue.Name, "", true)
	tc.ExpectNoMessage(mainMsgs, 500*time.Millisecond)

	// Check DLQ - should have the expired message
	dlqMsgs := tc.StartConsumer(dlq.Name, "", false)
	deadMsg, ok := tc.ConsumeWithTimeout(dlqMsgs, 2*time.Second)
	require.True(t, ok, "should receive dead-lettered message")

	assert.Equal(t, []byte("will expire and go to DLX"), deadMsg.Body)

	// Verify x-death headers
	xDeath, ok := deadMsg.Headers["x-death"].([]interface{})
	require.True(t, ok, "x-death header should be an array")
	require.Len(t, xDeath, 1, "should have one death entry")

	deathEntry := xDeath[0].(amqp.Table)
	assert.Equal(t, "expired", deathEntry["reason"], "death reason should be 'expired'")
	assert.Equal(t, mainQueue.Name, deathEntry["queue"])
	assert.Equal(t, int64(1), deathEntry["count"]) // AMQP encodes uint64 as int64

	// Verify original-expiration is preserved
	originalExpiration, ok := deathEntry["original-expiration"].(string)
	require.True(t, ok, "original-expiration should be present")
	assert.NotEmpty(t, originalExpiration)

	// Verify x-first-death headers
	assert.Equal(t, mainQueue.Name, deadMsg.Headers["x-first-death-queue"])
	assert.Equal(t, "expired", deadMsg.Headers["x-first-death-reason"])

	// Ack the message
	err = deadMsg.Ack(false)
	require.NoError(t, err)
}

// TestTTL_PerQueueExpiration verifies per-queue TTL (x-message-ttl)
func TestTTL_PerQueueExpiration(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX infrastructure
	dlx := "test-dlx-queue-ttl"
	err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("test-dlq-queue-ttl", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, "expired", dlx, false, nil)
	require.NoError(t, err)

	// Create main queue with per-queue TTL (100ms) and DLX
	mainQueue, err := tc.Ch.QueueDeclare(
		"test-queue-ttl",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-message-ttl":             int32(100), // 100ms queue-level TTL
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": "expired",
		},
	)
	require.NoError(t, err)

	// Publish message WITHOUT per-message expiration (relies on queue TTL)
	err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("queue TTL test"),
	})
	require.NoError(t, err)

	// Wait for queue TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Main queue should be empty
	mainMsgs := tc.StartConsumer(mainQueue.Name, "", true)
	tc.ExpectNoMessage(mainMsgs, 500*time.Millisecond)

	// Check DLQ - should have expired message
	dlqMsgs := tc.StartConsumer(dlq.Name, "", false)
	deadMsg, ok := tc.ConsumeWithTimeout(dlqMsgs, 2*time.Second)
	require.True(t, ok, "should receive dead-lettered message")

	assert.Equal(t, []byte("queue TTL test"), deadMsg.Body)

	// Verify death reason is 'expired'
	xDeath := deadMsg.Headers["x-death"].([]interface{})
	deathEntry := xDeath[0].(amqp.Table)
	assert.Equal(t, "expired", deathEntry["reason"])

	err = deadMsg.Ack(false)
	require.NoError(t, err)
}

// TestTTL_PerMessageOverridesQueue verifies per-message TTL takes precedence over queue TTL
func TestTTL_PerMessageOverridesQueue(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX
	dlx := "test-dlx-override"
	err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("test-dlq-override", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, "expired", dlx, false, nil)
	require.NoError(t, err)

	// Create queue with 5 second queue TTL
	mainQueue, err := tc.Ch.QueueDeclare(
		"test-queue-override",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-message-ttl":             int32(5000), // 5 seconds queue TTL
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": "expired",
		},
	)
	require.NoError(t, err)

	// Publish message with 100ms per-message TTL (should override 5s queue TTL)
	err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("per-message override"),
		Expiration:  "100", // TTL in milliseconds (relative)
	})
	require.NoError(t, err)

	// Wait for per-message expiration (200ms, much less than 5s)
	time.Sleep(200 * time.Millisecond)

	// Try to consume from main queue (triggers lazy expiration check)
	mainMsgs := tc.StartConsumer(mainQueue.Name, "", true)
	tc.ExpectNoMessage(mainMsgs, 500*time.Millisecond)

	// Check DLQ - should already have the message (didn't wait 5 seconds)
	dlqMsgs := tc.StartConsumer(dlq.Name, "", false)
	deadMsg, ok := tc.ConsumeWithTimeout(dlqMsgs, 2*time.Second)
	require.True(t, ok, "message should expire by per-message TTL, not queue TTL")

	assert.Equal(t, []byte("per-message override"), deadMsg.Body)

	err = deadMsg.Ack(false)
	require.NoError(t, err)
}

// TestTTL_BasicGet_ExpiredMessage verifies expired messages are not returned by Basic.Get
func TestTTL_BasicGet_ExpiredMessage(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX
	dlx := "test-dlx-get"
	err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("test-dlq-get", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, "expired", dlx, false, nil)
	require.NoError(t, err)

	// Create main queue with DLX
	mainQueue, err := tc.Ch.QueueDeclare(
		"test-get-queue",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": "expired",
		},
	)
	require.NoError(t, err)

	// Publish message with 100ms TTL
	err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("get expired test"),
		Expiration:  "100", // TTL in milliseconds (relative)
	})
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Try Basic.Get - should return empty
	msg, ok, err := tc.Ch.Get(mainQueue.Name, true)
	require.NoError(t, err)
	assert.False(t, ok, "Get should return false for expired message")
	assert.Empty(t, msg.Body, "Get should return empty message")

	// Verify message went to DLQ
	dlqMsg, ok, err := tc.Ch.Get(dlq.Name, false)
	require.NoError(t, err)
	require.True(t, ok, "DLQ should have the expired message")
	assert.Equal(t, []byte("get expired test"), dlqMsg.Body)

	err = dlqMsg.Ack(false)
	require.NoError(t, err)
}

// TestTTL_MultipleMessages_DifferentExpirations verifies handling of multiple messages with different TTLs
func TestTTL_MultipleMessages_DifferentExpirations(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX
	dlx := "test-dlx-multi"
	err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("test-dlq-multi", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, "expired", dlx, false, nil)
	require.NoError(t, err)

	// Create main queue
	mainQueue, err := tc.Ch.QueueDeclare(
		"test-queue-multi",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": "expired",
		},
	)
	require.NoError(t, err)

	// Publish 3 messages with different TTLs
	messages := []struct {
		body string
		ttl  string
	}{
		{"expires in 100ms", "100"},
		{"expires in 200ms", "200"},
		{"expires in 300ms", "300"},
	}

	for _, msg := range messages {
		err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(msg.body),
			Expiration:  msg.ttl, // TTL in milliseconds (relative)
		})
		require.NoError(t, err)
	}

	// Wait for all to expire
	time.Sleep(400 * time.Millisecond)

	// Main queue should be empty
	mainMsgs := tc.StartConsumer(mainQueue.Name, "", true)
	tc.ExpectNoMessage(mainMsgs, 500*time.Millisecond)

	// DLQ should have all 3 messages
	dlqMsgs := tc.StartConsumer(dlq.Name, "", false)
	received := tc.DrainMessages(dlqMsgs, 3, 2*time.Second)
	require.Len(t, received, 3, "should receive all 3 expired messages in DLQ")

	// Ack all
	tc.AckAll(received)
}

// TestTTL_Requeue_ExpiredNotRequeued verifies expired messages are not requeued on nack
func TestTTL_Requeue_ExpiredNotRequeued(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX
	dlx := "test-dlx-requeue"
	err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("test-dlq-requeue", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, "expired", dlx, false, nil)
	require.NoError(t, err)

	// Create main queue
	mainQueue, err := tc.Ch.QueueDeclare(
		"test-queue-requeue",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": "expired",
		},
	)
	require.NoError(t, err)

	// Publish message with 200ms TTL
	err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("will expire before requeue"),
		Expiration:  "200", // TTL in milliseconds (relative)
	})
	require.NoError(t, err)

	// Consume the message
	msgs := tc.StartConsumer(mainQueue.Name, "", false)
	msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	require.True(t, ok, "should receive message")

	// Wait for expiration BEFORE nacking
	time.Sleep(250 * time.Millisecond)

	// Nack with requeue=true (but message is expired, so should go to DLX instead)
	err = msg.Nack(false, true)
	require.NoError(t, err)

	// Main queue should remain empty (expired message not requeued)
	time.Sleep(200 * time.Millisecond)
	tc.ExpectNoMessage(msgs, 500*time.Millisecond)

	// Check DLQ - should have the expired message
	dlqMsgs := tc.StartConsumer(dlq.Name, "", false)
	deadMsg, ok := tc.ConsumeWithTimeout(dlqMsgs, 2*time.Second)
	require.True(t, ok, "expired message should go to DLX, not requeue")

	assert.Equal(t, []byte("will expire before requeue"), deadMsg.Body)

	// Verify it died from expiration, not rejection
	xDeath := deadMsg.Headers["x-death"].([]interface{})
	deathEntry := xDeath[0].(amqp.Table)
	assert.Equal(t, "expired", deathEntry["reason"], "should be expired, not rejected")

	err = deadMsg.Ack(false)
	require.NoError(t, err)
}

// TestTTL_NoExpiration_MessageDelivered verifies messages without TTL are delivered normally
func TestTTL_NoExpiration_MessageDelivered(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	queue, err := tc.Ch.QueueDeclare("test-no-ttl", false, true, false, false, nil)
	require.NoError(t, err)

	// Publish message WITHOUT expiration
	err = tc.Ch.Publish("", queue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("no expiration"),
	})
	require.NoError(t, err)

	// Wait a bit
	time.Sleep(500 * time.Millisecond)

	// Should still be consumable
	msgs := tc.StartConsumer(queue.Name, "", false)
	msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	require.True(t, ok, "message without TTL should be delivered")
	assert.Equal(t, []byte("no expiration"), msg.Body)

	err = msg.Ack(false)
	require.NoError(t, err)
}

// TestTTL_PersistentMessage_DeadLettered verifies persistent expired messages are handled correctly
func TestTTL_PersistentMessage_DeadLettered(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Setup DLX
	dlx := "test-dlx-persistent"
	err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
	require.NoError(t, err)

	dlq, err := tc.Ch.QueueDeclare("test-dlq-persistent", false, true, false, false, nil)
	require.NoError(t, err)

	err = tc.Ch.QueueBind(dlq.Name, "expired", dlx, false, nil)
	require.NoError(t, err)

	// Create main queue
	mainQueue, err := tc.Ch.QueueDeclare(
		"test-queue-persistent",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": "expired",
		},
	)
	require.NoError(t, err)

	// Publish PERSISTENT message with TTL
	err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
		ContentType:  "text/plain",
		Body:         []byte("persistent message"),
		DeliveryMode: amqp.Persistent,
		Expiration:   "100", // TTL in milliseconds (relative)
	})
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Try to consume from main queue to trigger lazy expiration check
	mainMsgs := tc.StartConsumer(mainQueue.Name, "", true)
	tc.ExpectNoMessage(mainMsgs, 500*time.Millisecond)

	// Check DLQ
	dlqMsgs := tc.StartConsumer(dlq.Name, "", false)
	deadMsg, ok := tc.ConsumeWithTimeout(dlqMsgs, 2*time.Second)
	require.True(t, ok, "persistent expired message should be dead-lettered")

	assert.Equal(t, []byte("persistent message"), deadMsg.Body)
	assert.Equal(t, uint8(2), deadMsg.DeliveryMode, "delivery mode should be preserved")

	err = deadMsg.Ack(false)
	require.NoError(t, err)
}
