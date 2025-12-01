package management

import (
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type overviewFakeBroker struct {
	*fakeBroker
	brokerDetails   models.OverviewBrokerDetails
	nodeDetails     models.OverviewNodeDetails
	objectTotals    models.OverviewObjectTotals
	connectionStats models.OverviewConnectionStats
	config          models.BrokerConfigOverview
}

func (ofb *overviewFakeBroker) GetBrokerOverviewDetails() models.OverviewBrokerDetails {
	return ofb.brokerDetails
}

func (ofb *overviewFakeBroker) GetOverviewNodeDetails() models.OverviewNodeDetails {
	return ofb.nodeDetails
}

func (ofb *overviewFakeBroker) GetObjectTotalsOverview() models.OverviewObjectTotals {
	return ofb.objectTotals
}

func (ofb *overviewFakeBroker) GetOverviewConnStats() models.OverviewConnectionStats {
	return ofb.connectionStats
}

func (ofb *overviewFakeBroker) GetBrokerOverviewConfig() models.BrokerConfigOverview {
	return ofb.config
}

func setupTestBrokerForOverview(t *testing.T) *overviewFakeBroker {
	t.Helper()
	base := setupTestBroker(t).(*fakeBroker)

	// Create some test queues with messages
	vh := base.vhosts["/"]
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

	return &overviewFakeBroker{
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

func TestGetOverview_Success(t *testing.T) {
	broker := setupTestBrokerForOverview(t)
	service := NewService(broker)

	overview, err := service.GetOverview()
	require.NoError(t, err)
	require.NotNil(t, overview)

	// Verify broker details
	assert.Equal(t, "OtterMQ", overview.BrokerDetails.Product)
	assert.Equal(t, "0.15.0", overview.BrokerDetails.Version)
	assert.Equal(t, "golang", overview.BrokerDetails.Platform)

	// Verify node details
	assert.Equal(t, "ottermq@localhost", overview.NodeDetails.Name)
	assert.Equal(t, 134217728, overview.NodeDetails.MemoryUsage)
	assert.Equal(t, 42, overview.NodeDetails.Goroutines)

	// Verify object totals
	assert.Equal(t, 3, overview.ObjectTotals.Connections)
	assert.Equal(t, 5, overview.ObjectTotals.Channels)
	assert.Equal(t, 8, overview.ObjectTotals.Exchanges)
	assert.Equal(t, 2, overview.ObjectTotals.Queues)
	assert.Equal(t, 4, overview.ObjectTotals.Consumers)

	// Verify connection stats
	assert.Equal(t, 3, overview.ConnectionStats.Total)
	assert.Equal(t, 3, overview.ConnectionStats.ClientConnections)
	assert.Equal(t, 3, overview.ConnectionStats.Running)

	// Verify configuration
	assert.Equal(t, "5672", overview.Configuration.AMQPPort)
	assert.Equal(t, "3000", overview.Configuration.HTTPPort)
	assert.Equal(t, 100000, overview.Configuration.QueueBufferSize)
	assert.Equal(t, "json", overview.Configuration.PersistenceBackend)

	// Verify message stats
	assert.Equal(t, 15, overview.MessageStats.MessagesReady)  // 10 + 5
	assert.Equal(t, 3, overview.MessageStats.MessagesUnacked) // 3 unacked in queue1
	assert.Equal(t, 18, overview.MessageStats.MessagesTotal)  // 15 + 3
}

func TestGetMessageStats_MultipleQueues(t *testing.T) {
	broker := setupTestBrokerForOverview(t)
	service := NewService(broker)

	stats := service.GetMessageStats()

	assert.Equal(t, 15, stats.MessagesReady)
	assert.Equal(t, 3, stats.MessagesUnacked)
	assert.Equal(t, 18, stats.MessagesTotal)
	assert.Len(t, stats.QueueStats, 2)

	// Find queue1 stats
	var queue1Stats *models.QueueMessageBreakdown
	for i := range stats.QueueStats {
		if stats.QueueStats[i].QueueName == "queue1" {
			queue1Stats = &stats.QueueStats[i]
			break
		}
	}
	require.NotNil(t, queue1Stats)
	assert.Equal(t, "/", queue1Stats.VHost)
	assert.Equal(t, "queue1", queue1Stats.QueueName)
	assert.Equal(t, 10, queue1Stats.MessagesReady)
	assert.Equal(t, 3, queue1Stats.MessagesUnacked)
}

func TestGetMessageStats_EmptyBroker(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	stats := service.GetMessageStats()

	assert.Equal(t, 0, stats.MessagesReady)
	assert.Equal(t, 0, stats.MessagesUnacked)
	assert.Equal(t, 0, stats.MessagesTotal)
	assert.Empty(t, stats.QueueStats)
}

func TestGetBrokerInfo(t *testing.T) {
	broker := setupTestBrokerForOverview(t)
	service := NewService(broker)

	info := service.GetBrokerInfo()

	assert.Equal(t, "OtterMQ", info.Product)
	assert.Equal(t, "0.15.0", info.Version)
	assert.Equal(t, "golang", info.Platform)
}

func TestGetOverview_AllFieldsPopulated(t *testing.T) {
	broker := setupTestBrokerForOverview(t)
	service := NewService(broker)

	overview, err := service.GetOverview()
	require.NoError(t, err)

	// Ensure all top-level fields are populated
	assert.NotEmpty(t, overview.BrokerDetails.Product)
	assert.NotEmpty(t, overview.NodeDetails.Name)
	assert.NotZero(t, overview.ObjectTotals.Connections)
	assert.NotNil(t, overview.MessageStats)
	assert.NotZero(t, overview.ConnectionStats.Total)
	assert.NotEmpty(t, overview.Configuration.AMQPPort)
}

func TestGetMessageStats_VerifyAggregation(t *testing.T) {
	broker := setupTestBroker(t).(*fakeBroker)
	vh := broker.vhosts["/"]

	// Create 3 queues with different message counts
	props := &vhost.QueueProperties{
		Durable:    false,
		AutoDelete: false,
		Exclusive:  false,
	}
	_, _ = vh.CreateQueue("q1", props, vhost.MANAGEMENT_CONNECTION_ID)
	_, _ = vh.CreateQueue("q2", props, vhost.MANAGEMENT_CONNECTION_ID)
	_, _ = vh.CreateQueue("q3", props, vhost.MANAGEMENT_CONNECTION_ID)

	// Bind queues to default exchange
	_ = vh.BindQueue("", "q1", "q1", nil, vhost.MANAGEMENT_CONNECTION_ID)
	_ = vh.BindQueue("", "q2", "q2", nil, vhost.MANAGEMENT_CONNECTION_ID)
	_ = vh.BindQueue("", "q3", "q3", nil, vhost.MANAGEMENT_CONNECTION_ID)

	// Add messages: q1=10, q2=20, q3=30
	for i := 0; i < 10; i++ {
		msg := vhost.NewMessage(amqp.Message{Body: []byte("test")}, vhost.GenerateMessageId())
		_, _ = vh.Publish("", "q1", &msg)
	}
	for i := 0; i < 20; i++ {
		msg := vhost.NewMessage(amqp.Message{Body: []byte("test")}, vhost.GenerateMessageId())
		_, _ = vh.Publish("", "q2", &msg)
	}
	for i := 0; i < 30; i++ {
		msg := vhost.NewMessage(amqp.Message{Body: []byte("test")}, vhost.GenerateMessageId())
		_, _ = vh.Publish("", "q3", &msg)
	}

	// Add unacked: q1=2, q2=5, q3=8
	addUnackedMessages(vh, "q1", 2)
	addUnackedMessages(vh, "q2", 5)
	addUnackedMessages(vh, "q3", 8)

	service := NewService(broker)
	stats := service.GetMessageStats()

	// Verify totals
	assert.Equal(t, 60, stats.MessagesReady)   // 10+20+30
	assert.Equal(t, 15, stats.MessagesUnacked) // 2+5+8
	assert.Equal(t, 75, stats.MessagesTotal)   // 60+15
	assert.Len(t, stats.QueueStats, 3)
}
