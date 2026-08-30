package management

import (
	"testing"

	"github.com/ottermq/ottermq/internal/core/amqp"
	"github.com/ottermq/ottermq/internal/core/broker/vhost"
	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateQueue_WithAllProperties(t *testing.T) {
	// Setup broker and service
	broker := setupTestBroker(t)
	service := NewService(broker)

	// Create queue with TTL, DLX, QLL
	maxLen := int32(1000)
	ttl := int64(60000)
	dlx := "my-dlx"

	req := models.CreateQueueRequest{
		Durable:            true,
		AutoDelete:         false,
		MaxLength:          &maxLen,
		MessageTTL:         &ttl,
		DeadLetterExchange: &dlx,
	}

	dto, err := service.CreateQueue("/", "", req)
	require.NoError(t, err)

	assert.NotEmpty(t, dto.Name) // Auto-generated name
	assert.True(t, dto.Durable)
	assert.NotNil(t, dto.MaxLength)
	assert.Equal(t, int32(1000), *dto.MaxLength)
	assert.NotNil(t, dto.MessageTTL)
	assert.Equal(t, int64(60000), *dto.MessageTTL)
	assert.NotNil(t, dto.DeadLetterExchange)
	assert.Equal(t, "my-dlx", *dto.DeadLetterExchange)
}

func TestDeleteQueue_IfUnused(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	// Create queue
	queueName := "test-queue"
	service.CreateQueue("/", queueName, models.CreateQueueRequest{})

	// Add consumer (simulate)
	vh := broker.GetVHost("/")
	vh.ConsumersByQueue[queueName] = []*vhost.Consumer{
		{Tag: "consumer-1"},
	}

	// Try to delete with ifUnused=true (should fail)
	err := service.DeleteQueue("/", queueName, true, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has 1 consumers")

	// Remove consumer
	vh.ConsumersByQueue[queueName] = nil

	// Try again (should succeed)
	err = service.DeleteQueue("/", queueName, true, false)
	assert.NoError(t, err)
}

func TestListQueues_ShowsUnackedCount(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	// Create queue
	queueName := "test-queue"
	service.CreateQueue("/", queueName, models.CreateQueueRequest{})

	// Simulate unacked messages
	vh := broker.GetVHost("/")
	addUnackedMessages(vh, queueName, 5)

	// List queues
	dtos := service.ListQueues()
	require.Len(t, dtos, 1)

	assert.Equal(t, 5, dtos[0].MessagesUnacked)
}

func TestSortFunc(t *testing.T) {
	tests := []struct {
		name string
		a, b models.QueueDTO
		want int
	}{
		{
			name: "same vhost, a before b by name",
			a:    models.QueueDTO{VHost: "/", Name: "alpha"},
			b:    models.QueueDTO{VHost: "/", Name: "beta"},
			want: -1,
		},
		{
			name: "same vhost, a after b by name",
			a:    models.QueueDTO{VHost: "/", Name: "beta"},
			b:    models.QueueDTO{VHost: "/", Name: "alpha"},
			want: 1,
		},
		{
			name: "vhost takes precedence over name",
			a:    models.QueueDTO{VHost: "a-host", Name: "zzz"},
			b:    models.QueueDTO{VHost: "b-host", Name: "aaa"},
			want: -1,
		},
		{
			name: "identical vhost and name",
			a:    models.QueueDTO{VHost: "/", Name: "same"},
			b:    models.QueueDTO{VHost: "/", Name: "same"},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sortFunc(tt.a, tt.b))
		})
	}
}

func TestListQueues_SortedByVhostThenName(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)
	require.NoError(t, broker.CreateVHost("beta-host"))

	// Create queues out of order, across two vhosts, to prove ListQueues
	// sorts its result rather than just happening to return creation order
	// (which Go map iteration never guarantees).
	_, err := service.CreateQueue("beta-host", "aaa-queue", models.CreateQueueRequest{})
	require.NoError(t, err)
	_, err = service.CreateQueue("/", "zzz-queue", models.CreateQueueRequest{})
	require.NoError(t, err)
	_, err = service.CreateQueue("/", "mmm-queue", models.CreateQueueRequest{})
	require.NoError(t, err)

	dtos := service.ListQueues()
	require.Len(t, dtos, 3)

	assert.Equal(t, "/", dtos[0].VHost)
	assert.Equal(t, "mmm-queue", dtos[0].Name)
	assert.Equal(t, "/", dtos[1].VHost)
	assert.Equal(t, "zzz-queue", dtos[1].Name)
	assert.Equal(t, "beta-host", dtos[2].VHost)
	assert.Equal(t, "aaa-queue", dtos[2].Name)
}

func TestGetQueue(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	// Create queue
	queueName := "get-queue"
	_, err := service.CreateQueue("/", queueName, models.CreateQueueRequest{
		Durable: true,
	})
	require.NoError(t, err)

	// Get queue
	dto, err := service.GetQueue("/", queueName)
	require.NoError(t, err)
	assert.Equal(t, queueName, dto.Name)
	assert.True(t, dto.Durable)
}

func TestPurgeQueue(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	// Create queue
	queueName := "purge-queue"
	_, err := service.CreateQueue("/", queueName, models.CreateQueueRequest{})
	require.NoError(t, err)

	// Add messages
	vh := broker.GetVHost("/")

	msg := vhost.NewMessage(amqp.Message{
		Body: []byte("test message"),
	}, "msg-id-1")

	// Publish to default exchange (direct) with routing key = queue name
	_, err = vh.Publish("", queueName, &msg)
	require.NoError(t, err)

	// Verify count before purge
	dto, err := service.GetQueue("/", queueName)
	require.NoError(t, err)
	assert.Equal(t, 1, dto.Messages)

	// Purge queue
	count, err := service.PurgeQueue("/", queueName)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify count after purge
	dto, err = service.GetQueue("/", queueName)
	require.NoError(t, err)
	assert.Equal(t, 0, dto.Messages)
}

func TestCreateQueue_Idempotency(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	// Create queue
	queueName := "idempotent-queue"
	_, err := service.CreateQueue("/", queueName, models.CreateQueueRequest{
		Durable: true,
	})
	require.NoError(t, err)

	// Create again with same props
	_, err = service.CreateQueue("/", queueName, models.CreateQueueRequest{
		Durable: true,
	})
	require.NoError(t, err)

	// Create again with different props (should fail)
	_, err = service.CreateQueue("/", queueName, models.CreateQueueRequest{
		Durable: false, // Different
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "different properties")
}

// addUnackedMessages simulates unacked messages by inserting DeliveryRecords into
// a synthetic ChannelDeliveryState for the provided queue name.
func addUnackedMessages(vh *vhost.VHost, queueName string, count int) {
	key := vhost.ConnectionChannelKey{ConnectionID: vhost.MANAGEMENT_CONNECTION_ID, Channel: 0}
	state, ok := vh.ChannelDeliveries[key]
	if !ok {
		state = &vhost.ChannelDeliveryState{
			UnackedByTag:      make(map[uint64]*vhost.DeliveryRecord),
			UnackedByConsumer: make(map[string]map[uint64]*vhost.DeliveryRecord),
		}
		vh.ChannelDeliveries[key] = state
	}
	consumerTag := "test-consumer"
	if state.UnackedByConsumer[consumerTag] == nil {
		state.UnackedByConsumer[consumerTag] = make(map[uint64]*vhost.DeliveryRecord)
	}
	// Start from LastDeliveryTag+1 to avoid overwriting previous unacked messages
	startTag := state.LastDeliveryTag + 1
	for i := range count {
		tag := startTag + uint64(i)
		rec := &vhost.DeliveryRecord{DeliveryTag: tag, ConsumerTag: consumerTag, QueueName: queueName}
		state.UnackedByTag[tag] = rec
		state.UnackedByConsumer[consumerTag][tag] = rec
		state.LastDeliveryTag = tag
	}
}
