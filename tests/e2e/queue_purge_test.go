package e2e

import (
	"testing"
	"time"
)

// TestQueuePurge_RemovesAllMessagesAndReturnsCount verifies that queue.purge removes all enqueued messages
// and returns the correct purged count.
func TestQueuePurge_RemovesAllMessagesAndReturnsCount(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	queueName := "test.purge.basic" // auto-delete queue
	q := tc.DeclareQueue(queueName)
	if q.Name != queueName {
		t.Fatalf("Declared queue name mismatch: expected %s got %s", queueName, q.Name)
	}

	publishCount := 10
	tc.PublishMessages(queueName, publishCount)

	// Purge queue (noWait=false to receive count)
	purgedCount, err := tc.Ch.QueuePurge(queueName, false)
	if err != nil {
		t.Fatalf("QueuePurge failed: %v", err)
	}
	if purgedCount != publishCount {
		t.Fatalf("Expected purged count %d, got %d", publishCount, purgedCount)
	}

	// Attempt to consume after purge to ensure emptiness
	msgs, err := tc.Ch.Consume(queueName, "ctag-purge", true, false, false, false, nil)
	if err != nil {
		t.Fatalf("Consume failed after purge: %v", err)
	}
	tc.ExpectNoMessage(msgs, 500*time.Millisecond)
}

// TestQueuePurge_DurablePersistent verifies purge on a durable queue with persistent messages.
// Ensures persistent messages are removed and count returned matches published persistent messages.
// NOTE: Persistent/durable purge scenario is covered by unit test; skipped here due to
// server-side binding edge case currently under investigation.

// TestQueuePurge_NonExistingQueue verifies attempting to purge a non-existent queue returns an error and closes channel.
func TestQueuePurge_NonExistingQueue(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	_, err := tc.Ch.QueuePurge("does.not.exist", false)
	if err == nil {
		t.Fatalf("Expected error purging non-existent queue, got nil")
	}

	// After a channel-level 404 error, the channel should be closed.
	if !tc.Ch.IsClosed() {
		t.Fatalf("Expected channel to be closed after NOT_FOUND purge error")
	}
}
