package vhost

import (
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoOpDeadLetterer(t *testing.T) {
	dl := &NoOpDeadLetterer{}
	msg := amqp.Message{
		RoutingKey: "test.key",
		Body:       []byte("test message"),
	}
	args := map[string]any{
		"x-dead-letter-exchange": "dlx",
	}
	queue := &Queue{
		Name: "test-queue",
		Props: &QueueProperties{
			Arguments: args,
		},
	}

	err := dl.DeadLetter(msg, queue, REASON_REJECTED)
	assert.NoError(t, err, "NoOpDeadLetterer should never return error")
}

func TestDeadLetter_BasicRejection(t *testing.T) {
	vh := NewVhost("test-vhost", VHostOptions{
		Persistence:     &dummy.DummyPersistence{},
		QueueBufferSize: 100,
		EnableDLX:       true,
	})

	// Create DLX exchange and queue
	err := vh.CreateExchange("dlx", "direct", nil)
	require.NoError(t, err)

	dlqProps := NewQueueProperties()
	_, err = vh.CreateQueue("dead-letter-queue", dlqProps, nil)
	require.NoError(t, err)

	err = vh.BindQueue("dlx", "dead-letter-queue", "dead", nil, nil)
	require.NoError(t, err)

	// Create main queue with DLX
	mainProps := NewQueueProperties()
	mainProps.Arguments = map[string]any{
		"x-dead-letter-exchange":    "dlx",
		"x-dead-letter-routing-key": "dead",
	}
	mainQueue, err := vh.CreateQueue("main-queue", mainProps, nil)
	require.NoError(t, err)

	// Create and dead-letter a message
	msg := amqp.Message{
		RoutingKey: "original.key",
		Exchange:   "amq.direct",
		Body:       []byte("test message"),
		Properties: amqp.BasicProperties{
			Headers: make(map[string]any),
		},
	}

	dl := &DeadLetter{vh: vh}
	err = dl.DeadLetter(msg, mainQueue, REASON_REJECTED)
	require.NoError(t, err)

	// Verify message was routed to DLQ
	dlq := vh.Queues["dead-letter-queue"]
	assert.Equal(t, 1, dlq.Len(), "Dead letter queue should have one message")

	// Pop and verify the dead-lettered message
	deadMsg := dlq.Pop()
	require.NotNil(t, deadMsg)
	assert.Equal(t, []byte("test message"), deadMsg.Body)
	assert.Equal(t, "dead", deadMsg.RoutingKey, "Routing key should be overridden")
	assert.Equal(t, "dlx", deadMsg.Exchange, "Exchange should be DLX")

	// Verify x-death headers
	headers := deadMsg.Properties.Headers
	assert.NotNil(t, headers["x-death"])
	assert.Equal(t, "main-queue", headers["x-first-death-queue"])
	assert.Equal(t, "rejected", headers["x-first-death-reason"])
	assert.Equal(t, "amq.direct", headers["x-first-death-exchange"])
	assert.Equal(t, "main-queue", headers["x-last-death-queue"])
	assert.Equal(t, "rejected", headers["x-last-death-reason"])
	assert.Equal(t, "amq.direct", headers["x-last-death-exchange"])

	xDeath, ok := headers["x-death"].([]map[string]any)
	require.True(t, ok, "x-death should be array of entries")
	require.Len(t, xDeath, 1, "Should have one death entry")

	firstDeath := xDeath[0]
	assert.Equal(t, "main-queue", firstDeath["queue"])
	assert.Equal(t, "rejected", firstDeath["reason"])
	assert.Equal(t, uint64(1), firstDeath["count"])
	assert.Equal(t, "amq.direct", firstDeath["exchange"])
	assert.NotEmpty(t, firstDeath["time"])

	routingKeys, ok := firstDeath["routing-keys"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"original.key"}, routingKeys)
}

func TestDeadLetter_NoRoutingKeyOverride(t *testing.T) {
	vh := NewVhost("test-vhost", VHostOptions{
		Persistence:     &dummy.DummyPersistence{},
		QueueBufferSize: 100,
		EnableDLX:       true,
	})

	// Create DLX exchange and queue
	err := vh.CreateExchange("dlx", "direct", nil)
	require.NoError(t, err)

	dlqProps := NewQueueProperties()
	_, err = vh.CreateQueue("dead-letter-queue", dlqProps, nil)
	require.NoError(t, err)

	err = vh.BindQueue("dlx", "dead-letter-queue", "original.key", nil, nil)
	require.NoError(t, err)

	// Create main queue with DLX but NO routing key override
	mainProps := NewQueueProperties()
	mainProps.Arguments = map[string]any{
		"x-dead-letter-exchange": "dlx",
	}
	mainQueue, err := vh.CreateQueue("main-queue", mainProps, nil)
	require.NoError(t, err)

	// Create and dead-letter a message
	msg := amqp.Message{
		RoutingKey: "original.key",
		Exchange:   "amq.direct",
		Body:       []byte("test message"),
		Properties: amqp.BasicProperties{
			Headers: make(map[string]any),
		},
	}

	dl := &DeadLetter{vh: vh}
	err = dl.DeadLetter(msg, mainQueue, REASON_REJECTED)
	require.NoError(t, err)

	// Verify message was routed with original routing key
	dlq := vh.Queues["dead-letter-queue"]
	assert.Equal(t, 1, dlq.Len())

	deadMsg := dlq.Pop()
	assert.Equal(t, "original.key", deadMsg.RoutingKey, "Should use original routing key")
}

func TestDeadLetter_MultipleDeaths(t *testing.T) {
	vh := NewVhost("test-vhost", VHostOptions{
		Persistence:     &dummy.DummyPersistence{},
		QueueBufferSize: 100,
		EnableDLX:       true,
	})

	// Create DLX chain: queue1 -> dlx1 -> queue2 -> dlx2 -> queue3
	err := vh.CreateExchange("dlx1", "direct", nil)
	require.NoError(t, err)
	err = vh.CreateExchange("dlx2", "direct", nil)
	require.NoError(t, err)

	// Queue 1 (DLX -> dlx1)
	props1 := NewQueueProperties()
	props1.Arguments = map[string]any{
		"x-dead-letter-exchange":    "dlx1",
		"x-dead-letter-routing-key": "to-queue2",
	}
	queue1, err := vh.CreateQueue("queue1", props1, nil)
	require.NoError(t, err)

	// Queue 2 (DLX -> dlx2)
	props2 := NewQueueProperties()
	props2.Arguments = map[string]any{
		"x-dead-letter-exchange":    "dlx2",
		"x-dead-letter-routing-key": "to-queue3",
	}
	queue2, err := vh.CreateQueue("queue2", props2, nil)
	require.NoError(t, err)

	// Queue 3 (no DLX)
	queue3, err := vh.CreateQueue("queue3", NewQueueProperties(), nil)
	require.NoError(t, err)

	// Bind queues
	err = vh.BindQueue("dlx1", "queue2", "to-queue2", nil, nil)
	require.NoError(t, err)
	err = vh.BindQueue("dlx2", "queue3", "to-queue3", nil, nil)
	require.NoError(t, err)

	// Create message and dead-letter it twice
	msg := amqp.Message{
		RoutingKey: "original.key",
		Exchange:   "amq.direct",
		Body:       []byte("test message"),
		Properties: amqp.BasicProperties{
			Headers: make(map[string]any),
		},
	}

	dl := &DeadLetter{vh: vh}

	// First death: queue1 -> queue2
	err = dl.DeadLetter(msg, queue1, REASON_REJECTED)
	require.NoError(t, err)

	// Get message from queue2
	assert.Equal(t, 1, queue2.Len())
	msg2 := queue2.Pop()
	require.NotNil(t, msg2)

	// Verify first death headers
	headers := msg2.Properties.Headers
	assert.Equal(t, "queue1", headers["x-first-death-queue"])
	assert.Equal(t, "rejected", headers["x-first-death-reason"])
	assert.Equal(t, "queue1", headers["x-last-death-queue"])

	xDeath, ok := headers["x-death"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, xDeath, 1)
	assert.Equal(t, uint64(1), xDeath[0]["count"])

	// Second death: queue2 -> queue3
	err = dl.DeadLetter(*msg2, queue2, REASON_REJECTED)
	require.NoError(t, err)

	// Get message from queue3
	assert.Equal(t, 1, queue3.Len())
	msg3 := queue3.Pop()
	require.NotNil(t, msg3)

	// Verify x-first-death still points to queue1
	headers = msg3.Properties.Headers
	assert.Equal(t, "queue1", headers["x-first-death-queue"], "x-first-death should track the very first death")
	assert.Equal(t, "rejected", headers["x-first-death-reason"])

	// x-last-death should point to queue2 (the previous death)
	assert.Equal(t, "queue2", headers["x-last-death-queue"])
	assert.Equal(t, "rejected", headers["x-last-death-reason"])

	// Verify x-death array has 2 entries, newest first
	xDeath, ok = headers["x-death"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, xDeath, 2, "Should have two death entries")

	// First entry (index 0) should be the most recent death (queue2)
	assert.Equal(t, "queue2", xDeath[0]["queue"])
	assert.Equal(t, uint64(2), xDeath[0]["count"])

	// Second entry (index 1) should be the original death (queue1)
	assert.Equal(t, "queue1", xDeath[1]["queue"])
	assert.Equal(t, uint64(1), xDeath[1]["count"])
}

func TestDeadLetter_CCBCCHeaders(t *testing.T) {
	vh := NewVhost("test-vhost", VHostOptions{
		Persistence:     &dummy.DummyPersistence{},
		QueueBufferSize: 100,
		EnableDLX:       true,
	})

	err := vh.CreateExchange("dlx", "direct", nil)
	require.NoError(t, err)

	dlqProps := NewQueueProperties()
	_, err = vh.CreateQueue("dlq", dlqProps, nil)
	require.NoError(t, err)

	err = vh.BindQueue("dlx", "dlq", "dead", nil, nil)
	require.NoError(t, err)

	mainProps := NewQueueProperties()
	mainProps.Arguments = map[string]any{
		"x-dead-letter-exchange":    "dlx",
		"x-dead-letter-routing-key": "dead",
	}
	mainQueue, err := vh.CreateQueue("main-queue", mainProps, nil)
	require.NoError(t, err)

	// Create message with CC and BCC headers
	msg := amqp.Message{
		RoutingKey: "original.key",
		Exchange:   "amq.direct",
		Body:       []byte("test message"),
		Properties: amqp.BasicProperties{
			Headers: map[string]any{
				"CC":  []string{"cc.key1", "cc.key2"},
				"BCC": []string{"bcc.key1"},
			},
		},
	}

	dl := &DeadLetter{vh: vh}
	err = dl.DeadLetter(msg, mainQueue, REASON_REJECTED)
	require.NoError(t, err)

	// Verify CC and BCC are included in routing-keys
	dlq := vh.Queues["dlq"]
	deadMsg := dlq.Pop()
	require.NotNil(t, deadMsg)

	xDeath := deadMsg.Properties.Headers["x-death"].([]map[string]any)
	routingKeys, ok := xDeath[0]["routing-keys"].([]string)
	require.True(t, ok)

	expected := []string{"original.key", "cc.key1", "cc.key2", "bcc.key1"}
	assert.Equal(t, expected, routingKeys, "Should include original key, CC, and BCC")
}

func TestDeadLetter_ExpirationCleared(t *testing.T) {
	vh := NewVhost("test-vhost", VHostOptions{
		Persistence:     &dummy.DummyPersistence{},
		QueueBufferSize: 100,
		EnableDLX:       true,
	})

	err := vh.CreateExchange("dlx", "direct", nil)
	require.NoError(t, err)

	dlqProps := NewQueueProperties()
	_, err = vh.CreateQueue("dlq", dlqProps, nil)
	require.NoError(t, err)

	err = vh.BindQueue("dlx", "dlq", "dead", nil, nil)
	require.NoError(t, err)

	mainProps := NewQueueProperties()
	mainProps.Arguments = map[string]any{
		"x-dead-letter-exchange":    "dlx",
		"x-dead-letter-routing-key": "dead",
	}
	mainQueue, err := vh.CreateQueue("main-queue", mainProps, nil)
	require.NoError(t, err)

	// Create message with expiration
	msg := amqp.Message{
		RoutingKey: "original.key",
		Exchange:   "amq.direct",
		Body:       []byte("test message"),
		Properties: amqp.BasicProperties{
			Headers:    make(map[string]any),
			Expiration: "60000", // 60 seconds in milliseconds
		},
	}

	dl := &DeadLetter{vh: vh}
	err = dl.DeadLetter(msg, mainQueue, REASON_REJECTED)
	require.NoError(t, err)

	// Verify expiration is cleared but preserved in x-death
	dlq := vh.Queues["dlq"]
	deadMsg := dlq.Pop()
	require.NotNil(t, deadMsg)

	assert.Empty(t, deadMsg.Properties.Expiration, "Expiration should be cleared")

	xDeath := deadMsg.Properties.Headers["x-death"].([]map[string]any)
	assert.Equal(t, "60000", xDeath[0]["original-expiration"], "Original expiration should be preserved in x-death")
}

func TestDeadLetter_DifferentReasons(t *testing.T) {
	vh := NewVhost("test-vhost", VHostOptions{
		Persistence:     &dummy.DummyPersistence{},
		QueueBufferSize: 100,
		EnableDLX:       true,
	})

	err := vh.CreateExchange("dlx", "direct", nil)
	require.NoError(t, err)

	dlqProps := NewQueueProperties()
	_, err = vh.CreateQueue("dlq", dlqProps, nil)
	require.NoError(t, err)

	err = vh.BindQueue("dlx", "dlq", "dead", nil, nil)
	require.NoError(t, err)

	mainProps := NewQueueProperties()
	mainProps.Arguments = map[string]any{
		"x-dead-letter-exchange":    "dlx",
		"x-dead-letter-routing-key": "dead",
	}
	mainQueue, err := vh.CreateQueue("main-queue", mainProps, nil)
	require.NoError(t, err)

	dl := &DeadLetter{vh: vh}

	testCases := []struct {
		reason   ReasonType
		expected string
	}{
		{REASON_REJECTED, "rejected"},
		{REASON_EXPIRED, "expired"},
		{REASON_MAX_LENGTH, "maxlen"},
		{REASON_DELIVERY_LIMIT, "delivery_limit"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.reason), func(t *testing.T) {
			msg := amqp.Message{
				RoutingKey: "test.key",
				Exchange:   "amq.direct",
				Body:       []byte("test"),
				Properties: amqp.BasicProperties{
					Headers: make(map[string]any),
				},
			}

			err := dl.DeadLetter(msg, mainQueue, tc.reason)
			require.NoError(t, err)

			dlq := vh.Queues["dlq"]
			deadMsg := dlq.Pop()
			require.NotNil(t, deadMsg)

			headers := deadMsg.Properties.Headers
			assert.Equal(t, tc.expected, headers["x-first-death-reason"])
			assert.Equal(t, tc.expected, headers["x-last-death-reason"])

			xDeath := headers["x-death"].([]map[string]any)
			assert.Equal(t, tc.expected, xDeath[0]["reason"])
		})
	}
}

func TestDeadLetter_NilHeaders(t *testing.T) {
	vh := NewVhost("test-vhost", VHostOptions{
		Persistence:     &dummy.DummyPersistence{},
		QueueBufferSize: 100,
		EnableDLX:       true,
	})

	err := vh.CreateExchange("dlx", "direct", nil)
	require.NoError(t, err)

	dlqProps := NewQueueProperties()
	_, err = vh.CreateQueue("dlq", dlqProps, nil)
	require.NoError(t, err)

	err = vh.BindQueue("dlx", "dlq", "dead", nil, nil)
	require.NoError(t, err)

	mainProps := NewQueueProperties()
	mainProps.Arguments = map[string]any{
		"x-dead-letter-exchange":    "dlx",
		"x-dead-letter-routing-key": "dead",
	}
	mainQueue, err := vh.CreateQueue("main-queue", mainProps, nil)
	require.NoError(t, err)

	// Create message with nil headers
	msg := amqp.Message{
		RoutingKey: "test.key",
		Exchange:   "amq.direct",
		Body:       []byte("test"),
		Properties: amqp.BasicProperties{
			Headers: nil, // Explicitly nil
		},
	}

	dl := &DeadLetter{vh: vh}
	err = dl.DeadLetter(msg, mainQueue, REASON_REJECTED)
	require.NoError(t, err)

	// Should not panic and should create headers map
	dlq := vh.Queues["dlq"]
	deadMsg := dlq.Pop()
	require.NotNil(t, deadMsg)
	assert.NotNil(t, deadMsg.Properties.Headers, "Headers should be created if nil")
	assert.NotNil(t, deadMsg.Properties.Headers["x-death"])
}

func TestDeadLetter_DisabledFeature(t *testing.T) {
	vh := NewVhost("test-vhost", VHostOptions{
		Persistence:     &dummy.DummyPersistence{},
		QueueBufferSize: 100,
		EnableDLX:       false, // DLX disabled
	})

	// DeadLetterer should be NoOpDeadLetterer
	_, ok := vh.DeadLetterer.(*NoOpDeadLetterer)
	assert.True(t, ok, "Should use NoOpDeadLetterer when DLX is disabled")

	// Create queue with DLX arguments (they will be ignored)
	mainProps := NewQueueProperties()
	mainProps.Arguments = map[string]any{
		"x-dead-letter-exchange": "dlx",
	}
	mainQueue, err := vh.CreateQueue("main-queue", mainProps, nil)
	require.NoError(t, err)

	msg := amqp.Message{
		RoutingKey: "test.key",
		Body:       []byte("test"),
	}

	// Should not panic or error
	err = vh.DeadLetterer.DeadLetter(msg, mainQueue, REASON_REJECTED)
	assert.NoError(t, err)
}

func TestDeadLetter_TopicExchange(t *testing.T) {
	vh := NewVhost("test-vhost", VHostOptions{
		Persistence:     &dummy.DummyPersistence{},
		QueueBufferSize: 100,
		EnableDLX:       true,
	})

	// Create DLX as topic exchange
	err := vh.CreateExchange("dlx", "topic", nil)
	require.NoError(t, err)

	// Create multiple DLQs bound with different patterns
	dlq1Props := NewQueueProperties()
	_, err = vh.CreateQueue("dlq.errors", dlq1Props, nil)
	require.NoError(t, err)

	dlq2Props := NewQueueProperties()
	_, err = vh.CreateQueue("dlq.all", dlq2Props, nil)
	require.NoError(t, err)

	err = vh.BindQueue("dlx", "dlq.errors", "error.#", nil, nil)
	require.NoError(t, err)
	err = vh.BindQueue("dlx", "dlq.all", "#", nil, nil)
	require.NoError(t, err)

	// Create main queue with topic routing key override
	mainProps := NewQueueProperties()
	mainProps.Arguments = map[string]any{
		"x-dead-letter-exchange":    "dlx",
		"x-dead-letter-routing-key": "error.critical.timeout",
	}
	mainQueue, err := vh.CreateQueue("main-queue", mainProps, nil)
	require.NoError(t, err)

	msg := amqp.Message{
		RoutingKey: "original.key",
		Exchange:   "amq.direct",
		Body:       []byte("test message"),
		Properties: amqp.BasicProperties{
			Headers: make(map[string]any),
		},
	}

	dl := &DeadLetter{vh: vh}
	err = dl.DeadLetter(msg, mainQueue, REASON_REJECTED)
	require.NoError(t, err)

	// Both DLQs should receive the message due to topic routing
	dlq1 := vh.Queues["dlq.errors"]
	dlq2 := vh.Queues["dlq.all"]

	assert.Equal(t, 1, dlq1.Len(), "error.# pattern should match error.critical.timeout")
	assert.Equal(t, 1, dlq2.Len(), "# pattern should match everything")
}

func TestDeadLetter_PersistentMessage(t *testing.T) {
	vh := NewVhost("test-vhost", VHostOptions{
		Persistence:     &dummy.DummyPersistence{},
		QueueBufferSize: 100,
		EnableDLX:       true,
	})

	err := vh.CreateExchange("dlx", "direct", nil)
	require.NoError(t, err)

	dlqProps := NewQueueProperties()
	dlqProps.Durable = true
	_, err = vh.CreateQueue("dlq", dlqProps, nil)
	require.NoError(t, err)

	err = vh.BindQueue("dlx", "dlq", "dead", nil, nil)
	require.NoError(t, err)

	mainProps := NewQueueProperties()
	mainProps.Durable = true
	mainProps.Arguments = map[string]any{
		"x-dead-letter-exchange":    "dlx",
		"x-dead-letter-routing-key": "dead",
	}
	mainQueue, err := vh.CreateQueue("main-queue", mainProps, nil)
	require.NoError(t, err)

	// Create persistent message
	msg := amqp.Message{
		RoutingKey: "original.key",
		Exchange:   "amq.direct",
		Body:       []byte("persistent test message"),
		Properties: amqp.BasicProperties{
			Headers:      make(map[string]any),
			DeliveryMode: 2, // Persistent
		},
	}

	dl := &DeadLetter{vh: vh}
	err = dl.DeadLetter(msg, mainQueue, REASON_REJECTED)
	require.NoError(t, err)

	// Verify message in DLQ retains persistence
	dlq := vh.Queues["dlq"]
	deadMsg := dlq.Pop()
	require.NotNil(t, deadMsg)
	assert.Equal(t, amqp.DeliveryMode(2), deadMsg.Properties.DeliveryMode, "Persistent message should remain persistent")
}

func TestAppendCCBCCToRoutingKey_EmptyHeaders(t *testing.T) {
	result := appendCCBCCToRoutingKey(nil, []string{"original"})
	assert.Equal(t, []string{"original"}, result)

	result = appendCCBCCToRoutingKey(make(map[string]any), []string{"original"})
	assert.Equal(t, []string{"original"}, result)
}

func TestAppendCCBCCToRoutingKey_InvalidTypes(t *testing.T) {
	headers := map[string]any{
		"CC":  "not-an-array", // Wrong type
		"BCC": []int{1, 2, 3}, // Wrong element type
	}

	result := appendCCBCCToRoutingKey(headers, []string{"original"})
	assert.Equal(t, []string{"original"}, result, "Should ignore invalid types")
}

func TestAppendCCBCCToRoutingKey_EmptyArrays(t *testing.T) {
	headers := map[string]any{
		"CC":  []string{},
		"BCC": []string{},
	}

	result := appendCCBCCToRoutingKey(headers, []string{"original"})
	assert.Equal(t, []string{"original"}, result, "Should ignore empty arrays")
}

func TestReasonType_String(t *testing.T) {
	testCases := []struct {
		reason   ReasonType
		expected string
	}{
		{REASON_REJECTED, "rejected"},
		{REASON_EXPIRED, "expired"},
		{REASON_MAX_LENGTH, "maxlen"},
		{REASON_DELIVERY_LIMIT, "delivery_limit"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.reason.String())
		})
	}
}
