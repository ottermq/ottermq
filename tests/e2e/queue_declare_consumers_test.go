package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Regression test for #335: queue.declare-ok reports the queue's consumer
// count, so a passive declare can tell whether anything consumes from it.
func TestQueueDeclare_ReportsConsumerCount(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	queueName := tc.UniqueQueueName("e2e.declare.consumers")
	q, err := tc.Ch.QueueDeclare(queueName, false, false, false, false, nil)
	require.NoError(t, err)
	require.Equal(t, 0, q.Consumers, "new queue")
	defer func() {
		_, _ = tc.Ch.QueueDelete(queueName, false, false, false)
	}()

	ch1, err := tc.Conn.Channel()
	require.NoError(t, err)
	defer ch1.Close()
	ch2, err := tc.Conn.Channel()
	require.NoError(t, err)
	defer ch2.Close()

	tag1 := tc.UniqueConsumerTag("declare-c1")
	_, err = ch1.Consume(queueName, tag1, true, false, false, false, nil)
	require.NoError(t, err)
	_, err = ch2.Consume(queueName, tc.UniqueConsumerTag("declare-c2"), true, false, false, false, nil)
	require.NoError(t, err)

	q, err = tc.Ch.QueueDeclarePassive(queueName, false, false, false, false, nil)
	require.NoError(t, err)
	require.Equal(t, 2, q.Consumers, "passive declare with two consumers")

	q, err = tc.Ch.QueueDeclare(queueName, false, false, false, false, nil)
	require.NoError(t, err)
	require.Equal(t, 2, q.Consumers, "redeclare with two consumers")

	require.NoError(t, ch1.Cancel(tag1, false))

	q, err = tc.Ch.QueueDeclarePassive(queueName, false, false, false, false, nil)
	require.NoError(t, err)
	require.Equal(t, 1, q.Consumers, "passive declare after one cancel")
}
