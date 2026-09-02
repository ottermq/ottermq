package e2e

import (
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
)

// TestDLX_RejectAfterDLQDeleted reproduces a broker panic that occurred when:
//  1. A queue has a dead-letter-exchange (DLX) configured
//  2. The dead-letter target queue is deleted (closing its internal channel)
//  3. A message is rejected with requeue=false, triggering DLX routing
//
// Before the fix: broker panics with "send on closed channel" followed by
// "sync: unlock of unlocked mutex", crashing the process.
// After the fix: the broker discards the dead-lettered message gracefully and
// remains alive for subsequent connections.
func TestDLX_RejectAfterDLQDeleted(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	dlxName := tc.UniqueName("dlx")
	dlk := "dead"
	dlqName := tc.UniqueName("dlq")
	srcName := tc.UniqueName("src")

	// --- Setup ---

	// DLX exchange
	err := tc.Ch.ExchangeDeclare(dlxName, "direct", false, true, false, false, nil)
	require.NoError(t, err, "declare DLX exchange")

	// Dead-letter queue (non-auto-delete so QueueDelete controls its lifetime)
	_, err = tc.Ch.QueueDeclare(dlqName, false, false, false, false, nil)
	require.NoError(t, err, "declare DLQ")
	err = tc.Ch.QueueBind(dlqName, dlk, dlxName, false, nil)
	require.NoError(t, err, "bind DLQ to DLX")

	// Source queue with DLX configured
	_, err = tc.Ch.QueueDeclare(srcName, false, true, false, false, amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": dlk,
	})
	require.NoError(t, err, "declare source queue")

	// --- Publish and consume ---

	err = tc.Ch.Publish("", srcName, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("will-be-dead-lettered"),
	})
	require.NoError(t, err, "publish message")

	tc.SetQoS(1, false)
	msgs := tc.StartConsumer(srcName, tc.UniqueConsumerTag("consumer"), false)

	msg, ok := tc.ConsumeWithTimeout(msgs, 2*time.Second)
	require.True(t, ok, "should receive message from source queue")

	// --- Delete the DLQ while the message is unacked ---
	// This closes the DLQ's internal channel. Any subsequent push to it panics
	// unless the broker guards against writing to a closed channel.
	_, err = tc.Ch.QueueDelete(dlqName, false, false, false)
	require.NoError(t, err, "delete DLQ")

	// Small gap to ensure the deletion is committed before the reject lands
	time.Sleep(20 * time.Millisecond)

	// --- Reject: triggers dead-letter routing to the now-deleted DLQ ---
	// Before fix: broker goroutine panics → process exits → this connection dies.
	// After fix: broker discards the dead-lettered message and stays alive.
	err = msg.Reject(false)
	require.NoError(t, err, "reject message")

	// Give the broker time to process the rejection
	time.Sleep(150 * time.Millisecond)

	// --- Verify the broker is still alive ---
	// Open a fresh connection. If the broker crashed, Dial returns an error.
	probe, err := amqp.Dial(brokerURL)
	require.NoError(t, err, "broker should still accept connections after reject-to-deleted-DLQ")
	defer probe.Close()

	probeCh, err := probe.Channel()
	require.NoError(t, err, "broker should still open channels after reject-to-deleted-DLQ")
	defer probeCh.Close()

	// Do a benign operation to confirm the broker is fully functional
	_, err = probeCh.QueueDeclare(tc.UniqueName("probe"), false, true, false, false, nil)
	require.NoError(t, err, "broker should handle normal operations after reject-to-deleted-DLQ")
}
