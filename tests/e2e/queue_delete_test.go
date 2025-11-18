package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestQueueDelete_IfUnused_WithActiveConsumer_ClosesChannel(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	q := tc.DeclareQueue("e2e.qdel.ifunused")

	// Start a consumer so queue is in-use
	msgs := tc.StartConsumer(q.Name, "ctag-qdel", true)
	_ = msgs

	// Attempt delete with ifUnused=true
	_, err := tc.Ch.QueueDelete(q.Name, true, false, false)
	if err == nil {
		t.Fatalf("expected error when deleting in-use queue with ifUnused=true")
	}

	// Channel should be closed after channel exception
	if !tc.Ch.IsClosed() {
		t.Fatalf("expected channel to be closed after delete precondition failure")
	}
}

func TestQueueDelete_IfEmpty_WithMessages_ClosesChannel(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	q := tc.DeclareQueue("e2e.qdel.ifempty")
	tc.PublishMessages(q.Name, 1)

	// Attempt delete with ifEmpty=true
	_, err := tc.Ch.QueueDelete(q.Name, false, true, false)
	if err == nil {
		t.Fatalf("expected error when deleting non-empty queue with ifEmpty=true")
	}

	if !tc.Ch.IsClosed() {
		t.Fatalf("expected channel to be closed after delete precondition failure")
	}
}

func TestQueueDelete_Success_ReturnsMessageCount(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	q := tc.DeclareQueue("e2e.qdel.ok")
	tc.PublishMessages(q.Name, 3)

	// Delete with flags disabled, expect count of removed messages
	deletedCount, err := tc.Ch.QueueDelete(q.Name, false, false, false)
	if err != nil {
		t.Fatalf("QueueDelete failed: %v", err)
	}
	if deletedCount != 3 {
		t.Fatalf("expected deleted count 3, got %d", deletedCount)
	}
}

// TestQueueDelete_Exclusive_NonOwner_AccessRefused verifies that attempting to delete
// an exclusive queue from a connection other than the owner results in ACCESS_REFUSED error.
func TestQueueDelete_Exclusive_NonOwner_AccessRefused(t *testing.T) {

	// Use unique queue name to avoid conflicts
	queueName := fmt.Sprintf("e2e.qdel.excl.%d", time.Now().UnixNano())

	// Owner connection declares non-auto-delete exclusive queue
	ownerConn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("failed to connect owner: %v", err)
	}
	defer ownerConn.Close()
	ownerCh, err := ownerConn.Channel()
	if err != nil {
		t.Fatalf("failed to open owner channel: %v", err)
	}
	defer ownerCh.Close()

	// Declare durable=false, autoDelete=false, exclusive=true
	q, err := ownerCh.QueueDeclare(queueName, false, false, true, false, nil)
	if err != nil {
		t.Fatalf("failed to declare exclusive queue: %v", err)
	}

	// Foreign connection attempts delete
	foreignConn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("failed to connect foreign: %v", err)
	}
	defer foreignConn.Close()
	foreignCh, err := foreignConn.Channel()
	if err != nil {
		t.Fatalf("failed to open foreign channel: %v", err)
	}
	defer foreignCh.Close()

	_, err = foreignCh.QueueDelete(q.Name, false, false, false)
	if err == nil {
		t.Fatalf("expected error when non-owner deletes exclusive queue")
	}

	// Verify we got ACCESS_REFUSED error
	amqpErr, ok := err.(*amqp.Error)
	if !ok {
		t.Fatalf("expected AMQP error, got: %T %v", err, err)
	}
	if amqpErr.Code != amqp.AccessRefused {
		t.Errorf("expected ACCESS_REFUSED (403), got code %d: %s", amqpErr.Code, amqpErr.Reason)
	}
	if !strings.Contains(strings.ToLower(amqpErr.Reason), "exclusive") {
		t.Errorf("expected 'exclusive' in error reason, got: %s", amqpErr.Reason)
	}

	// Clean up: owner deletes the queue
	_, err = ownerCh.QueueDelete(q.Name, false, false, false)
	if err != nil {
		t.Logf("Warning: failed to cleanup queue: %v", err)
	}
}
