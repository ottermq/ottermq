package e2e

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	testQueue = "test-qos-queue"
)

// TestQoS_PerConsumer_PrefetchLimit verifies that prefetch count limits message delivery per consumer
func TestQoS_PerConsumer_PrefetchLimit(t *testing.T) {
	tc := NewTestConnection(t, brokerURL)
	defer tc.Close()

	// Declare queue with unique name
	queueName := tc.UniqueQueueName("qos-per-consumer")
	tc.DeclareQueue(queueName)

	// Set QoS with prefetch=3, global=false (per-consumer)
	tc.SetQoS(3, false)

	// Publish 10 messages
	tc.PublishMessages(queueName, 10)

	// Start consuming without auto-ack
	consumerTag := tc.UniqueConsumerTag("test-consumer")
	msgs := tc.StartConsumer(queueName, consumerTag, false)

	// Receive messages but don't ack them immediately
	var receivedCount int32
	var receivedMsgs []amqp.Delivery
	timeout := time.After(2 * time.Second)

	// Collect messages until we hit prefetch limit or timeout
	for {
		select {
		case msg := <-msgs:
			atomic.AddInt32(&receivedCount, 1)
			receivedMsgs = append(receivedMsgs, msg)
			t.Logf("Received message %d: %s", receivedCount, string(msg.Body))

			// After receiving 3 messages, wait a bit to ensure no more are delivered
			if receivedCount == 3 {
				time.Sleep(500 * time.Millisecond)
			}

		case <-timeout:
			goto done
		}
	}

done:
	// Should have received exactly 3 messages (prefetch limit)
	if receivedCount != 3 {
		t.Errorf("Expected 3 messages due to prefetch limit, got %d", receivedCount)
	}

	// Ack first message, should receive 4th message
	if len(receivedMsgs) > 0 {
		if err := receivedMsgs[0].Ack(false); err != nil {
			t.Errorf("Failed to ack message: %v", err)
		}

		// Wait for next message
		select {
		case msg := <-msgs:
			atomic.AddInt32(&receivedCount, 1)
			t.Logf("Received additional message after ack: %s", string(msg.Body))
		case <-time.After(1 * time.Second):
			t.Error("Expected to receive another message after ack, but didn't")
		}
	}

	// Final count should be 4
	if receivedCount != 4 {
		t.Errorf("Expected 4 messages total (3 initial + 1 after ack), got %d", receivedCount)
	}
}

// TestQoS_Global_ChannelLimit verifies that global=true limits all consumers on the channel
func TestQoS_Global_ChannelLimit(t *testing.T) {
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

	queueName := testQueue + "-global"

	// Declare queue
	_, err = ch.QueueDeclare(
		queueName,
		false, // durable
		true,  // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Set QoS with prefetch=5, global=true (channel-wide)
	err = ch.Qos(
		5,    // prefetchCount
		0,    // prefetchSize
		true, // global (channel-wide)
	)
	if err != nil {
		t.Fatalf("Failed to set QoS: %v", err)
	}

	// Publish 15 messages
	for i := 0; i < 15; i++ {
		err = ch.PublishWithContext(
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
			t.Fatalf("Failed to publish message %d: %v", i, err)
		}
	}

	// Start two consumers on the same channel
	msgs1, err := ch.Consume(
		queueName,
		"consumer-1",
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to start consumer 1: %v", err)
	}

	msgs2, err := ch.Consume(
		queueName,
		"consumer-2",
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to start consumer 2: %v", err)
	}

	// Collect messages from both consumers
	var totalReceived int32
	var mu sync.Mutex
	var allMsgs []amqp.Delivery
	timeout := time.After(2 * time.Second)

	go func() {
		for msg := range msgs1 {
			mu.Lock()
			atomic.AddInt32(&totalReceived, 1)
			allMsgs = append(allMsgs, msg)
			t.Logf("Consumer 1 received: %s", string(msg.Body))
			mu.Unlock()
		}
	}()

	go func() {
		for msg := range msgs2 {
			mu.Lock()
			atomic.AddInt32(&totalReceived, 1)
			allMsgs = append(allMsgs, msg)
			t.Logf("Consumer 2 received: %s", string(msg.Body))
			mu.Unlock()
		}
	}()

	// Wait for messages to arrive
	time.Sleep(1 * time.Second)

	// Total received across both consumers should be limited to 5 (global prefetch)
	received := atomic.LoadInt32(&totalReceived)
	if received != 5 {
		t.Errorf("Expected 5 messages total across both consumers (global limit), got %d", received)
	}

	// Ack 2 messages, should receive 2 more
	mu.Lock()
	if len(allMsgs) >= 2 {
		allMsgs[0].Ack(false)
		allMsgs[1].Ack(false)
	}
	mu.Unlock()

	// Wait for additional messages
	<-timeout

	finalReceived := atomic.LoadInt32(&totalReceived)
	if finalReceived < 6 { // Should have received at least 2 more after acks
		t.Errorf("Expected at least 6-7 messages after acking 2, got %d", finalReceived)
	}
}

// TestQoS_MultipleAck verifies that multiple=true in ack releases multiple slots
func TestQoS_MultipleAck(t *testing.T) {
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

	queueName := testQueue + "-multiple-ack"

	_, err = ch.QueueDeclare(queueName, false, true, false, false, nil)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Set prefetch to 2
	err = ch.Qos(2, 0, false)
	if err != nil {
		t.Fatalf("Failed to set QoS: %v", err)
	}

	// Publish 6 messages
	for i := 0; i < 6; i++ {
		err = ch.PublishWithContext(
			context.Background(),
			"", queueName, false, false,
			amqp.Publishing{Body: []byte(fmt.Sprintf("Msg %d", i))},
		)
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	msgs, err := ch.Consume(queueName, "test", false, false, false, false, nil)
	if err != nil {
		t.Fatalf("Failed to consume: %v", err)
	}

	var received []amqp.Delivery

	// Receive first 2 (prefetch limit)
	for i := 0; i < 2; i++ {
		select {
		case msg := <-msgs:
			received = append(received, msg)
			t.Logf("Initial receive %d: %s", i+1, string(msg.Body))
		case <-time.After(1 * time.Second):
			t.Fatalf("Timeout waiting for initial message %d", i+1)
		}
	}

	// Verify no more messages arrive (prefetch limit reached)
	select {
	case msg := <-msgs:
		t.Errorf("Unexpected message received while at prefetch limit: %s", string(msg.Body))
	case <-time.After(500 * time.Millisecond):
		t.Log("Correctly blocked at prefetch limit")
	}

	// Ack both with multiple=true (should release 2 slots)
	err = received[1].Ack(true) // Acks both deliveryTag 1 and 2
	if err != nil {
		t.Fatalf("Failed to ack multiple: %v", err)
	}

	// Should now receive 2 more messages
	for i := 0; i < 2; i++ {
		select {
		case msg := <-msgs:
			t.Logf("Received after multiple ack %d: %s", i+1, string(msg.Body))
		case <-time.After(1 * time.Second):
			t.Errorf("Expected message %d after multiple ack, but didn't receive it", i+1)
		}
	}
}

// TestQoS_Nack_WithRequeue verifies that nack with requeue respects QoS limits
func TestQoS_Nack_WithRequeue(t *testing.T) {
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

	queueName := testQueue + "-nack-requeue"

	_, err = ch.QueueDeclare(queueName, false, true, false, false, nil)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Set prefetch to 2
	err = ch.Qos(2, 0, false)
	if err != nil {
		t.Fatalf("Failed to set QoS: %v", err)
	}

	// Publish 4 messages
	for i := 0; i < 4; i++ {
		err = ch.PublishWithContext(
			context.Background(),
			"", queueName, false, false,
			amqp.Publishing{Body: []byte(fmt.Sprintf("Msg %d", i))},
		)
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	msgs, err := ch.Consume(queueName, "test", false, false, false, false, nil)
	if err != nil {
		t.Fatalf("Failed to consume: %v", err)
	}

	// Receive 2 messages (prefetch limit)
	var received []amqp.Delivery
	for i := 0; i < 2; i++ {
		select {
		case msg := <-msgs:
			received = append(received, msg)
			t.Logf("Received: %s, Redelivered: %v", string(msg.Body), msg.Redelivered)
		case <-time.After(1 * time.Second):
			t.Fatalf("Timeout receiving message %d", i)
		}
	}

	// Nack first message with requeue=true
	err = received[0].Nack(false, true)
	if err != nil {
		t.Fatalf("Failed to nack: %v", err)
	}

	// Should receive one more message (either the nacked one or a new one)
	select {
	case msg := <-msgs:
		t.Logf("Received after nack: %s, Redelivered: %v", string(msg.Body), msg.Redelivered)
		// If it's the redelivered message, the Redelivered flag should be true
		if string(msg.Body) == string(received[0].Body) && !msg.Redelivered {
			t.Error("Redelivered message should have Redelivered=true flag set")
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected to receive message after nack with requeue")
	}
}

// TestQoS_ZeroPrefetch_Unlimited verifies that prefetch=0 means unlimited
func TestQoS_ZeroPrefetch_Unlimited(t *testing.T) {
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

	queueName := testQueue + "-unlimited"

	_, err = ch.QueueDeclare(queueName, false, true, false, false, nil)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Set prefetch to 0 (unlimited)
	err = ch.Qos(0, 0, false)
	if err != nil {
		t.Fatalf("Failed to set QoS: %v", err)
	}

	// Publish 50 messages
	for i := 0; i < 50; i++ {
		err = ch.PublishWithContext(
			context.Background(),
			"", queueName, false, false,
			amqp.Publishing{Body: []byte(fmt.Sprintf("Msg %d", i))},
		)
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	msgs, err := ch.Consume(queueName, "test", false, false, false, false, nil)
	if err != nil {
		t.Fatalf("Failed to consume: %v", err)
	}

	// Should receive all 50 messages without any limit
	var count int32
	timeout := time.After(3 * time.Second)

	for count < 50 {
		select {
		case msg := <-msgs:
			atomic.AddInt32(&count, 1)
			if count%10 == 0 {
				t.Logf("Received %d messages so far", count)
			}
			// Don't ack - with prefetch=0, should still receive all
			_ = msg
		case <-timeout:
			goto done
		}
	}

done:
	if count < 50 {
		t.Errorf("Expected to receive all 50 messages with unlimited prefetch, got %d", count)
	} else {
		t.Logf("Successfully received all %d messages with prefetch=0", count)
	}
}
