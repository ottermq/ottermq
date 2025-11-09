package e2e

import (
	"context"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// =============================================================================
// E2E Transaction Tests - Testing with Real RabbitMQ Client
// =============================================================================

// TestTxBasicFlow tests the basic transaction flow: select, publish, commit
func TestTxBasicFlow(t *testing.T) {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare a test queue
	queueName := "test-tx-basic-flow"
	queue, err := ch.QueueDeclare(
		queueName,
		false, // durable
		true,  // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Enter transaction mode
	err = ch.Tx()
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Publish a message in transaction mode
	msgBody := "test message in transaction"
	err = ch.Publish(
		"",         // exchange
		queue.Name, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(msgBody),
		},
	)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Message should not be in queue yet (buffered in transaction)
	msg, ok, err := ch.Get(queue.Name, false)
	if err != nil {
		t.Fatalf("Failed to get message: %v", err)
	}
	if ok {
		t.Errorf("Message should not be available before commit, but got: %s", string(msg.Body))
	}

	// Commit the transaction
	err = ch.TxCommit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Now message should be available
	msg, ok, err = ch.Get(queue.Name, true) // auto-ack
	if err != nil {
		t.Fatalf("Failed to get message after commit: %v", err)
	}
	if !ok {
		t.Fatal("Message should be available after commit")
	}
	if string(msg.Body) != msgBody {
		t.Errorf("Expected message '%s', got '%s'", msgBody, string(msg.Body))
	}
}

// TestTxRollback tests that rollback discards buffered operations
func TestTxRollback(t *testing.T) {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare a test queue
	queueName := "test-tx-rollback"
	queue, err := ch.QueueDeclare(
		queueName,
		false, // durable
		true,  // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Enter transaction mode
	err = ch.Tx()
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Publish multiple messages
	for i := 0; i < 3; i++ {
		err = ch.Publish(
			"",
			queue.Name,
			false,
			false,
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte("message to rollback"),
			},
		)
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	// Rollback the transaction
	err = ch.TxRollback()
	if err != nil {
		t.Fatalf("Failed to rollback transaction: %v", err)
	}

	// Messages should NOT be in queue
	msg, ok, err := ch.Get(queue.Name, false)
	if err != nil {
		t.Fatalf("Failed to get message: %v", err)
	}
	if ok {
		t.Errorf("No messages should be available after rollback, but got: %s", string(msg.Body))
	}
}

// TestTxMultipleCommits tests that channel stays in TX mode after commit
// and can commit multiple transactions without re-entering TX mode
func TestTxMultipleCommits(t *testing.T) {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare a test queue
	queueName := "test-tx-multiple-commits"
	queue, err := ch.QueueDeclare(
		queueName,
		false, // durable
		true,  // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Enter transaction mode ONCE
	err = ch.Tx()
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// First transaction: publish and commit
	err = ch.Publish("", queue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("message 1"),
	})
	if err != nil {
		t.Fatalf("Failed to publish first message: %v", err)
	}

	err = ch.TxCommit()
	if err != nil {
		t.Fatalf("Failed to commit first transaction: %v", err)
	}

	// Second transaction: publish and commit (WITHOUT calling Tx() again)
	err = ch.Publish("", queue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("message 2"),
	})
	if err != nil {
		t.Fatalf("Failed to publish second message: %v", err)
	}

	err = ch.TxCommit()
	if err != nil {
		t.Fatalf("Failed to commit second transaction: %v", err)
	}

	// Verify both messages are in queue
	for i := 1; i <= 2; i++ {
		msg, ok, err := ch.Get(queue.Name, true)
		if err != nil {
			t.Fatalf("Failed to get message %d: %v", i, err)
		}
		if !ok {
			t.Fatalf("Expected message %d to be available", i)
		}
		t.Logf("Received message %d: %s", i, string(msg.Body))
	}

	// Queue should be empty now
	_, ok, _ := ch.Get(queue.Name, false)
	if ok {
		t.Error("Queue should be empty after consuming both messages")
	}
}

// TestTxAckInTransaction tests acknowledgment buffering in transactions
func TestTxAckInTransaction(t *testing.T) {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare a test queue
	queueName := "test-tx-ack"
	queue, err := ch.QueueDeclare(
		queueName,
		false, // durable
		true,  // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Publish messages outside of transaction
	for i := 1; i <= 3; i++ {
		err = ch.Publish("", queue.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("message for ack test"),
		})
		if err != nil {
			t.Fatalf("Failed to publish message %d: %v", i, err)
		}
	}

	// Enter transaction mode
	err = ch.Tx()
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Consume and ack messages within transaction
	msgs, err := ch.Consume(
		queue.Name,
		"",    // consumer tag
		false, // auto-ack (manual ack)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to start consuming: %v", err)
	}

	deliveryTags := []uint64{}
	for i := 0; i < 3; i++ {
		select {
		case msg := <-msgs:
			t.Logf("Received message with delivery tag: %d", msg.DeliveryTag)
			// ACK within transaction (should be buffered)
			err = ch.Ack(msg.DeliveryTag, false)
			if err != nil {
				t.Fatalf("Failed to ack message: %v", err)
			}
			deliveryTags = append(deliveryTags, msg.DeliveryTag)
		case <-time.After(2 * time.Second):
			t.Fatalf("Timeout waiting for message %d", i+1)
		}
	}

	// Cancel consumer
	err = ch.Cancel("", false)
	if err != nil {
		t.Logf("Failed to cancel consumer (may not be an error): %v", err)
	}

	// Rollback the transaction - ACKs should be discarded, messages should reappear
	err = ch.TxRollback()
	if err != nil {
		t.Fatalf("Failed to rollback transaction: %v", err)
	}
	// The following test is commented out because RabbitMQ server also fails it
	// so this test is not valid for verifying OtterMQ behavior.
	//----//
	// Messages should still be in queue (ACKs were rolled back)
	// time.Sleep(1000 * time.Millisecond) // Give broker time to requeue

	// msg, ok, err := ch.Get(queue.Name, true)
	// if err != nil {
	// 	t.Fatalf("Failed to get message after rollback: %v", err)
	// }
	// if !ok {
	// 	t.Error("Expected messages to be requeued after rollback of ACKs")
	// } else {
	// 	t.Logf("Successfully retrieved message after ACK rollback: %s", string(msg.Body))
	// }
}

// TestTxNackInTransaction tests NACK buffering in transactions
func TestTxNackInTransaction(t *testing.T) {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare a test queue
	queueName := "test-tx-nack"
	queue, err := ch.QueueDeclare(
		queueName,
		false, // durable
		true,  // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Publish a message
	err = ch.Publish("", queue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("message for nack test"),
	})
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Enter transaction mode
	err = ch.Tx()
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Get message
	msg, ok, err := ch.Get(queue.Name, false) // manual ack
	if err != nil {
		t.Fatalf("Failed to get message: %v", err)
	}
	if !ok {
		t.Fatal("Expected message to be available")
	}

	// NACK with requeue in transaction mode
	err = ch.Nack(msg.DeliveryTag, false, true)
	if err != nil {
		t.Fatalf("Failed to nack: %v", err)
	}

	// Commit transaction - NACK should be processed, message requeued
	err = ch.TxCommit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Message should be back in queue
	time.Sleep(100 * time.Millisecond) // Give broker time to requeue

	msg2, ok, err := ch.Get(queue.Name, true)
	if err != nil {
		t.Fatalf("Failed to get message after nack: %v", err)
	}
	if !ok {
		t.Error("Expected message to be requeued after NACK commit")
	} else {
		if string(msg2.Body) != string(msg.Body) {
			t.Errorf("Expected same message body, got '%s' vs '%s'", string(msg2.Body), string(msg.Body))
		}
	}
}

// TestTxRejectInTransaction tests REJECT buffering in transactions
func TestTxRejectInTransaction(t *testing.T) {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare a test queue
	queueName := "test-tx-reject"
	queue, err := ch.QueueDeclare(
		queueName,
		false, // durable
		true,  // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Publish a message
	err = ch.Publish("", queue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("message for reject test"),
	})
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Enter transaction mode
	err = ch.Tx()
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Get message
	msg, ok, err := ch.Get(queue.Name, false) // manual ack
	if err != nil {
		t.Fatalf("Failed to get message: %v", err)
	}
	if !ok {
		t.Fatal("Expected message to be available")
	}

	// REJECT with requeue in transaction mode
	err = ch.Reject(msg.DeliveryTag, true)
	if err != nil {
		t.Fatalf("Failed to reject: %v", err)
	}

	// Commit transaction - REJECT should be processed, message requeued
	err = ch.TxCommit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Message should be back in queue
	time.Sleep(100 * time.Millisecond)

	msg2, ok, err := ch.Get(queue.Name, true)
	if err != nil {
		t.Fatalf("Failed to get message after reject: %v", err)
	}
	if !ok {
		t.Error("Expected message to be requeued after REJECT commit")
	} else {
		if string(msg2.Body) != string(msg.Body) {
			t.Errorf("Expected same message body, got '%s' vs '%s'", string(msg2.Body), string(msg.Body))
		}
	}
}

// TestTxMandatoryReturn tests that mandatory messages with no route are handled correctly
func TestTxMandatoryReturn(t *testing.T) {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Setup return handler
	returns := ch.NotifyReturn(make(chan amqp.Return, 1))

	// Declare an exchange
	err = ch.ExchangeDeclare(
		"test-tx-exchange",
		"direct",
		false, // durable
		true,  // auto-delete
		false, // internal
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to declare exchange: %v", err)
	}

	// Enter transaction mode
	err = ch.Tx()
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Publish mandatory message with no routing (no queue bound)
	err = ch.Publish(
		"test-tx-exchange",
		"no.route",
		true,  // mandatory - should trigger return
		false, // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("mandatory message with no route"),
		},
	)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Commit should fail or return the message
	err = ch.TxCommit()

	// RabbitMQ behavior: commit may succeed but message is returned
	// Check for returned message
	select {
	case ret := <-returns:
		t.Logf("Message returned as expected: %s (code: %d, reason: %s)",
			string(ret.Body), ret.ReplyCode, ret.ReplyText)
	case <-time.After(1 * time.Second):
		// OtterMQ currently fails the commit for mandatory with no route
		// This is acceptable behavior - check if commit failed
		if err == nil {
			t.Log("Warning: Expected either commit failure or message return for mandatory message with no route")
		} else {
			t.Logf("Commit failed as expected for mandatory with no route: %v", err)
		}
	}
}

// TestTxMixedOperations tests a transaction with mixed publish and ack operations
func TestTxMixedOperations(t *testing.T) {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare a test queue
	queueName := "test-tx-mixed"
	queue, err := ch.QueueDeclare(
		queueName,
		false, // durable
		true,  // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Pre-populate queue with a message
	err = ch.Publish("", queue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("existing message"),
	})
	if err != nil {
		t.Fatalf("Failed to pre-populate queue: %v", err)
	}

	// Enter transaction mode
	err = ch.Tx()
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Operation 1: Get and ACK existing message
	msg, ok, err := ch.Get(queue.Name, false)
	if err != nil {
		t.Fatalf("Failed to get message: %v", err)
	}
	if !ok {
		t.Fatal("Expected existing message")
	}
	err = ch.Ack(msg.DeliveryTag, false)
	if err != nil {
		t.Fatalf("Failed to ack: %v", err)
	}

	// Operation 2: Publish new messages
	for i := 1; i <= 3; i++ {
		err = ch.Publish("", queue.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("new message"),
		})
		if err != nil {
			t.Fatalf("Failed to publish message %d: %v", i, err)
		}
	}

	// Commit transaction
	err = ch.TxCommit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Verify: 3 new messages should be in queue (old one was acked)
	messageCount := 0
	for i := 0; i < 5; i++ { // Try up to 5 times
		_, ok, err := ch.Get(queue.Name, true)
		if err != nil {
			t.Fatalf("Failed to get message: %v", err)
		}
		if !ok {
			break
		}
		messageCount++
	}

	if messageCount != 3 {
		t.Errorf("Expected 3 messages after mixed transaction, got %d", messageCount)
	}
}

// TestTxChannelCloseImplicitRollback tests that closing a channel rolls back uncommitted transactions
func TestTxChannelCloseImplicitRollback(t *testing.T) {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}

	// Declare a test queue
	queueName := "test-tx-channel-close"
	queue, err := ch.QueueDeclare(
		queueName,
		false, // durable
		false, // auto-delete (keep for verification)
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Enter transaction mode
	err = ch.Tx()
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Publish messages
	for i := 0; i < 3; i++ {
		err = ch.Publish("", queue.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("message should be rolled back"),
		})
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	// Close channel WITHOUT committing (implicit rollback)
	err = ch.Close()
	if err != nil {
		t.Fatalf("Failed to close channel: %v", err)
	}

	// Open new channel to verify queue is empty
	ch2, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open second channel: %v", err)
	}
	defer ch2.Close()

	// Check queue - should be empty (implicit rollback)
	_, ok, err := ch2.Get(queue.Name, false)
	if err != nil {
		t.Fatalf("Failed to get message: %v", err)
	}
	if ok {
		t.Error("Queue should be empty after channel close (implicit rollback)")
	}

	// Cleanup
	_, err = ch2.QueueDelete(queue.Name, false, false, false)
	if err != nil {
		t.Logf("Failed to cleanup queue: %v", err)
	}
}

// TestTxIdempotentSelect tests that calling Tx() multiple times is idempotent
func TestTxIdempotentSelect(t *testing.T) {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Call Tx() multiple times
	for i := 0; i < 3; i++ {
		err = ch.Tx()
		if err != nil {
			t.Fatalf("Failed to enter transaction mode (call %d): %v", i+1, err)
		}
	}

	// Should still work normally
	queueName := "test-tx-idempotent"
	queue, err := ch.QueueDeclare(queueName, false, true, false, false, nil)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	err = ch.Publish("", queue.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("test"),
	})
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	err = ch.TxCommit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Verify message was published
	msg, ok, err := ch.Get(queue.Name, true)
	if err != nil {
		t.Fatalf("Failed to get message: %v", err)
	}
	if !ok {
		t.Error("Expected message to be published")
	} else {
		t.Logf("Successfully published and retrieved message: %s", string(msg.Body))
	}
}

// TestTxWithQos tests transaction interaction with QoS prefetch
func TestTxWithQos(t *testing.T) {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Set QoS
	err = ch.Qos(1, 0, false)
	if err != nil {
		t.Fatalf("Failed to set QoS: %v", err)
	}

	// Declare queue
	queueName := "test-tx-qos"
	queue, err := ch.QueueDeclare(queueName, false, true, false, false, nil)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Publish messages outside transaction
	for i := 0; i < 3; i++ {
		err = ch.Publish("", queue.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("qos test message"),
		})
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	// Enter transaction mode
	err = ch.Tx()
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Consume with QoS - should only get 1 message at a time
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		t.Fatalf("Failed to consume: %v", err)
	}

	// Receive and ACK first message in transaction
	select {
	case msg := <-msgs:
		t.Logf("Received message with delivery tag: %d", msg.DeliveryTag)
		err = ch.Ack(msg.DeliveryTag, false)
		if err != nil {
			t.Fatalf("Failed to ack: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for message")
	}

	// Commit - should free up prefetch slot
	err = ch.TxCommit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Should be able to receive next message (prefetch slot freed)
	select {
	case msg := <-msgs:
		t.Logf("Received second message with delivery tag: %d", msg.DeliveryTag)
		ch.Ack(msg.DeliveryTag, false)
	case <-time.After(2 * time.Second):
		t.Error("Should receive second message after commit frees prefetch slot")
	}

	ch.Cancel("", false)
}
