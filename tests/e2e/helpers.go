package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TestConnection manages a connection to the broker for testing
type TestConnection struct {
	Conn *amqp.Connection
	Ch   *amqp.Channel
	t    *testing.T
}

// NewTestConnection creates a new connection and channel for testing
func NewTestConnection(t *testing.T, brokerURL string) *TestConnection {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect to broker: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		t.Fatalf("Failed to open channel: %v", err)
	}

	return &TestConnection{
		Conn: conn,
		Ch:   ch,
		t:    t,
	}
}

// Close closes the channel and connection
func (tc *TestConnection) Close() {
	if tc.Ch != nil {
		tc.Ch.Close()
	}
	if tc.Conn != nil {
		tc.Conn.Close()
	}
}

// DeclareQueue declares a test queue with auto-delete enabled
func (tc *TestConnection) DeclareQueue(name string) amqp.Queue {
	q, err := tc.Ch.QueueDeclare(
		name,
		false, // durable
		true,  // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	if err != nil {
		tc.t.Fatalf("Failed to declare queue %s: %v", name, err)
	}
	return q
}

// PublishMessages publishes multiple messages to a queue
func (tc *TestConnection) PublishMessages(queueName string, count int) {
	for i := 0; i < count; i++ {
		err := tc.Ch.PublishWithContext(
			context.Background(),
			"",        // exchange
			queueName, // routing key
			false,     // mandatory
			false,     // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(fmt.Sprintf("Message %d", i)),
			},
		)
		if err != nil {
			tc.t.Fatalf("Failed to publish message %d: %v", i, err)
		}
	}
}

// ConsumeWithTimeout attempts to consume a message with a timeout
func (tc *TestConnection) ConsumeWithTimeout(msgs <-chan amqp.Delivery, timeout time.Duration) (amqp.Delivery, bool) {
	select {
	case msg := <-msgs:
		return msg, true
	case <-time.After(timeout):
		return amqp.Delivery{}, false
	}
}

// ExpectNoMessage verifies that no message arrives within the timeout
func (tc *TestConnection) ExpectNoMessage(msgs <-chan amqp.Delivery, timeout time.Duration) {
	select {
	case msg := <-msgs:
		tc.t.Errorf("Unexpected message received: %s", string(msg.Body))
	case <-time.After(timeout):
		// Expected behavior
	}
}

// SetQoS sets QoS parameters on the channel
func (tc *TestConnection) SetQoS(prefetchCount int, global bool) {
	err := tc.Ch.Qos(prefetchCount, 0, global)
	if err != nil {
		tc.t.Fatalf("Failed to set QoS (prefetch=%d, global=%v): %v", prefetchCount, global, err)
	}
}

// StartConsumer starts a consumer and returns the message channel
func (tc *TestConnection) StartConsumer(queueName, consumerTag string, autoAck bool) <-chan amqp.Delivery {
	msgs, err := tc.Ch.Consume(
		queueName,
		consumerTag,
		autoAck,
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		tc.t.Fatalf("Failed to start consumer %s: %v", consumerTag, err)
	}
	return msgs
}

// DrainMessages consumes and counts messages up to maxCount or timeout
func (tc *TestConnection) DrainMessages(msgs <-chan amqp.Delivery, maxCount int, timeout time.Duration) []amqp.Delivery {
	var received []amqp.Delivery
	deadline := time.After(timeout)

	for len(received) < maxCount {
		select {
		case msg := <-msgs:
			received = append(received, msg)
		case <-deadline:
			return received
		}
	}
	return received
}

// WaitForMessages waits for exactly count messages, failing if timeout occurs
func (tc *TestConnection) WaitForMessages(msgs <-chan amqp.Delivery, count int, timeout time.Duration) []amqp.Delivery {
	var received []amqp.Delivery
	deadline := time.After(timeout)

	for len(received) < count {
		select {
		case msg := <-msgs:
			received = append(received, msg)
			tc.t.Logf("Received message %d/%d: %s", len(received), count, string(msg.Body))
		case <-deadline:
			tc.t.Fatalf("Timeout waiting for messages: expected %d, got %d", count, len(received))
		}
	}
	return received
}

// AckAll acknowledges all messages in the slice
func (tc *TestConnection) AckAll(msgs []amqp.Delivery) {
	for _, msg := range msgs {
		if err := msg.Ack(false); err != nil {
			tc.t.Errorf("Failed to ack message: %v", err)
		}
	}
}

// UniqueQueueName generates a unique queue name based on the test name
func (tc *TestConnection) UniqueQueueName(prefix string) string {
	testName := tc.t.Name()
	// Replace slashes and other problematic chars with hyphens
	safeName := strings.ReplaceAll(testName, "/", "-")
	safeName = strings.ReplaceAll(safeName, " ", "-")
	return fmt.Sprintf("%s-%s", prefix, safeName)
}

// UniqueConsumerTag generates a unique consumer tag based on the test name
func (tc *TestConnection) UniqueConsumerTag(prefix string) string {
	testName := tc.t.Name()
	safeName := strings.ReplaceAll(testName, "/", "-")
	safeName = strings.ReplaceAll(safeName, " ", "-")
	return fmt.Sprintf("%s-%s", prefix, safeName)
}
