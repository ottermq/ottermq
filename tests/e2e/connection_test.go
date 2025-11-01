package e2e

import (
	"strings"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestChannelCloseScenarios(t *testing.T) {
	tests := []struct {
		name       string
		trigger    func(ch *amqp.Channel) error
		expectCode int
		expectText string
	}{
		{
			name: "Passive Declare on non-existent queue",
			trigger: func(ch *amqp.Channel) error {
				_, err := ch.QueueDeclarePassive("ghost_queue", false, false, false, false, nil)
				return err
			},
			expectCode: amqp.NotFound,
			expectText: "no queue",
		},
		{
			name: "Publish to non-existent exchange",
			trigger: func(ch *amqp.Channel) error {
				return ch.Publish("ghost_exchange", "key", false, false, amqp.Publishing{
					ContentType: "text/plain",
					Body:        []byte("test"),
				})
			},
			expectCode: amqp.NotFound,
			expectText: "no exchange",
		},
		{
			name: "Passive Declare on non-existent exchange",
			trigger: func(ch *amqp.Channel) error {
				err := ch.ExchangeDeclarePassive("ghost_exchange", "direct", false, false, false, false, nil)
				return err
			},
			expectCode: amqp.NotFound,
			expectText: "no exchange",
		},
		{
			name: "Bind to non-existent exchange",
			trigger: func(ch *amqp.Channel) error {
				return ch.QueueBind(
					"some_queue",
					"key",
					"ghost_exchange",
					false,
					nil,
				)
			},
			expectCode: amqp.NotFound,
			expectText: "no exchange",
		},
		{
			name: "Bind to non-existent exchange",
			trigger: func(ch *amqp.Channel) error {
				return ch.QueueBind(
					"some_queue",
					"key",
					"ghost_exchange",
					false,
					nil,
				)
			},
			expectCode: amqp.NotFound,
			expectText: "no exchange",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create connection
			conn, err := amqp.Dial(brokerURL)
			if err != nil {
				t.Fatalf("Failed to connect: %v", err)
			}
			defer conn.Close()

			// Open channel
			ch, err := conn.Channel()
			if err != nil {
				t.Fatalf("Failed to open channel: %v", err)
			}
			// Don't defer ch.Close() here - we expect it to be closed by the error

			// Trigger the error condition
			err = tt.trigger(ch)
			if err == nil {
				t.Fatalf("Expected error for test: %s", tt.name)
			}

			// Verify it's an AMQP error with expected code
			amqpErr, ok := err.(*amqp.Error)
			if !ok {
				t.Fatalf("Expected AMQP error, got: %v", err)
			}
			if amqpErr.Code != tt.expectCode {
				t.Errorf("Expected error code %d, got %d", tt.expectCode, amqpErr.Code)
			}
			if !strings.Contains(amqpErr.Reason, tt.expectText) {
				t.Errorf("Expected error text to contain '%s', got '%s'", tt.expectText, amqpErr.Reason)
			}

			// Verify channel is closed by attempting another operation
			_, err = ch.QueueDeclare("should_fail", false, false, false, false, nil)
			if err == nil {
				t.Fatalf("Expected channel to be closed, but operation succeeded")
			}

			// The connection should still be valid, but we'll close it in defer
		})
	}
}

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
