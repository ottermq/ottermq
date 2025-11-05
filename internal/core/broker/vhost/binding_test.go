package vhost

import (
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/amqp/errors"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
)

// Test UnbindQueue with valid binding
func TestUnbindQueue_Success(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	// Create exchange and queue
	exchangeName := "test-exchange"
	queueName := "test-queue"
	routingKey := "test.key"

	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})
	vh.CreateQueue(queueName, &QueueProperties{Durable: false})

	// Bind queue to exchange
	err := vh.BindQueue(exchangeName, queueName, routingKey, nil)
	if err != nil {
		t.Fatalf("BindQueue failed: %v", err)
	}

	// Verify binding exists
	exchange := vh.Exchanges[exchangeName]
	if len(exchange.Bindings[routingKey]) != 1 {
		t.Fatal("Expected 1 binding")
	}

	// Unbind queue
	err = vh.UnbindQueue(exchangeName, queueName, routingKey, nil)
	if err != nil {
		t.Fatalf("UnbindQueue failed: %v", err)
	}

	// Verify binding removed
	if len(exchange.Bindings[routingKey]) != 0 {
		t.Error("Expected binding to be removed")
	}

	// Verify queue still exists
	if _, exists := vh.Queues[queueName]; !exists {
		t.Error("Queue should not be deleted during unbind")
	}
}

// Test UnbindQueue with non-existent exchange
func TestUnbindQueue_ExchangeNotFound(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	queueName := "test-queue"
	vh.CreateQueue(queueName, &QueueProperties{Durable: false})

	err := vh.UnbindQueue("ghost-exchange", queueName, "test.key", nil)

	if err == nil {
		t.Fatal("Expected error for non-existent exchange")
	}

	amqpErr, ok := err.(errors.AMQPError)
	if !ok {
		t.Fatalf("Expected AMQPError, got %T", err)
	}

	if amqpErr.ReplyCode() != uint16(amqp.NOT_FOUND) {
		t.Errorf("Expected reply code %d (NOT_FOUND), got %d", amqp.NOT_FOUND, amqpErr.ReplyCode())
	}

	if amqpErr.MethodID() != uint16(amqp.QUEUE_UNBIND) {
		t.Errorf("Expected method ID %d (QUEUE_UNBIND), got %d", amqp.QUEUE_UNBIND, amqpErr.MethodID())
	}
}

// Test UnbindQueue with non-existent queue
func TestUnbindQueue_QueueNotFound(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-exchange"
	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})

	err := vh.UnbindQueue(exchangeName, "ghost-queue", "test.key", nil)

	if err == nil {
		t.Fatal("Expected error for non-existent queue")
	}

	amqpErr, ok := err.(errors.AMQPError)
	if !ok {
		t.Fatalf("Expected AMQPError, got %T", err)
	}

	if amqpErr.ReplyCode() != uint16(amqp.NOT_FOUND) {
		t.Errorf("Expected reply code %d (NOT_FOUND), got %d", amqp.NOT_FOUND, amqpErr.ReplyCode())
	}

	if amqpErr.MethodID() != uint16(amqp.QUEUE_UNBIND) {
		t.Errorf("Expected method ID %d (QUEUE_UNBIND), got %d", amqp.QUEUE_UNBIND, amqpErr.MethodID())
	}
}

// Test UnbindQueue with non-existent binding
func TestUnbindQueue_BindingNotFound(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-exchange"
	queueName := "test-queue"

	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})
	vh.CreateQueue(queueName, &QueueProperties{Durable: false})

	// Try to unbind without creating a binding first
	err := vh.UnbindQueue(exchangeName, queueName, "nonexistent.key", nil)

	if err == nil {
		t.Fatal("Expected error for non-existent binding")
	}

	amqpErr, ok := err.(errors.AMQPError)
	if !ok {
		t.Fatalf("Expected AMQPError, got %T", err)
	}

	if amqpErr.ReplyCode() != uint16(amqp.NOT_FOUND) {
		t.Errorf("Expected reply code %d (NOT_FOUND), got %d", amqp.NOT_FOUND, amqpErr.ReplyCode())
	}

	if amqpErr.MethodID() != uint16(amqp.QUEUE_UNBIND) {
		t.Errorf("Expected method ID %d (QUEUE_UNBIND), got %d", amqp.QUEUE_UNBIND, amqpErr.MethodID())
	}
}

// Test UnbindQueue with multiple bindings on same routing key
func TestUnbindQueue_MultipleBindings(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-exchange"
	routingKey := "shared.key"

	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})
	vh.CreateQueue("queue1", &QueueProperties{Durable: false})
	vh.CreateQueue("queue2", &QueueProperties{Durable: false})
	vh.CreateQueue("queue3", &QueueProperties{Durable: false})

	// Bind all queues to same routing key
	vh.BindQueue(exchangeName, "queue1", routingKey, nil)
	vh.BindQueue(exchangeName, "queue2", routingKey, nil)
	vh.BindQueue(exchangeName, "queue3", routingKey, nil)

	exchange := vh.Exchanges[exchangeName]
	if len(exchange.Bindings[routingKey]) != 3 {
		t.Fatalf("Expected 3 bindings, got %d", len(exchange.Bindings[routingKey]))
	}

	// Unbind queue2
	err := vh.UnbindQueue(exchangeName, "queue2", routingKey, nil)
	if err != nil {
		t.Fatalf("UnbindQueue failed: %v", err)
	}

	// Verify only queue2 was removed
	if len(exchange.Bindings[routingKey]) != 2 {
		t.Errorf("Expected 2 bindings after unbind, got %d", len(exchange.Bindings[routingKey]))
	}

	// Verify queue1 and queue3 are still bound
	bindings := exchange.Bindings[routingKey]
	hasQueue1 := false
	hasQueue3 := false
	hasQueue2 := false

	for _, b := range bindings {
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
		t.Error("queue2 should not be bound anymore")
	}
}

// Test UnbindQueue removes routing key when last queue is unbound
func TestUnbindQueue_RemovesRoutingKeyWhenEmpty(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-exchange"
	queueName := "test-queue"
	routingKey := "test.key"

	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})
	vh.CreateQueue(queueName, &QueueProperties{Durable: false})
	vh.BindQueue(exchangeName, queueName, routingKey, nil)

	exchange := vh.Exchanges[exchangeName]
	if _, exists := exchange.Bindings[routingKey]; !exists {
		t.Fatal("Expected routing key to exist")
	}

	// Unbind the only queue
	err := vh.UnbindQueue(exchangeName, queueName, routingKey, nil)
	if err != nil {
		t.Fatalf("UnbindQueue failed: %v", err)
	}

	// Verify routing key is removed from bindings map
	if _, exists := exchange.Bindings[routingKey]; exists {
		t.Error("Expected routing key to be removed when no queues are bound")
	}
}

// Test UnbindQueue with fanout exchange
func TestUnbindQueue_FanoutExchange(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-fanout"
	queueName := "test-queue"

	vh.CreateExchange(exchangeName, FANOUT, &ExchangeProperties{Durable: false})
	vh.CreateQueue(queueName, &QueueProperties{Durable: false})

	// Bind queue to fanout exchange
	err := vh.BindQueue(exchangeName, queueName, "", nil)
	if err != nil {
		t.Fatalf("BindQueue failed: %v", err)
	}

	exchange := vh.Exchanges[exchangeName]
	// if _, exists := exchange.Bindings[queueName]; !exists {
	// 	t.Fatal("Expected queue to be in fanout exchange")
	// }
	// Verify binding exists
	if _, exists := exchange.Bindings[""]; !exists {
		t.Fatal("Expected routing key '' to exist for fanout exchange")
	}
	found := false
	for _, b := range exchange.Bindings[""] {
		if b.Queue.Name == queueName {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Expected queue to be in fanout exchange bindings")
	}

	// Unbind queue
	err = vh.UnbindQueue(exchangeName, queueName, "", nil)
	if err != nil {
		t.Fatalf("UnbindQueue failed: %v", err)
	}

	// Verify queue removed from fanout exchange
	// if _, exists := exchange.Queues[queueName]; exists {
	// 	t.Error("Expected queue to be removed from fanout exchange")
	// }
	if _, exists := exchange.Bindings[""]; exists {
		for _, b := range exchange.Bindings[""] {
			if b.Queue.Name == queueName {
				t.Error("Expected queue to be removed from fanout exchange")
			}
		}
	}
}

// Test UnbindQueue triggers auto-delete of exchange
func TestUnbindQueue_AutoDeleteExchange(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-exchange"
	queueName := "test-queue"
	routingKey := "test.key"

	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{
		Durable:    false,
		AutoDelete: true,
	})
	vh.CreateQueue(queueName, &QueueProperties{Durable: false})
	vh.BindQueue(exchangeName, queueName, routingKey, nil)

	// Verify exchange exists
	if _, exists := vh.Exchanges[exchangeName]; !exists {
		t.Fatal("Expected exchange to exist")
	}

	// Unbind the only queue (should trigger auto-delete)
	err := vh.UnbindQueue(exchangeName, queueName, routingKey, nil)
	if err != nil {
		t.Fatalf("UnbindQueue failed: %v", err)
	}

	// Verify exchange was auto-deleted
	if _, exists := vh.Exchanges[exchangeName]; exists {
		t.Error("Expected exchange to be auto-deleted")
	}
}

// Test UnbindQueue doesn't auto-delete exchange when other bindings exist
func TestUnbindQueue_NoAutoDeleteWhenOtherBindingsExist(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-exchange"
	routingKey := "test.key"

	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{
		Durable:    false,
		AutoDelete: true,
	})
	vh.CreateQueue("queue1", &QueueProperties{Durable: false})
	vh.CreateQueue("queue2", &QueueProperties{Durable: false})

	vh.BindQueue(exchangeName, "queue1", routingKey, nil)
	vh.BindQueue(exchangeName, "queue2", routingKey, nil)

	// Unbind one queue
	err := vh.UnbindQueue(exchangeName, "queue1", routingKey, nil)
	if err != nil {
		t.Fatalf("UnbindQueue failed: %v", err)
	}

	// Exchange should still exist because queue2 is still bound
	if _, exists := vh.Exchanges[exchangeName]; !exists {
		t.Error("Expected exchange to still exist with remaining binding")
	}
}

// Test DeleteBindingUnlocked with queue not in binding list
func TestDeleteBindingUnlocked_QueueNotInBindingList(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchange := &Exchange{
		Name:     "test-exchange",
		Typ:      DIRECT,
		Bindings: make(map[string][]*Binding),
		Props:    &ExchangeProperties{},
	}

	queue1 := &Queue{Name: "queue1"}
	_ = &Queue{Name: "queue2"}

	// Add queue1 to binding
	// exchange.Bindings["test.key"] = []*Queue{queue1}
	exchange.Bindings["test.key"] = []*Binding{
		{
			Queue:      queue1,
			RoutingKey: "test.key",
			Args:       nil,
		},
	}

	// Try to remove queue2 which is not in the binding list
	err := vh.DeleteBindingUnlocked(exchange, "queue2", "test.key", nil)

	if err == nil {
		t.Fatal("Expected error when queue not in binding list")
	}

	amqpErr, ok := err.(errors.AMQPError)
	if !ok {
		t.Fatalf("Expected AMQPError, got %T", err)
	}

	if amqpErr.ReplyCode() != uint16(amqp.NOT_FOUND) {
		t.Errorf("Expected reply code %d (NOT_FOUND), got %d", amqp.NOT_FOUND, amqpErr.ReplyCode())
	}

	if amqpErr.MethodID() != uint16(amqp.QUEUE_UNBIND) {
		t.Errorf("Expected method ID %d (QUEUE_UNBIND), got %d", amqp.QUEUE_UNBIND, amqpErr.MethodID())
	}
}

// Test idempotency - trying to unbind already unbound queue
func TestUnbindQueue_AlreadyUnbound(t *testing.T) {
	vh := NewVhost("/", 100, &dummy.DummyPersistence{})

	exchangeName := "test-exchange"
	queueName := "test-queue"
	routingKey := "test.key"

	vh.CreateExchange(exchangeName, DIRECT, &ExchangeProperties{Durable: false})
	vh.CreateQueue(queueName, &QueueProperties{Durable: false})

	// Bind and then unbind
	vh.BindQueue(exchangeName, queueName, routingKey, nil)
	err := vh.UnbindQueue(exchangeName, queueName, routingKey, nil)
	if err != nil {
		t.Fatalf("First unbind failed: %v", err)
	}

	// Try to unbind again
	err = vh.UnbindQueue(exchangeName, queueName, routingKey, nil)
	if err == nil {
		t.Fatal("Expected error when unbinding already unbound queue")
	}

	// Should return NOT_FOUND error
	amqpErr, ok := err.(errors.AMQPError)
	if !ok {
		t.Fatalf("Expected AMQPError, got %T", err)
	}

	if amqpErr.ReplyCode() != uint16(amqp.NOT_FOUND) {
		t.Errorf("Expected reply code %d (NOT_FOUND), got %d", amqp.NOT_FOUND, amqpErr.ReplyCode())
	}
}
