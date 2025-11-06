package vhost

import (
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/amqp/errors"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
)

// ========== FANOUT EXCHANGE TESTS ==========

// Test fanout publish distributes to all bound queues
func TestFanoutPublish_DistributesToAllQueues(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-fanout"
	vh.CreateExchange(exchangeName, FANOUT, &ExchangeProperties{Durable: false})

	// Create and bind multiple queues
	queues := []string{"queue1", "queue2", "queue3"}
	for _, qName := range queues {
		vh.CreateQueue(qName, &QueueProperties{Durable: false}, nil)
		err := vh.BindQueue(exchangeName, qName, "", nil, nil)
		if err != nil {
			t.Fatalf("Failed to bind %s: %v", qName, err)
		}
	}

	// Publish a message
	msg := &amqp.Message{
		ID:   "msg-1",
		Body: []byte("test message"),
		Properties: amqp.BasicProperties{
			ContentType: "text/plain",
		},
	}

	_, err := vh.Publish(exchangeName, "any.key", msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Verify all queues received the message
	for _, qName := range queues {
		count, _ := vh.GetMessageCount(qName)
		if count != 1 {
			t.Errorf("Queue %s expected 1 message, got %d", qName, count)
		}

		receivedMsg := vh.GetMessage(qName)
		if receivedMsg == nil {
			t.Errorf("Queue %s should have a message", qName)
		} else if string(receivedMsg.Body) != "test message" {
			t.Errorf("Queue %s message body mismatch", qName)
		}
	}
}

// Test fanout with empty bindings
func TestFanoutPublish_NoBindings(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-fanout-empty"
	vh.CreateExchange(exchangeName, FANOUT, &ExchangeProperties{Durable: false})

	msg := &amqp.Message{
		ID:   "msg-1",
		Body: []byte("test message"),
		Properties: amqp.BasicProperties{
			ContentType: "text/plain",
		},
	}

	// Publishing to fanout with no bindings should fail
	_, err := vh.Publish(exchangeName, "", msg)
	if err == nil {
		t.Fatal("Expected error when publishing to fanout exchange with no bindings")
	}
}

// Test fanout ignores routing key during publish
func TestFanoutPublish_IgnoresRoutingKey(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-fanout"
	vh.CreateExchange(exchangeName, FANOUT, &ExchangeProperties{Durable: false})
	vh.CreateQueue("queue1", &QueueProperties{Durable: false}, nil)
	vh.BindQueue(exchangeName, "queue1", "", nil, nil)

	msg := &amqp.Message{
		ID:   "msg-1",
		Body: []byte("test"),
		Properties: amqp.BasicProperties{
			ContentType: "text/plain",
		},
	}

	// Try with different routing keys - all should work
	routingKeys := []string{"", "foo", "bar.baz", "anything"}
	for _, key := range routingKeys {
		_, err := vh.Publish(exchangeName, key, msg)
		if err != nil {
			t.Errorf("Publish with routing key '%s' failed: %v", key, err)
		}
	}

	// Verify all messages arrived
	count, _ := vh.GetMessageCount("queue1")
	if count != len(routingKeys) {
		t.Errorf("Expected %d messages, got %d", len(routingKeys), count)
	}
}

// Test fanout unbind removes correct queue
func TestFanoutUnbind_RemovesSpecificQueue(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-fanout"
	vh.CreateExchange(exchangeName, FANOUT, &ExchangeProperties{Durable: false})

	// Bind multiple queues
	vh.CreateQueue("queue1", &QueueProperties{Durable: false}, nil)
	vh.CreateQueue("queue2", &QueueProperties{Durable: false}, nil)
	vh.CreateQueue("queue3", &QueueProperties{Durable: false}, nil)

	vh.BindQueue(exchangeName, "queue1", "", nil, nil)
	vh.BindQueue(exchangeName, "queue2", "", nil, nil)
	vh.BindQueue(exchangeName, "queue3", "", nil, nil)

	exchange := vh.Exchanges[exchangeName]
	if len(exchange.Bindings[""]) != 3 {
		t.Fatalf("Expected 3 bindings, got %d", len(exchange.Bindings[""]))
	}

	// Unbind queue2
	err := vh.UnbindQueue(exchangeName, "queue2", "", nil, nil)
	if err != nil {
		t.Fatalf("Unbind failed: %v", err)
	}

	// Verify only 2 bindings remain
	if len(exchange.Bindings[""]) != 2 {
		t.Errorf("Expected 2 bindings, got %d", len(exchange.Bindings[""]))
	}

	// Verify queue1 and queue3 still bound
	hasQueue1 := false
	hasQueue3 := false
	hasQueue2 := false

	for _, b := range exchange.Bindings[""] {
		if b.Queue.Name == "queue1" {
			hasQueue1 = true
		}
		if b.Queue.Name == "queue3" {
			hasQueue3 = true
		}
		if b.Queue.Name == "queue2" {
			hasQueue2 = true
		}
	}

	if !hasQueue1 || !hasQueue3 {
		t.Error("queue1 and queue3 should still be bound")
	}
	if hasQueue2 {
		t.Error("queue2 should not be bound")
	}
}

// Test fanout auto-delete after last unbind
func TestFanoutUnbind_AutoDeleteExchange(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-fanout-autodel"
	vh.CreateExchange(exchangeName, FANOUT, &ExchangeProperties{
		Durable:    false,
		AutoDelete: true,
	})

	vh.CreateQueue("queue1", &QueueProperties{Durable: false}, nil)
	vh.BindQueue(exchangeName, "queue1", "", nil, nil)

	// Verify exchange exists
	if _, exists := vh.Exchanges[exchangeName]; !exists {
		t.Fatal("Exchange should exist")
	}

	// Unbind the only queue
	err := vh.UnbindQueue(exchangeName, "queue1", "", nil, nil)
	if err != nil {
		t.Fatalf("Unbind failed: %v", err)
	}

	// Exchange should be auto-deleted
	if _, exists := vh.Exchanges[exchangeName]; exists {
		t.Error("Exchange should be auto-deleted")
	}
}

// Test fanout unbind with routing key is ignored
func TestFanoutUnbind_IgnoresRoutingKey(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-fanout"
	vh.CreateExchange(exchangeName, FANOUT, &ExchangeProperties{Durable: false})
	vh.CreateQueue("queue1", &QueueProperties{Durable: false}, nil)
	vh.BindQueue(exchangeName, "queue1", "", nil, nil)

	// Unbind with various routing keys - all should work because fanout ignores them
	err := vh.UnbindQueue(exchangeName, "queue1", "any.routing.key", nil, nil)
	if err != nil {
		t.Fatalf("Unbind should succeed even with routing key for fanout: %v", err)
	}

	// Verify binding removed
	exchange := vh.Exchanges[exchangeName]
	if len(exchange.Bindings[""]) != 0 {
		t.Error("Binding should be removed")
	}
}

// ========== BINDING ARGUMENTS TESTS ==========

// Test binding with same queue+exchange+key but different args creates multiple bindings
func TestBindQueue_DifferentArguments_CreatesMultipleBindings(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-exchange"
	queueName := "test-queue"
	routingKey := "test.key"

	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})
	vh.CreateQueue(queueName, &QueueProperties{Durable: false}, nil)

	// Bind with first set of args
	args1 := map[string]interface{}{"x-match": "all", "format": "pdf"}
	err := vh.BindQueue(exchangeName, queueName, routingKey, args1, nil)
	if err != nil {
		t.Fatalf("First bind failed: %v", err)
	}

	// Bind with different args - should succeed
	args2 := map[string]interface{}{"x-match": "any", "format": "json"}
	err = vh.BindQueue(exchangeName, queueName, routingKey, args2, nil)
	if err != nil {
		t.Fatalf("Second bind with different args failed: %v", err)
	}

	// Verify two bindings exist
	exchange := vh.Exchanges[exchangeName]
	bindings := exchange.Bindings[routingKey]
	if len(bindings) != 2 {
		t.Fatalf("Expected 2 bindings, got %d", len(bindings))
	}

	// Verify arguments are different
	if bindingArgumentsMatch(bindings[0].Args, bindings[1].Args) {
		t.Error("Binding arguments should be different")
	}
}

// Test binding with identical args fails with PRECONDITION_FAILED
func TestBindQueue_DuplicateBinding_FailsWithPreconditionFailed(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-exchange"
	queueName := "test-queue"
	routingKey := "test.key"

	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})
	vh.CreateQueue(queueName, &QueueProperties{Durable: false}, nil)

	args := map[string]interface{}{"x-match": "all", "format": "pdf"}

	// First bind
	err := vh.BindQueue(exchangeName, queueName, routingKey, args, nil)
	if err != nil {
		t.Fatalf("First bind failed: %v", err)
	}

	// Second bind with identical args should fail
	err = vh.BindQueue(exchangeName, queueName, routingKey, args, nil)
	if err == nil {
		t.Fatal("Expected error for duplicate binding")
	}

	amqpErr, ok := err.(errors.AMQPError)
	if !ok {
		t.Fatalf("Expected AMQPError, got %T", err)
	}

	if amqpErr.ReplyCode() != uint16(amqp.PRECONDITION_FAILED) {
		t.Errorf("Expected PRECONDITION_FAILED (406), got %d", amqpErr.ReplyCode())
	}
}

// Test unbind requires matching arguments
func TestUnbindQueue_RequiresMatchingArguments(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-exchange"
	queueName := "test-queue"
	routingKey := "test.key"

	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})
	vh.CreateQueue(queueName, &QueueProperties{Durable: false}, nil)

	// Bind with specific args
	args1 := map[string]interface{}{"format": "pdf"}
	err := vh.BindQueue(exchangeName, queueName, routingKey, args1, nil)
	if err != nil {
		t.Fatalf("Bind failed: %v", err)
	}

	// Try to unbind with different args - should fail
	args2 := map[string]interface{}{"format": "json"}
	err = vh.UnbindQueue(exchangeName, queueName, routingKey, args2, nil)
	if err == nil {
		t.Fatal("Expected error when unbinding with mismatched arguments")
	}

	amqpErr, ok := err.(errors.AMQPError)
	if !ok {
		t.Fatalf("Expected AMQPError, got %T", err)
	}

	if amqpErr.ReplyCode() != uint16(amqp.NOT_FOUND) {
		t.Errorf("Expected NOT_FOUND (404), got %d", amqpErr.ReplyCode())
	}

	// Unbind with correct args should succeed
	err = vh.UnbindQueue(exchangeName, queueName, routingKey, args1, nil)
	if err != nil {
		t.Fatalf("Unbind with matching args failed: %v", err)
	}
}

// Test unbind removes only binding with matching arguments
func TestUnbindQueue_RemovesOnlyMatchingArgumentBinding(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-exchange"
	queueName := "test-queue"
	routingKey := "test.key"

	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})
	vh.CreateQueue(queueName, &QueueProperties{Durable: false}, nil)

	// Create two bindings with different args
	args1 := map[string]interface{}{"priority": 1}
	args2 := map[string]interface{}{"priority": 2}

	vh.BindQueue(exchangeName, queueName, routingKey, args1, nil)
	vh.BindQueue(exchangeName, queueName, routingKey, args2, nil)

	exchange := vh.Exchanges[exchangeName]
	if len(exchange.Bindings[routingKey]) != 2 {
		t.Fatalf("Expected 2 bindings, got %d", len(exchange.Bindings[routingKey]))
	}

	// Unbind the first one
	err := vh.UnbindQueue(exchangeName, queueName, routingKey, args1, nil)
	if err != nil {
		t.Fatalf("Unbind failed: %v", err)
	}

	// Should have 1 binding left
	if len(exchange.Bindings[routingKey]) != 1 {
		t.Errorf("Expected 1 binding remaining, got %d", len(exchange.Bindings[routingKey]))
	}

	// Verify the remaining binding has args2
	remaining := exchange.Bindings[routingKey][0]
	if !bindingArgumentsMatch(remaining.Args, args2) {
		t.Error("Wrong binding was removed")
	}
}

// Test binding with nil arguments
func TestBindQueue_NilArguments(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-exchange"
	queueName := "test-queue"
	routingKey := "test.key"

	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})
	vh.CreateQueue(queueName, &QueueProperties{Durable: false}, nil)

	// Bind with nil args
	err := vh.BindQueue(exchangeName, queueName, routingKey, nil, nil)
	if err != nil {
		t.Fatalf("Bind with nil args failed: %v", err)
	}

	// Bind again with nil should fail (duplicate)
	err = vh.BindQueue(exchangeName, queueName, routingKey, nil, nil)
	if err == nil {
		t.Fatal("Expected error for duplicate binding with nil args")
	}

	// Bind with empty map should also fail (equivalent to nil for AMQP)
	emptyArgs := map[string]interface{}{}
	err = vh.BindQueue(exchangeName, queueName, routingKey, emptyArgs, nil)
	if err == nil {
		t.Fatal("Expected error for duplicate binding with empty args (equivalent to nil)")
	}

	// Should have 1 binding
	exchange := vh.Exchanges[exchangeName]
	if len(exchange.Bindings[routingKey]) != 1 {
		t.Errorf("Expected 1 binding, got %d", len(exchange.Bindings[routingKey]))
	}
}

// ========== DIRECT EXCHANGE TESTS ==========

// Test direct exchange routes only to matching key
func TestDirectPublish_OnlyMatchingRoutingKey(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-direct"
	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})

	// Bind queues with different routing keys
	vh.CreateQueue("queue1", &QueueProperties{Durable: false}, nil)
	vh.CreateQueue("queue2", &QueueProperties{Durable: false}, nil)
	vh.CreateQueue("queue3", &QueueProperties{Durable: false}, nil)

	vh.BindQueue(exchangeName, "queue1", "logs.error", nil, nil)
	vh.BindQueue(exchangeName, "queue2", "logs.info", nil, nil)
	vh.BindQueue(exchangeName, "queue3", "logs.error", nil, nil)

	// Publish to logs.error
	msg := &amqp.Message{
		ID:   "msg-1",
		Body: []byte("error message"),
		Properties: amqp.BasicProperties{
			ContentType: "text/plain",
		},
	}

	_, err := vh.Publish(exchangeName, "logs.error", msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Verify only queue1 and queue3 received message
	count1, _ := vh.GetMessageCount("queue1")
	count2, _ := vh.GetMessageCount("queue2")
	count3, _ := vh.GetMessageCount("queue3")

	if count1 != 1 {
		t.Errorf("queue1 expected 1 message, got %d", count1)
	}
	if count2 != 0 {
		t.Errorf("queue2 expected 0 messages, got %d", count2)
	}
	if count3 != 1 {
		t.Errorf("queue3 expected 1 message, got %d", count3)
	}
}

// Test direct exchange with non-existent routing key
func TestDirectPublish_NonExistentRoutingKey(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-direct"
	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})

	vh.CreateQueue("queue1", &QueueProperties{Durable: false}, nil)
	vh.BindQueue(exchangeName, "queue1", "existing.key", nil, nil)

	msg := &amqp.Message{
		ID:   "msg-1",
		Body: []byte("test"),
		Properties: amqp.BasicProperties{
			ContentType: "text/plain",
		},
	}

	// Publish to non-existent routing key
	_, err := vh.Publish(exchangeName, "nonexistent.key", msg)
	if err == nil {
		t.Fatal("Expected error when publishing to non-existent routing key")
	}
}

// ========== AUTO-DELETE EXCHANGE TESTS ==========

// Test auto-delete doesn't trigger when other bindings exist
func TestAutoDelete_OnlyWhenAllBindingsRemoved(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-autodel"
	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{
		Durable:    false,
		AutoDelete: true,
	})

	vh.CreateQueue("queue1", &QueueProperties{Durable: false}, nil)
	vh.CreateQueue("queue2", &QueueProperties{Durable: false}, nil)

	// Bind both queues to same routing key
	vh.BindQueue(exchangeName, "queue1", "test.key", nil, nil)
	vh.BindQueue(exchangeName, "queue2", "test.key", nil, nil)

	// Unbind queue1
	vh.UnbindQueue(exchangeName, "queue1", "test.key", nil, nil)

	// Exchange should still exist
	if _, exists := vh.Exchanges[exchangeName]; !exists {
		t.Error("Exchange should not be auto-deleted when other bindings exist")
	}

	// Unbind queue2 (last binding)
	vh.UnbindQueue(exchangeName, "queue2", "test.key", nil, nil)

	// Now exchange should be deleted
	if _, exists := vh.Exchanges[exchangeName]; exists {
		t.Error("Exchange should be auto-deleted after last binding removed")
	}
}

// Test auto-delete with multiple routing keys
func TestAutoDelete_MultipleRoutingKeys(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-autodel"
	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{
		Durable:    false,
		AutoDelete: true,
	})

	vh.CreateQueue("queue1", &QueueProperties{Durable: false}, nil)
	vh.CreateQueue("queue2", &QueueProperties{Durable: false}, nil)

	// Bind to different routing keys
	vh.BindQueue(exchangeName, "queue1", "key1", nil, nil)
	vh.BindQueue(exchangeName, "queue2", "key2", nil, nil)

	// Unbind key1
	vh.UnbindQueue(exchangeName, "queue1", "key1", nil, nil)

	// Exchange should still exist (key2 still bound)
	if _, exists := vh.Exchanges[exchangeName]; !exists {
		t.Error("Exchange should not be deleted while key2 binding exists")
	}

	// Unbind key2
	vh.UnbindQueue(exchangeName, "queue2", "key2", nil, nil)

	// Now exchange should be deleted
	if _, exists := vh.Exchanges[exchangeName]; exists {
		t.Error("Exchange should be auto-deleted after all routing keys unbound")
	}
}

// ========== HasRoutingForMessage TESTS ==========

// Test HasRoutingForMessage with fanout
func TestHasRoutingForMessage_Fanout(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-fanout"
	vh.CreateExchange(exchangeName, FANOUT, &ExchangeProperties{Durable: false})

	// No bindings - should return false
	hasRouting, err := vh.HasRoutingForMessage(exchangeName, "any.key")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if hasRouting {
		t.Error("Should return false for fanout with no bindings")
	}

	// Add a binding
	vh.CreateQueue("queue1", &QueueProperties{Durable: false}, nil)
	vh.BindQueue(exchangeName, "queue1", "", nil, nil)

	// Should return true regardless of routing key
	keys := []string{"", "foo", "bar.baz"}
	for _, key := range keys {
		hasRouting, err = vh.HasRoutingForMessage(exchangeName, key)
		if err != nil {
			t.Fatalf("Unexpected error for key '%s': %v", key, err)
		}
		if !hasRouting {
			t.Errorf("Should return true for fanout with bindings (key: '%s')", key)
		}
	}
}

// Test HasRoutingForMessage with direct
func TestHasRoutingForMessage_Direct(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-direct"
	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})

	vh.CreateQueue("queue1", &QueueProperties{Durable: false}, nil)
	vh.BindQueue(exchangeName, "queue1", "logs.error", nil, nil)

	// Should return true for bound key
	hasRouting, _ := vh.HasRoutingForMessage(exchangeName, "logs.error")
	if !hasRouting {
		t.Error("Should return true for bound routing key")
	}

	// Should return false for unbound key
	hasRouting, _ = vh.HasRoutingForMessage(exchangeName, "logs.info")
	if hasRouting {
		t.Error("Should return false for unbound routing key")
	}
}

// Note: TestHasRoutingForMessage_NonExistentExchange already exists in message_test.go

// ========== BINDING ARGUMENTS HELPER TESTS ==========

func TestBindingArgumentsMatch(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string]interface{}
		b        map[string]interface{}
		expected bool
	}{
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "both empty",
			a:        map[string]interface{}{},
			b:        map[string]interface{}{},
			expected: true,
		},
		{
			name:     "nil vs empty - equivalent for AMQP",
			a:        nil,
			b:        map[string]interface{}{},
			expected: true,
		},
		{
			name:     "same values",
			a:        map[string]interface{}{"key": "value"},
			b:        map[string]interface{}{"key": "value"},
			expected: true,
		},
		{
			name:     "different values",
			a:        map[string]interface{}{"key": "value1"},
			b:        map[string]interface{}{"key": "value2"},
			expected: false,
		},
		{
			name:     "different keys",
			a:        map[string]interface{}{"key1": "value"},
			b:        map[string]interface{}{"key2": "value"},
			expected: false,
		},
		{
			name:     "subset",
			a:        map[string]interface{}{"key1": "value1"},
			b:        map[string]interface{}{"key1": "value1", "key2": "value2"},
			expected: false,
		},
		{
			name:     "complex matching",
			a:        map[string]interface{}{"x-match": "all", "format": "pdf", "priority": 5},
			b:        map[string]interface{}{"x-match": "all", "format": "pdf", "priority": 5},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bindingArgumentsMatch(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("bindingArgumentsMatch(%v, %v) = %v, expected %v",
					tt.a, tt.b, result, tt.expected)
			}
		})
	}
}
