package metrics

import (
	"sync"
	"time"
)

// MockMetricsData holds pre-configured metrics data for testing.
type MockMetricsData struct {
	// Exchange metrics: map[exchangeName]snapshot
	ExchangeMetrics map[string]ExchangeSnapshot

	// Queue metrics: map[queueName]snapshot
	QueueMetrics map[string]QueueSnapshot

	// Broker-level metrics
	BrokerMetrics *BrokerSnapshot
}

// MockCollector is a simple mock implementation of MetricsCollector for testing.
// It returns pre-configured data instead of tracking actual metrics.
type MockCollector struct {
	mu sync.RWMutex

	// Pre-configured data to return
	exchangeSnapshots map[string]ExchangeSnapshot
	queueSnapshots    map[string]QueueSnapshot
	brokerSnapshot    *BrokerSnapshot

	enabled bool
}

// NewMockCollector creates a new mock collector with optional pre-configured data.
// If data is nil, returns empty metrics.
func NewMockCollector(data *MockMetricsData) *MockCollector {
	mock := &MockCollector{
		exchangeSnapshots: make(map[string]ExchangeSnapshot),
		queueSnapshots:    make(map[string]QueueSnapshot),
		brokerSnapshot: &BrokerSnapshot{
			ConnectionCount: 0,
			ChannelCount:    0,
			PublishRate:     0.0,
			DeliveryRate:    0.0,
			AckRate:         0.0,
		},
		enabled: true,
	}

	if data != nil {
		if data.ExchangeMetrics != nil {
			mock.exchangeSnapshots = data.ExchangeMetrics
		}
		if data.QueueMetrics != nil {
			mock.queueSnapshots = data.QueueMetrics
		}
		if data.BrokerMetrics != nil {
			mock.brokerSnapshot = data.BrokerMetrics
		}
	}

	return mock
}

// Exchange metrics - all methods are no-ops since we return pre-configured data
func (m *MockCollector) RecordExchangePublish(exchangeName, exchangeType string) {}

func (m *MockCollector) RecordExchangeDelivery(exchangeName string) {}

func (m *MockCollector) GetExchangeMetrics(exchangeName string) *ExchangeMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot, ok := m.exchangeSnapshots[exchangeName]
	if !ok {
		return nil
	}

	// Convert snapshot back to ExchangeMetrics for compatibility
	em := &ExchangeMetrics{
		Name:      snapshot.Name,
		Type:      snapshot.Type,
		CreatedAt: snapshot.CreatedAt,
	}
	em.PublishCount.Store(snapshot.PublishCount)
	em.DeliveryCount.Store(snapshot.DeliveryCount)

	return em
}

func (m *MockCollector) GetAllExchangeMetrics() []*ExchangeMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ExchangeMetrics, 0, len(m.exchangeSnapshots))
	for _, snapshot := range m.exchangeSnapshots {
		em := &ExchangeMetrics{
			Name:      snapshot.Name,
			Type:      snapshot.Type,
			CreatedAt: snapshot.CreatedAt,
		}
		em.PublishCount.Store(snapshot.PublishCount)
		em.DeliveryCount.Store(snapshot.DeliveryCount)
		result = append(result, em)
	}
	return result
}

func (m *MockCollector) RemoveExchange(exchangeName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.exchangeSnapshots, exchangeName)
}

// Queue metrics - all recording methods are no-ops
func (m *MockCollector) RecordQueuePublish(queueName string) {}

func (m *MockCollector) RecordQueueRequeue(queueName string) {}

func (m *MockCollector) RecordQueueDelivery(queueName string, autoAck bool) {}

func (m *MockCollector) RecordQueueAck(queueName string) {}

func (m *MockCollector) RecordQueueNack(queueName string) {}

func (m *MockCollector) SetQueueDepth(queueName string, depth int64) {}

func (m *MockCollector) RecordConsumerAdded(queueName string) {}

func (m *MockCollector) RecordConsumerRemoved(queueName string) {}

func (m *MockCollector) GetQueueMetrics(queueName string) *QueueMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot, ok := m.queueSnapshots[queueName]
	if !ok {
		return nil
	}

	// Convert snapshot back to QueueMetrics for compatibility
	qm := &QueueMetrics{
		Name:      snapshot.Name,
		CreatedAt: snapshot.CreatedAt,
	}
	qm.MessageCount.Store(snapshot.MessageCount)
	qm.UnackedCount.Store(snapshot.UnackedCount)
	qm.ConsumerCount.Store(snapshot.ConsumerCount)
	qm.AckCount.Store(snapshot.AckCount)

	return qm
}

func (m *MockCollector) GetAllQueueMetrics() []*QueueMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*QueueMetrics, 0, len(m.queueSnapshots))
	for _, snapshot := range m.queueSnapshots {
		qm := &QueueMetrics{
			Name:      snapshot.Name,
			CreatedAt: snapshot.CreatedAt,
		}
		qm.MessageCount.Store(snapshot.MessageCount)
		qm.UnackedCount.Store(snapshot.UnackedCount)
		qm.ConsumerCount.Store(snapshot.ConsumerCount)
		qm.AckCount.Store(snapshot.AckCount)
		result = append(result, qm)
	}
	return result
}

func (m *MockCollector) RemoveQueue(queueName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.queueSnapshots, queueName)
}

// Broker-level metrics - all recording methods are no-ops
func (m *MockCollector) RecordConnection() {}

func (m *MockCollector) RecordConnectionClose() {}

func (m *MockCollector) RecordChannelOpen() {}

func (m *MockCollector) RecordChannelClose() {}

func (m *MockCollector) GetBrokerMetrics() *BrokerMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bm := &BrokerMetrics{}
	if m.brokerSnapshot != nil {

		bm.connectionCount = m.brokerSnapshot.ConnectionCount
		bm.channelCount = m.brokerSnapshot.ChannelCount
		// Rates are handled by the snapshot when needed
	}

	return bm
}

func (m *MockCollector) GetBrokerSnapshot() *BrokerSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.brokerSnapshot
}

func (m *MockCollector) GetExchangeSnapshot(exchangeName string) *ExchangeSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot, ok := m.exchangeSnapshots[exchangeName]
	if !ok {
		return nil
	}
	return &snapshot
}

func (m *MockCollector) GetQueueSnapshot(queueName string) *QueueSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot, ok := m.queueSnapshots[queueName]
	if !ok {
		return nil
	}
	return &snapshot
}

// Time series (mock returns empty slices)
func (m *MockCollector) GetPublishRateTimeSeries(duration time.Duration) []Sample {
	return []Sample{}
}

func (m *MockCollector) GetDeliveryAutoAckRateTimeSeries(duration time.Duration) []Sample {
	return []Sample{}
}

func (m *MockCollector) GetDeliveryManualAckRateTimeSeries(duration time.Duration) []Sample {
	return []Sample{}
}

func (m *MockCollector) GetAckRateTimeSeries(duration time.Duration) []Sample {
	return []Sample{}
}

func (m *MockCollector) GetConnectionRateTimeSeries(duration time.Duration) []Sample {
	return []Sample{}
}

func (m *MockCollector) StartPeriodicSampling() {}

// Utility
func (m *MockCollector) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exchangeSnapshots = make(map[string]ExchangeSnapshot)
	m.queueSnapshots = make(map[string]QueueSnapshot)
}

func (m *MockCollector) IsEnabled() bool {
	return m.enabled
}

// Helper methods to update mock data during tests if needed
func (m *MockCollector) SetExchangeMetrics(name string, snapshot ExchangeSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exchangeSnapshots[name] = snapshot
}

func (m *MockCollector) SetQueueMetrics(name string, snapshot QueueSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queueSnapshots[name] = snapshot
}

func (m *MockCollector) SetBrokerMetrics(snapshot *BrokerSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.brokerSnapshot = snapshot
}
