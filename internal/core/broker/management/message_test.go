package management

import (
	"testing"
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishMessage_Success(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	// Create a queue first
	vh := broker.GetVHost("/")
	props := &vhost.QueueProperties{
		Durable:    false,
		AutoDelete: false,
		Exclusive:  false,
	}
	_, err := vh.CreateQueue("test-queue", props, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)

	// Bind queue to default exchange
	err = vh.BindQueue("", "test-queue", "test-queue", nil, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)

	req := models.PublishMessageRequest{
		RoutingKey:    "test-queue",
		Payload:       "test message",
		ContentType:   "text/plain",
		DeliveryMode:  2,
		Priority:      5,
		CorrelationId: "corr-123",
		ReplyTo:       "reply-queue",
		Expiration:    "60000",
		MessageID:     "msg-123",
		Type:          "test-type",
		UserId:        "guest",
		AppId:         "test-app",
		Headers:       map[string]any{"x-custom": "value"},
	}

	err = service.PublishMessage("/", "", req)
	require.NoError(t, err)

	// Verify message was published
	msgCount, _ := vh.GetMessageCount("test-queue")
	assert.Equal(t, 1, msgCount)
}

func TestPublishMessage_NonExistentVHost(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	req := models.PublishMessageRequest{
		RoutingKey: "test-queue",
		Payload:    "test",
	}

	err := service.PublishMessage("/nonexistent", "", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPublishMessage_NoRoute(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	req := models.PublishMessageRequest{
		RoutingKey: "nonexistent-queue",
		Payload:    "test",
	}

	// Should not error, but message is dropped
	err := service.PublishMessage("/", "", req)
	assert.NoError(t, err)
}

func TestGetMessages_Success(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	vh := broker.GetVHost("/")
	props := &vhost.QueueProperties{
		Durable:    false,
		AutoDelete: false,
		Exclusive:  false,
	}
	_, err := vh.CreateQueue("test-queue", props, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)

	// Bind queue to default exchange
	err = vh.BindQueue("", "test-queue", "test-queue", nil, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)

	// Publish some messages
	for i := 0; i < 5; i++ {
		msg := vhost.NewMessage(amqp.Message{
			Body: []byte("message"),
			Properties: amqp.BasicProperties{
				ContentType:  "text/plain",
				DeliveryMode: 1,
			},
		}, vhost.GenerateMessageId())
		_, err := vh.Publish("", "test-queue", &msg)
		require.NoError(t, err)
	}

	msgs, err := service.GetMessages("/", "test-queue", 3, models.NoAck)
	require.NoError(t, err)

	assert.Len(t, msgs, 3)
	assert.NotEmpty(t, msgs[0].ID)
	assert.NotEmpty(t, msgs[0].Payload)
	assert.Greater(t, msgs[0].DeliveryTag, uint64(0))
}

func TestGetMessages_WithAck(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	vh := broker.GetVHost("/")
	props := &vhost.QueueProperties{
		Durable:    false,
		AutoDelete: false,
		Exclusive:  false,
	}
	_, err := vh.CreateQueue("test-queue", props, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)

	// Bind queue to default exchange
	err = vh.BindQueue("", "test-queue", "test-queue", nil, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)

	msg := vhost.NewMessage(amqp.Message{
		Body: []byte("message"),
	}, vhost.GenerateMessageId())
	_, err = vh.Publish("", "test-queue", &msg)
	require.NoError(t, err)

	msgs, err := service.GetMessages("/", "test-queue", 1, models.Ack)
	require.NoError(t, err)

	assert.Len(t, msgs, 1)

	// Verify message is tracked as unacked
	key := vhost.ConnectionChannelKey{
		ConnectionID: vhost.MANAGEMENT_CONNECTION_ID,
		Channel:      0,
	}
	state := vh.ChannelDeliveries[key]
	require.NotNil(t, state)
	assert.Len(t, state.UnackedByTag, 1)
}

func TestGetMessages_EmptyQueue(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	vh := broker.GetVHost("/")
	props := &vhost.QueueProperties{
		Durable:    false,
		AutoDelete: false,
		Exclusive:  false,
	}
	_, err := vh.CreateQueue("empty-queue", props, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)

	msgs, err := service.GetMessages("/", "empty-queue", 10, models.NoAck)
	require.NoError(t, err)

	assert.Empty(t, msgs)
}

func TestGetMessages_NonExistentVHost(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	_, err := service.GetMessages("/nonexistent", "test-queue", 1, models.NoAck)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetMessages_EmptyQueueName(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	_, err := service.GetMessages("/", "", 1, models.NoAck)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestGetMessages_InvalidCount(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	_, err := service.GetMessages("/", "test-queue", 0, models.NoAck)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "greater than zero")
}

func TestGetPropsFromRequest(t *testing.T) {
	timestamp := time.Now()
	req := models.PublishMessageRequest{
		ContentType:   "application/json",
		DeliveryMode:  2,
		Priority:      7,
		CorrelationId: "test-corr",
		ReplyTo:       "test-reply",
		Expiration:    "30000",
		MessageID:     "msg-456",
		Timestamp:     &timestamp,
		Type:          "test-type",
		UserId:        "user1",
		AppId:         "app1",
		Headers:       map[string]any{"key": "value"},
	}

	props := getPropsFromRequest(req)

	assert.Equal(t, amqp.ContentType("application/json"), props.ContentType)
	assert.Equal(t, amqp.DeliveryMode(2), props.DeliveryMode)
	assert.Equal(t, uint8(7), props.Priority)
	assert.Equal(t, "test-corr", props.CorrelationID)
	assert.Equal(t, "test-reply", props.ReplyTo)
	assert.Equal(t, "30000", props.Expiration)
	assert.Equal(t, "msg-456", props.MessageID)
	assert.Equal(t, timestamp, props.Timestamp)
	assert.Equal(t, "test-type", props.Type)
	assert.Equal(t, "user1", props.UserID)
	assert.Equal(t, "app1", props.AppID)
	assert.Equal(t, "value", props.Headers["key"])
}

func TestGetPropsFromRequest_DefaultDeliveryMode(t *testing.T) {
	// Test invalid delivery modes default to transient (1)
	tests := []struct {
		name         string
		deliveryMode uint8
		expected     amqp.DeliveryMode
	}{
		{"zero", 0, 1},
		{"one", 1, 1},
		{"two", 2, 2},
		{"invalid", 3, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := models.PublishMessageRequest{
				DeliveryMode: tt.deliveryMode,
			}
			props := getPropsFromRequest(req)
			assert.Equal(t, tt.expected, props.DeliveryMode)
		})
	}
}

func TestPropertiesToMap(t *testing.T) {
	timestamp := time.Now()
	props := amqp.BasicProperties{
		ContentType:     "text/plain",
		ContentEncoding: "utf-8",
		Headers:         map[string]any{"x-key": "x-value"},
		DeliveryMode:    2,
		Priority:        5,
		CorrelationID:   "corr-789",
		ReplyTo:         "reply",
		Expiration:      "60000",
		MessageID:       "msg-789",
		Timestamp:       timestamp,
		Type:            "type1",
		UserID:          "user2",
		AppID:           "app2",
	}

	result := propertiesToMap(props)

	assert.Equal(t, "text/plain", result["ContentType"])
	assert.Equal(t, "utf-8", result["ContentEncoding"])
	assert.Equal(t, "2", result["DeliveryMode"])
	assert.Equal(t, "5", result["Priority"])
	assert.Equal(t, "corr-789", result["CorrelationId"])
	assert.Equal(t, "reply", result["ReplyTo"])
	assert.Equal(t, "60000", result["Expiration"])
	assert.Equal(t, "msg-789", result["MessageId"])
	assert.Equal(t, timestamp.String(), result["Timestamp"])
	assert.Equal(t, "type1", result["Type"])
	assert.Equal(t, "user2", result["UserId"])
	assert.Equal(t, "app2", result["AppId"])
	assert.Equal(t, "x-value", result["Headers"].(map[string]any)["x-key"])
}
