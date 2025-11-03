package e2e

import (
	"strings"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestChannelCloseOnPassiveDeclareFailure(t *testing.T) {
	// Setup broker and connections
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatalf("Failed to connect to broker: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}

	t.Logf("Triggering channel exception with passive declare")
	_, err = ch.QueueDeclarePassive(
		"non_existent_queue",
		false,
		false,
		false,
		false,
		nil,
	)
	if err == nil {
		t.Fatalf("Expected channel error when declaring non-existent passive queue")
	}
	amqpErr, ok := err.(*amqp.Error)
	if !ok {
		t.Fatalf("Expected AMQP error, got: %v", err)
	}
	if amqpErr.Code != amqp.NotFound {
		if !strings.Contains(amqpErr.Reason, "no queue") {
			t.Errorf("Unexpected error reason: %s", amqpErr.Reason)
		}
		t.Fatalf("Expected NotFound error code, got: %v", amqpErr.Code)
	}

	// Now the broker should have sent channel.close
	// Any further requests (except channel.close-ok & channel.close-ok) should be ignored

	// Attempt to declare another queue
	_, err = ch.QueueDeclare(
		"another_queue",
		false,
		false,
		false,
		false,
		nil,
	)
	if err == nil {
		t.Fatalf("Expected error when declaring another queue after channel.close")
	}
	// Here we would check that the broker ignored the request

	// Finally, close the channel and connection properly
	if err := ch.Close(); err != nil {
		t.Fatalf("Failed to close channel: %v", err)
	}
	// Here we would check that the broker properly closed the connection
	if err := conn.Close(); err != nil {
		t.Fatalf("Failed to close connection: %v", err)
	}

}
