package management

import (
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/andrelcunha/ottermq/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type exchangeFakeBroker struct {
	*fakeBroker
	brokerDetails   models.OverviewBrokerDetails
	nodeDetails     models.OverviewNodeDetails
	objectTotals    models.OverviewObjectTotals
	connectionStats models.OverviewConnectionStats
	config          models.BrokerConfigOverview
}

func setupTestBrokerForExchange(t *testing.T) *exchangeFakeBroker {
	t.Helper()
	base := setupTestBroker(t).(*fakeBroker)
	vh := base.vhosts["/"]

	// Create some test queues with messages
	props := &vhost.QueueProperties{
		Durable:    false,
		AutoDelete: false,
		Exclusive:  false,
	}
	_, err := vh.CreateQueue("queue1", props, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)
	_, err = vh.CreateQueue("queue2", props, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)

	// Bind queues to default exchange
	_ = vh.BindQueue("", "queue1", "queue1", nil, vhost.MANAGEMENT_CONNECTION_ID)
	_ = vh.BindQueue("", "queue2", "queue2", nil, vhost.MANAGEMENT_CONNECTION_ID)

	// Add messages to queues
	for i := 0; i < 10; i++ {
		msg := vhost.NewMessage(amqp.Message{
			Body: []byte("test"),
		}, vhost.GenerateMessageId())
		_, _ = vh.Publish("", "queue1", &msg)
	}

	for i := 0; i < 5; i++ {
		msg := vhost.NewMessage(amqp.Message{
			Body: []byte("test"),
		}, vhost.GenerateMessageId())
		_, _ = vh.Publish("", "queue2", &msg)
	}

	// Add unacked messages to queue1
	addUnackedMessages(vh, "queue1", 3)

	return &exchangeFakeBroker{
		fakeBroker: base,
		brokerDetails: models.OverviewBrokerDetails{
			Product:  "OtterMQ",
			Version:  "0.15.0",
			Platform: "golang",
		},
		nodeDetails: models.OverviewNodeDetails{
			Name:        "ottermq@localhost",
			MemoryUsage: 134217728,
			Goroutines:  42,
			FDUsed:      10,
			FDLimit:     1024,
		},
		objectTotals: models.OverviewObjectTotals{
			Connections: 3,
			Channels:    5,
			Exchanges:   8,
			Queues:      2,
			Consumers:   4,
		},
		connectionStats: models.OverviewConnectionStats{
			Total:             3,
			ClientConnections: 3,
			Running:           3,
		},
		config: models.BrokerConfigOverview{
			AMQPPort:           "5672",
			HTTPPort:           "3000",
			QueueBufferSize:    100000,
			PersistenceBackend: "json",
		},
	}
}

func TestCreateExchange_AllTypes(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	types := []string{"direct", "fanout", "topic"}
	// "headers" might not be fully supported in validation or parsing yet, checking CreateExchangeRequest validation
	// The request struct has `validate:"required,oneof=direct fanout topic headers"`
	// But ParseExchangeType in exchange.go only switches on direct, fanout, topic.
	// Let's stick to supported types.

	for _, typ := range types {
		exName := "test-" + typ
		t.Run(typ, func(t *testing.T) {
			req := models.CreateExchangeRequest{
				ExchangeType: typ,
				Durable:      true,
				AutoDelete:   false,
				Arguments:    nil,
			}
			dto, err := service.CreateExchange("/", exName, req)
			require.NoError(t, err)
			assert.Equal(t, exName, dto.Name)
			assert.Equal(t, typ, dto.Type)
			assert.True(t, dto.Durable)
		})
	}
}

func TestDeleteExchange_IfUnused(t *testing.T) {
	broker := setupTestBrokerForExchange(t)
	mockData := &metrics.MockMetricsData{
		QueueMetrics: map[string]metrics.QueueSnapshot{
			"test-queue": {
				Name:          "test-queue",
				MessageCount:  10,
				UnackedCount:  3,
				ConsumerCount: 2,
				AckCount:      10,
			},
		},
		ExchangeMetrics: map[string]metrics.ExchangeSnapshot{
			"test-exchange": {
				Name:          "test-exchange",
				Type:          "direct",
				PublishCount:  100,
				DeliveryCount: 95,
			},
		},
	}
	mockCollector := metrics.NewMockCollector(mockData)
	broker.collector = mockCollector
	broker.vhosts["/"].SetMetricsCollector(mockCollector)
	service := NewService(broker)

	// Create exchange
	exName := "test-exchange"
	_, err := service.CreateExchange("/", exName, models.CreateExchangeRequest{
		ExchangeType: "direct",
	})
	require.NoError(t, err)

	// Create queue
	qName := "test-queue"
	_, err = service.CreateQueue("/", qName, models.CreateQueueRequest{})
	require.NoError(t, err)

	vh := broker.GetVHost("/")

	err = vh.BindQueue(exName, qName, "key", nil, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)

	// Try delete with ifUnused=true (should fail)
	err = service.DeleteExchange("/", exName, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has active bindings")

	// Delete queue (should remove bindings)
	err = service.DeleteQueue("/", qName, false, false)
	require.NoError(t, err)

	// Try delete again (should succeed)
	err = service.DeleteExchange("/", exName, true)
	assert.NoError(t, err)
}

func TestGetExchange(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	exName := "my-exchange"
	req := models.CreateExchangeRequest{
		ExchangeType: "topic",
		Durable:      true,
		Arguments:    map[string]any{"x-custom": "value"},
	}
	_, err := service.CreateExchange("/", exName, req)
	require.NoError(t, err)

	dto, err := service.GetExchange("/", exName)
	require.NoError(t, err)
	assert.Equal(t, exName, dto.Name)
	assert.Equal(t, "topic", dto.Type)
	assert.Equal(t, "value", dto.Arguments["x-custom"])
}

func TestListExchanges(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	// Should have default exchanges initially
	initialList, err := service.ListExchanges()
	require.NoError(t, err)
	initialCount := len(initialList)

	// Create a new one
	exName := "list-test-exchange"
	_, err = service.CreateExchange("/", exName, models.CreateExchangeRequest{
		ExchangeType: "direct",
	})
	require.NoError(t, err)

	// List again
	newList, err := service.ListExchanges()
	require.NoError(t, err)
	assert.Equal(t, initialCount+1, len(newList))

	found := false
	for _, ex := range newList {
		if ex.Name == exName {
			found = true
			break
		}
	}
	assert.True(t, found, "Created exchange not found in list")
}

func TestCreateExchange_Idempotency(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	// Create exchange
	exName := "idempotent-exchange"
	_, err := service.CreateExchange("/", exName, models.CreateExchangeRequest{
		ExchangeType: "direct",
		Durable:      true,
	})
	require.NoError(t, err)

	// Create again with same props
	_, err = service.CreateExchange("/", exName, models.CreateExchangeRequest{
		ExchangeType: "direct",
		Durable:      true,
	})
	require.NoError(t, err)

	// Create again with different props (should fail)
	_, err = service.CreateExchange("/", exName, models.CreateExchangeRequest{
		ExchangeType: "fanout", // Different type
		Durable:      true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "different type")
}
