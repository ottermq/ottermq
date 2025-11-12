package e2e

import (
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
)

// TestDLX_Simple is a minimal test to verify DLX is working
func TestDLX_Simple(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Create simple DLX setup
	dlx := "simple-dlx"
	dlk := "dead"

	err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
	require.NoError(t, err, "Failed to declare DLX exchange")

	dlq, err := tc.Ch.QueueDeclare("simple-dlq", false, true, false, false, nil)
	require.NoError(t, err, "Failed to declare DLQ")
	err = tc.Ch.QueueBind(dlq.Name, dlk, dlx, false, nil)
	require.NoError(t, err, "Failed to bind DLQ")

	// Create main queue with DLX
	mainQueue, err := tc.Ch.QueueDeclare(
		"simple-main",
		false,
		true,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    dlx,
			"x-dead-letter-routing-key": dlk,
		},
	)
	require.NoError(t, err, "Failed to declare main queue")

	t.Logf("Created main queue: %s", mainQueue.Name)
	t.Logf("Created DLQ: %s", dlq.Name)

	// Publish a message
	err = tc.Ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
		Body: []byte("test"),
	})
	require.NoError(t, err, "Failed to publish")

	t.Log("Published message")

	// Consume it
	msgs := tc.StartConsumer(mainQueue.Name, "test", false)
	msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	require.True(t, ok, "Should receive message")

	t.Logf("Received message: %s", string(msg.Body))

	// Reject it
	err = msg.Reject(false)
	require.NoError(t, err, "Failed to reject")

	t.Log("Rejected message")

	time.Sleep(500 * time.Millisecond) // Give time for dead-lettering

	// Try to consume from DLQ
	t.Log("Attempting to consume from DLQ...")
	dlqMsgs := tc.StartConsumer(dlq.Name, "dlq-consumer", true)

	deadMsg, ok := tc.ConsumeWithTimeout(dlqMsgs, 3*time.Second)
	require.True(t, ok, "Should receive dead-lettered message in DLQ")

	t.Logf("Received from DLQ: %s", string(deadMsg.Body))
	t.Logf("Headers: %+v", deadMsg.Headers)
}
