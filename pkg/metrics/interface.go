package metrics

import "time"

// MetricsCollector is the interface for metrics collection in OtterMQ.
// This interface allows for easy mocking in tests.
type MetricsCollector interface {
	// Exchange metrics
	RecordExchangePublish(exchangeName, exchangeType string)
	RecordExchangeDelivery(exchangeName string)
	GetExchangeMetrics(exchangeName string) *ExchangeMetrics
	GetAllExchangeMetrics() []*ExchangeMetrics
	RemoveExchange(exchangeName string)

	// Queue metrics
	RecordQueuePublish(queueName string)
	RecordQueueRequeue(queueName string)
	RecordQueueDelivery(queueName string, autoAck bool)
	RecordQueueAck(queueName string)
	RecordQueueNack(queueName string)
	SetQueueDepth(queueName string, depth int64)
	RecordConsumerAdded(queueName string)
	RecordConsumerRemoved(queueName string)
	GetQueueMetrics(queueName string) *QueueMetrics
	GetAllQueueMetrics() []*QueueMetrics
	RemoveQueue(queueName string)

	// Broker-level metrics
	RecordConnection()
	RecordConnectionClose()
	RecordChannelOpen()
	RecordChannelClose()
	GetBrokerMetrics() *BrokerMetrics

	GetBrokerSnapshot() *BrokerSnapshot
	GetExchangeSnapshot(exchangeName string) *ExchangeSnapshot
	GetQueueSnapshot(queueName string) *QueueSnapshot

	// Time series
	GetPublishRateTimeSeries(duration time.Duration) []Sample
	GetDeliveryAutoAckRateTimeSeries(duration time.Duration) []Sample
	GetDeliveryManualAckRateTimeSeries(duration time.Duration) []Sample
	GetAckRateTimeSeries(duration time.Duration) []Sample
	GetConnectionRateTimeSeries(duration time.Duration) []Sample

	// Utility
	Clear()
	IsEnabled() bool
	// StartPeriodicSampling starts the periodic sampling of metrics at the given interval
	StartPeriodicSampling()
}

// Ensure Collector implements MetricsCollector
var _ MetricsCollector = (*Collector)(nil)
