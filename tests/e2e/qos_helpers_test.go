package e2e

import (
	"testing"
	"time"
)

// Example test using the helper utilities
func TestQoS_WithHelpers_Example(t *testing.T) {
	// Create connection and channel
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Declare test queue
	queueName := testQueue + "-helpers-example"
	tc.DeclareQueue(queueName)

	// Set QoS
	tc.SetQoS(3, false) // prefetch=3, per-consumer

	// Publish messages
	tc.PublishMessages(queueName, 10)

	// Start consumer
	consumerTag := tc.UniqueConsumerTag("test-consumer")
	msgs := tc.StartConsumer(queueName, consumerTag, false)

	// Wait for exactly 3 messages (prefetch limit)
	received := tc.WaitForMessages(msgs, 3, 2*time.Second)

	// Verify we got 3 messages
	if len(received) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(received))
	}

	// Verify no more messages arrive (prefetch limit reached)
	tc.ExpectNoMessage(msgs, 500*time.Millisecond)

	// Ack first message
	if err := received[0].Ack(false); err != nil {
		t.Fatalf("Failed to ack: %v", err)
	}

	// Should receive one more message
	msg, ok := tc.ConsumeWithTimeout(msgs, 1*time.Second)
	if !ok {
		t.Error("Expected to receive message after ack")
	} else {
		t.Logf("Received message after ack: %s", string(msg.Body))
	}
}
