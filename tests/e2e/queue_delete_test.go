package e2e

import (
	"strings"
	"testing"

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

func TestQueueDelete_Exclusive_NonOwner_AccessRefused(t *testing.T) {
	// Owner connection declares exclusive queue
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

	q, err := ownerCh.QueueDeclare("e2e.qdel.excl", false, true, true, false, nil)
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
	// do not defer close here because we expect it to be closed by broker

	_, err = foreignCh.QueueDelete(q.Name, false, false, false)
	if err == nil {
		t.Fatalf("expected error when non-owner deletes exclusive queue")
	}
	// RabbitMQ-style error code is 403; we at least verify reason
	if amqpErr, ok := err.(*amqp.Error); ok {
		if amqpErr.Code != amqp.AccessRefused && !strings.Contains(strings.ToLower(amqpErr.Reason), "exclusive") {
			t.Fatalf("unexpected error: code=%d reason=%s", amqpErr.Code, amqpErr.Reason)
		}
	}

	if !foreignCh.IsClosed() {
		t.Fatalf("expected foreign channel to be closed after access-refused")
	}
}
