package metrics

import (
	"sync"
	"time"
)

// MockCollector is a simple mock implementation of MetricsCollector for testing.
type MockCollector struct {
	mu sync.RWMutex

	// Simplified storage for testing
	exchangeMetrics map[string]*ExchangeMetrics
	queueMetrics    map[string]*QueueMetrics
	brokerMetrics   *BrokerMetrics

	enabled bool
}

// NewMockCollector creates a new mock collector.
func NewMockCollector() *MockCollector {
	return &MockCollector{
		exchangeMetrics: make(map[string]*ExchangeMetrics),
		queueMetrics:    make(map[string]*QueueMetrics),
		brokerMetrics: &BrokerMetrics{
			totalPublishes:  NewRateTracker(5*time.Minute, 60),
			totalDeliveries: NewRateTracker(5*time.Minute, 60),
			totalAcks:       NewRateTracker(5*time.Minute, 60),
			totalNacks:      NewRateTracker(5*time.Minute, 60),
			connectionRate:  NewRateTracker(5*time.Minute, 60),
			channelRate:     NewRateTracker(5*time.Minute, 60),
		},
		enabled: true,
	}
}

// Exchange metrics
func (m *MockCollector) RecordExchangePublish(exchangeName, exchangeType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.exchangeMetrics[exchangeName]; !ok {
		m.exchangeMetrics[exchangeName] = &ExchangeMetrics{
			Name:         exchangeName,
			Type:         exchangeType,
			PublishRate:  NewRateTracker(5*time.Minute, 60),
			DeliveryRate: NewRateTracker(5*time.Minute, 60),
			CreatedAt:    time.Now(),
		}
	}
	m.exchangeMetrics[exchangeName].PublishCount.Add(1)
}

func (m *MockCollector) RecordExchangeDelivery(exchangeName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ex, ok := m.exchangeMetrics[exchangeName]; ok {
		ex.DeliveryCount.Add(1)
	}
}

func (m *MockCollector) GetExchangeMetrics(exchangeName string) *ExchangeMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.exchangeMetrics[exchangeName]
}

func (m *MockCollector) GetAllExchangeMetrics() []*ExchangeMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ExchangeMetrics, 0, len(m.exchangeMetrics))
	for _, em := range m.exchangeMetrics {
		result = append(result, em)
	}
	return result
}

func (m *MockCollector) RemoveExchange(exchangeName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.exchangeMetrics, exchangeName)
}

// Queue metrics
func (m *MockCollector) RecordQueuePublish(queueName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.queueMetrics[queueName]; !ok {
		m.queueMetrics[queueName] = &QueueMetrics{
			Name:         queueName,
			MessageRate:  NewRateTracker(5*time.Minute, 60),
			DeliveryRate: NewRateTracker(5*time.Minute, 60),
			AckRate:      NewRateTracker(5*time.Minute, 60),
			CreatedAt:    time.Now(),
		}
	}
	m.queueMetrics[queueName].MessageCount.Add(1)
}

func (m *MockCollector) RecordQueueRequeue(queueName string) {
	m.RecordQueuePublish(queueName)
}

func (m *MockCollector) RecordQueueDelivery(queueName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if qm, ok := m.queueMetrics[queueName]; ok {
		qm.MessageCount.Add(-1)
		qm.UnackedCount.Add(1)
	}
}

func (m *MockCollector) RecordQueueAck(queueName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if qm, ok := m.queueMetrics[queueName]; ok {
		qm.UnackedCount.Add(-1)
		qm.AckCount.Add(1)
	}
}

func (m *MockCollector) RecordQueueNack(queueName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if qm, ok := m.queueMetrics[queueName]; ok {
		qm.UnackedCount.Add(-1)
	}
}

func (m *MockCollector) SetQueueDepth(queueName string, depth int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if qm, ok := m.queueMetrics[queueName]; ok {
		qm.MessageCount.Store(depth)
	}
}

func (m *MockCollector) RecordConsumerAdded(queueName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.queueMetrics[queueName]; !ok {
		m.queueMetrics[queueName] = &QueueMetrics{
			Name:         queueName,
			MessageRate:  NewRateTracker(5*time.Minute, 60),
			DeliveryRate: NewRateTracker(5*time.Minute, 60),
			AckRate:      NewRateTracker(5*time.Minute, 60),
			CreatedAt:    time.Now(),
		}
	}
	m.queueMetrics[queueName].ConsumerCount.Add(1)
}

func (m *MockCollector) RecordConsumerRemoved(queueName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if qm, ok := m.queueMetrics[queueName]; ok {
		qm.ConsumerCount.Add(-1)
	}
}

func (m *MockCollector) GetQueueMetrics(queueName string) *QueueMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.queueMetrics[queueName]
}

func (m *MockCollector) GetAllQueueMetrics() []*QueueMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*QueueMetrics, 0, len(m.queueMetrics))
	for _, qm := range m.queueMetrics {
		result = append(result, qm)
	}
	return result
}

func (m *MockCollector) RemoveQueue(queueName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.queueMetrics, queueName)
}

// Broker-level metrics
func (m *MockCollector) RecordConnection() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.brokerMetrics.connectionCount++
}

func (m *MockCollector) RecordConnectionClose() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.brokerMetrics.connectionCount--
}

func (m *MockCollector) RecordChannelOpen() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.brokerMetrics.channelCount++
}

func (m *MockCollector) RecordChannelClose() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.brokerMetrics.channelCount--
}

func (m *MockCollector) GetBrokerMetrics() *BrokerMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.brokerMetrics
}

// Time series (mock returns empty slices)
func (m *MockCollector) GetPublishRateTimeSeries(duration time.Duration) []Sample {
	return []Sample{}
}

func (m *MockCollector) GetDeliveryRateTimeSeries(duration time.Duration) []Sample {
	return []Sample{}
}

func (m *MockCollector) GetConnectionRateTimeSeries(duration time.Duration) []Sample {
	return []Sample{}
}

// Utility
func (m *MockCollector) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exchangeMetrics = make(map[string]*ExchangeMetrics)
	m.queueMetrics = make(map[string]*QueueMetrics)
}

func (m *MockCollector) IsEnabled() bool {
	return m.enabled
}
