package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Collector is the central metrics aggregation point for the broker.
// It tracks exchange, queue, and broker-level metrics using RateTrackers.
type Collector struct {
	exchangeMetrics sync.Map
	queueMetrics    sync.Map

	// Broker-wide rate metrics
	totalPublishes    *RateTracker
	totalDeliveries   *RateTracker
	totalAcks         *RateTracker
	totalNacks        *RateTracker
	connectionRate    *RateTracker
	channelRate       *RateTracker
	totalReadyDepth   *RateTracker
	totalUnackedDepth *RateTracker
	totalDepth        *RateTracker

	messageCount    atomic.Int64
	consumerCount   atomic.Int64
	connectionCount atomic.Int64
	channelCount    atomic.Int64
	queueCount      atomic.Int64
	exchangeCount   atomic.Int64
	readyCount      atomic.Int64
	unackedCount    atomic.Int64
	totalAckCount   atomic.Int64
	totalNackCount  atomic.Int64

	config *Config
}

// ExchangeMetrics tracks statistics for a single exchange
type ExchangeMetrics struct {
	Name          string
	Type          string // direct, fanout, topic, headers
	PublishRate   *RateTracker
	PublishCount  atomic.Int64
	DeliveryRate  *RateTracker // Messages delivered (routed) per second
	DeliveryCount atomic.Int64 // Total messages delivered (routed)
	CreatedAt     time.Time

	mu sync.RWMutex
}

// QueueMetrics tracks statistics for a single queue
type QueueMetrics struct {
	Name          string
	MessageRate   *RateTracker // Messages enqueued per second
	DeliveryRate  *RateTracker // Messages delivered per second
	AckRate       *RateTracker // ACKs per second
	MessageCount  atomic.Int64 // Current queue depth
	UnackedCount  atomic.Int64 // Current unacknowledged messages
	ConsumerCount atomic.Int64 // Current consumer count
	AckCount      atomic.Int64 // Cumulative ACK count (for rate calculation)
	CreatedAt     time.Time

	mu sync.RWMutex
}

// Config holds configuration for metrics collection
type Config struct {
	Enabled         bool          // Enable/disable metrics collection
	WindowSize      time.Duration // Time window for rate calculations (e.g., 5 minutes)
	MaxSamples      int           // Maximum samples to keep in ring buffer (e.g., 60)
	SamplesInterval uint8         // Interval between samples (e.g., 5 seconds)
}

// DefaultConfig returns sensible defaults for metrics collection
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		WindowSize:      5 * time.Minute,
		MaxSamples:      60, // One sample per 5 seconds for 5 minutes
		SamplesInterval: 5,
	}
}

// NewCollector creates a new metrics collector with the given configuration
func NewCollector(config *Config) *Collector {
	if config == nil {
		config = DefaultConfig()
	}

	return &Collector{
		totalPublishes:  NewRateTracker(config.WindowSize, config.MaxSamples),
		totalDeliveries: NewRateTracker(config.WindowSize, config.MaxSamples),
		totalAcks:       NewRateTracker(config.WindowSize, config.MaxSamples),
		totalNacks:      NewRateTracker(config.WindowSize, config.MaxSamples),
		connectionRate:  NewRateTracker(config.WindowSize, config.MaxSamples),
		channelRate:     NewRateTracker(config.WindowSize, config.MaxSamples),

		totalReadyDepth:   NewRateTracker(config.WindowSize, config.MaxSamples),
		totalUnackedDepth: NewRateTracker(config.WindowSize, config.MaxSamples),
		totalDepth:        NewRateTracker(config.WindowSize, config.MaxSamples),

		config: config,
	}
}

// ========================================
// Exchange Metrics
// ========================================

// RecordExchangePublish records a message publish to an exchange
func (c *Collector) RecordExchangePublish(exchangeName, exchangeType string) {
	if !c.config.Enabled {
		return
	}

	em := c.getOrCreateExchangeMetrics(exchangeName, exchangeType)
	em.PublishCount.Add(1)
	c.messageCount.Add(1)
}

// RecordExchangeDelivery records a message routed from an exchange
func (c *Collector) RecordExchangeDelivery(exchangeName string) {
	if !c.config.Enabled {
		return
	}

	em := c.getOrCreateExchangeMetrics(exchangeName, "")
	em.DeliveryCount.Add(1)
}

// GetExchangeMetrics retrieves metrics for a specific exchange
func (c *Collector) GetExchangeMetrics(exchangeName string) *ExchangeMetrics {
	if value, ok := c.exchangeMetrics.Load(exchangeName); ok {
		return value.(*ExchangeMetrics)
	}
	return nil
}

// GetAllExchangeMetrics returns metrics for all exchanges
func (c *Collector) GetAllExchangeMetrics() []*ExchangeMetrics {
	result := make([]*ExchangeMetrics, 0)
	c.exchangeMetrics.Range(func(key, value interface{}) bool {
		result = append(result, value.(*ExchangeMetrics))
		return true
	})
	return result
}

// getOrCreateExchangeMetrics gets existing or creates new exchange metrics
func (c *Collector) getOrCreateExchangeMetrics(name, exchangeType string) *ExchangeMetrics {
	if value, ok := c.exchangeMetrics.Load(name); ok {
		return value.(*ExchangeMetrics)
	}

	em := &ExchangeMetrics{
		Name:         name,
		Type:         exchangeType,
		PublishRate:  NewRateTracker(c.config.WindowSize, c.config.MaxSamples),
		DeliveryRate: NewRateTracker(c.config.WindowSize, c.config.MaxSamples),
		CreatedAt:    time.Now(),
	}

	actual, _ := c.exchangeMetrics.LoadOrStore(name, em)
	return actual.(*ExchangeMetrics)
}

// RemoveExchange removes metrics tracking for an exchange
func (c *Collector) RemoveExchange(exchangeName string) {
	c.exchangeMetrics.Delete(exchangeName)
	c.exchangeCount.Add(-1)
}

// sampleExchangeMetrics records periodic samples for an exchange
func (c *Collector) sampleExchangeMetrics(exchangeName string) {
	em := c.GetExchangeMetrics(exchangeName)
	if em == nil {
		return
	}
	em.mu.RLock()
	defer em.mu.RUnlock()
	em.PublishRate.Record(em.PublishCount.Load())
	em.DeliveryRate.Record(em.DeliveryCount.Load())
}

// ========================================
// Queue Metrics
// ========================================

// RecordQueuePublish records a message enqueued to a queue
// Called on EVERY enqueue
func (c *Collector) RecordQueuePublish(queueName string) {
	if !c.config.Enabled {
		return
	}

	qm := c.getOrCreateQueueMetrics(queueName)
	qm.MessageCount.Add(1)
	c.readyCount.Add(1)
}

// RecordQueueRequeue records a message requeued to a queue
// (alias for RecordQueuePublish for clarity)
func (c *Collector) RecordQueueRequeue(queueName string) {
	c.RecordQueuePublish(queueName)
}

// RecordQueueDelivery records a message delivered from a queue
func (c *Collector) RecordQueueDelivery(queueName string, autoAck bool) {
	if !c.config.Enabled {
		return
	}

	qm := c.getOrCreateQueueMetrics(queueName)
	qm.MessageCount.Add(-1)
	c.readyCount.Add(-1)

	if !autoAck {
		qm.UnackedCount.Add(1)
		c.unackedCount.Add(1)
	} else {
		qm.AckCount.Add(1)
		c.messageCount.Add(-1)
		c.totalAckCount.Add(1)
	}
}

// RecordQueueAck records a message acknowledgment
func (c *Collector) RecordQueueAck(queueName string) {
	if !c.config.Enabled {
		return
	}

	qm := c.getOrCreateQueueMetrics(queueName)
	qm.UnackedCount.Add(-1)
	qm.AckCount.Add(1)
	c.messageCount.Add(-1)
	c.totalAckCount.Add(1)
	c.unackedCount.Add(-1)

}

// RecordQueueNack records a message negative acknowledgment
func (c *Collector) RecordQueueNack(queueName string) {
	if !c.config.Enabled {
		return
	}

	qm := c.getOrCreateQueueMetrics(queueName)
	qm.UnackedCount.Add(-1)
	c.totalNackCount.Add(1)
}

// SetQueueDepth explicitly sets the queue depth (for periodic updates)
func (c *Collector) SetQueueDepth(queueName string, depth int64) {
	if !c.config.Enabled {
		return
	}

	qm := c.getOrCreateQueueMetrics(queueName)
	qm.MessageCount.Store(depth)
}

// RecordConsumerAdded records a new consumer on the queue
func (c *Collector) RecordConsumerAdded(queueName string) {
	if !c.config.Enabled {
		return
	}

	qm := c.getOrCreateQueueMetrics(queueName)
	qm.ConsumerCount.Add(1)
	c.consumerCount.Add(1)
}

// RecordConsumerRemoved records a consumer removal from the queue
func (c *Collector) RecordConsumerRemoved(queueName string) {
	if !c.config.Enabled {
		return
	}

	qm := c.getOrCreateQueueMetrics(queueName)
	qm.ConsumerCount.Add(-1)
	c.consumerCount.Add(-1)
}

// GetQueueMetrics retrieves metrics for a specific queue
func (c *Collector) GetQueueMetrics(queueName string) *QueueMetrics {
	if value, ok := c.queueMetrics.Load(queueName); ok {
		return value.(*QueueMetrics)
	}
	return nil
}

// GetAllQueueMetrics returns metrics for all queues
func (c *Collector) GetAllQueueMetrics() []*QueueMetrics {
	result := make([]*QueueMetrics, 0)
	c.queueMetrics.Range(func(key, value interface{}) bool {
		result = append(result, value.(*QueueMetrics))
		return true
	})
	return result
}

// getOrCreateQueueMetrics gets existing or creates new queue metrics
func (c *Collector) getOrCreateQueueMetrics(name string) *QueueMetrics {
	if value, ok := c.queueMetrics.Load(name); ok {
		return value.(*QueueMetrics)
	}

	qm := &QueueMetrics{
		Name:         name,
		MessageRate:  NewRateTracker(c.config.WindowSize, c.config.MaxSamples),
		DeliveryRate: NewRateTracker(c.config.WindowSize, c.config.MaxSamples),
		AckRate:      NewRateTracker(c.config.WindowSize, c.config.MaxSamples),
		CreatedAt:    time.Now(),
	}

	actual, _ := c.queueMetrics.LoadOrStore(name, qm)
	return actual.(*QueueMetrics)
}

// RemoveQueue removes metrics tracking for a queue
func (c *Collector) RemoveQueue(queueName string) {
	if qm := c.GetQueueMetrics(queueName); qm != nil {
		c.messageCount.Add(-qm.MessageCount.Load())
		c.consumerCount.Add(-qm.ConsumerCount.Load())
	}
	c.queueMetrics.Delete(queueName)
	c.queueCount.Add(-1)
}

func (c *Collector) sampleQueueMetrics(queueName string) {
	qm := c.GetQueueMetrics(queueName)
	if qm == nil {
		return
	}
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	qm.MessageRate.Record(qm.MessageCount.Load())
	qm.DeliveryRate.Record(qm.MessageCount.Load())
	qm.AckRate.Record(qm.MessageCount.Load())
}

// ========================================
// Broker-Level Metrics
// ========================================

// RecordConnection records a new connection
func (c *Collector) RecordConnection() {
	if !c.config.Enabled {
		return
	}

	c.connectionCount.Add(1)
}

// RecordConnectionClose records a connection closing
func (c *Collector) RecordConnectionClose() {
	if !c.config.Enabled {
		return
	}

	c.connectionCount.Add(-1)
}

// RecordChannelOpen records a new channel opening
func (c *Collector) RecordChannelOpen() {
	if !c.config.Enabled {
		return
	}

	c.channelCount.Add(1)
}

// RecordChannelClose records a channel closing
func (c *Collector) RecordChannelClose() {
	if !c.config.Enabled {
		return
	}

	c.channelCount.Add(-1)
}

// GetBrokerMetrics returns current broker-level metrics
func (c *Collector) GetBrokerMetrics() *BrokerMetrics {
	return &BrokerMetrics{
		totalPublishes:    c.totalPublishes,
		totalDeliveries:   c.totalDeliveries,
		totalAcks:         c.totalAcks,
		totalNacks:        c.totalNacks,
		connectionRate:    c.connectionRate,
		channelRate:       c.channelRate,
		totalReadyDepth:   c.totalReadyDepth,
		totalUnackedDepth: c.totalUnackedDepth,
		totalDepth:        c.totalDepth,
		messageCount:      c.messageCount.Load(),
		consumerCount:     c.consumerCount.Load(),
		connectionCount:   c.connectionCount.Load(),
		channelCount:      c.channelCount.Load(),
		queueCount:        c.queueCount.Load(),
		exchangeCount:     c.exchangeCount.Load(),
		readyCount:        c.readyCount.Load(),
		unackedCount:      c.unackedCount.Load(),
	}
}

// StartPeriodicSampling starts a background ticker that samples metrics
// This should be called ONCE when the broker starts
// Recommended interval: 5 seconds (gives 60 samples over 5 minutes)
func (c *Collector) StartPeriodicSampling() {
	interval := time.Duration(c.config.SamplesInterval) * time.Second
	if !c.config.Enabled {
		return
	}

	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			c.sampleBrokerMetrics()
		}
	}()
}

// sampleBrokerMetrics samples all current counters and records to time-series
func (c *Collector) sampleBrokerMetrics() {
	// Sample broker-wide depths
	ready := c.readyCount.Load()
	unacked := c.unackedCount.Load()
	c.totalReadyDepth.Record(ready)
	c.totalUnackedDepth.Record(unacked)
	c.totalDepth.Record(ready + unacked)

	// Sample broker-wide cumulative counters
	c.totalPublishes.Record(c.messageCount.Load())
	c.totalDeliveries.Record(c.messageCount.Load())
	c.totalAcks.Record(c.totalAckCount.Load())
	c.totalNacks.Record(c.totalNackCount.Load())
	c.connectionRate.Record(c.connectionCount.Load())
	c.channelRate.Record(c.channelCount.Load())

	// Sample each queue (for per-queue charts)
	c.queueMetrics.Range(func(key, value interface{}) bool {
		queueName := key.(string)
		c.sampleQueueMetrics(queueName)
		return true
	})

	// Sample each exchange (for per-exchange charts)
	c.exchangeMetrics.Range(func(key, value interface{}) bool {
		exchangeName := key.(string)
		c.sampleExchangeMetrics(exchangeName)
		return true
	})

}

// ========================================
// SNAPSHOTS & TIME-SERIES ACCESS
// ========================================

type BrokerMetrics struct {
	totalPublishes    *RateTracker
	totalDeliveries   *RateTracker
	totalAcks         *RateTracker
	totalNacks        *RateTracker
	connectionRate    *RateTracker
	channelRate       *RateTracker
	totalReadyDepth   *RateTracker
	totalUnackedDepth *RateTracker
	totalDepth        *RateTracker

	messageCount    int64
	consumerCount   int64
	connectionCount int64
	channelCount    int64
	queueCount      int64
	exchangeCount   int64
	readyCount      int64
	unackedCount    int64
}

// Getters for BrokerMetrics
func (bm *BrokerMetrics) TotalPublishes() *RateTracker  { return bm.totalPublishes }
func (bm *BrokerMetrics) TotalDeliveries() *RateTracker { return bm.totalDeliveries }
func (bm *BrokerMetrics) TotalAcks() *RateTracker       { return bm.totalAcks }
func (bm *BrokerMetrics) TotalNacks() *RateTracker      { return bm.totalNacks }
func (bm *BrokerMetrics) ConnectionRate() *RateTracker  { return bm.connectionRate }
func (bm *BrokerMetrics) ChannelRate() *RateTracker     { return bm.channelRate }
func (bm *BrokerMetrics) MessageCount() int64           { return bm.messageCount }
func (bm *BrokerMetrics) ConsumerCount() int64          { return bm.consumerCount }
func (bm *BrokerMetrics) ConnectionCount() int64        { return bm.connectionCount }
func (bm *BrokerMetrics) ChannelCount() int64           { return bm.channelCount }
func (bm *BrokerMetrics) QueueCount() int64             { return bm.queueCount }
func (bm *BrokerMetrics) ExchangeCount() int64          { return bm.exchangeCount }

// BrokerSnapshot represents current broker state (for Overview page)
type BrokerSnapshot struct {
	Timestamp time.Time `json:"timestamp"`

	// Current rates (single values for display cards)
	PublishRate    float64 `json:"publish_rate"`
	DeliveryRate   float64 `json:"delivery_rate"`
	AckRate        float64 `json:"ack_rate"`
	NackRate       float64 `json:"nack_rate"`
	ConnectionRate float64 `json:"connection_rate"`
	ChannelRate    float64 `json:"channel_rate"`

	// RateTrackers (for accessing time-series samples)
	// NOTE: These are pointers, not copies (useful for charts endpoint)

	TotalReadyDepth   *RateTracker `json:"total_ready_depth"`
	TotalUnackedDepth *RateTracker `json:"total_unacked_depth"`
	TotalDepth        *RateTracker `json:"total_depth"`

	// Current gauges
	MessageCount    int64 `json:"message_count"`
	ConsumerCount   int64 `json:"consumer_count"`
	ConnectionCount int64 `json:"connection_count"`
	ChannelCount    int64 `json:"channel_count"`
	QueueCount      int64 `json:"queue_count"`
	ExchangeCount   int64 `json:"exchange_count"`
}

// GetBrokerSnapshot returns a snapshot of current broker metrics
func (c *Collector) GetBrokerSnapshot() *BrokerSnapshot {
	return c.GetBrokerMetrics().Snapshot()
}

// Snapshot creates a BrokerSnapshot from the current BrokerMetrics
func (bm *BrokerMetrics) Snapshot() *BrokerSnapshot {
	return &BrokerSnapshot{
		Timestamp:         time.Now(),
		PublishRate:       bm.totalPublishes.Rate(),
		DeliveryRate:      bm.totalDeliveries.Rate(),
		AckRate:           bm.totalAcks.Rate(),
		NackRate:          bm.totalNacks.Rate(),
		ConnectionRate:    bm.connectionRate.Rate(),
		ChannelRate:       bm.channelRate.Rate(),
		TotalReadyDepth:   bm.totalReadyDepth,
		TotalUnackedDepth: bm.totalUnackedDepth,
		TotalDepth:        bm.totalDepth,
		MessageCount:      bm.messageCount,
		ConsumerCount:     bm.consumerCount,
		ConnectionCount:   bm.connectionCount,
		ChannelCount:      bm.channelCount,
		QueueCount:        bm.queueCount,
		ExchangeCount:     bm.exchangeCount,
	}
}

// GetTimeSeries returns historical samples for a specific metric
func (c *Collector) GetPublishRateTimeSeries(duration time.Duration) []Sample {
	return c.totalPublishes.GetSamples()
}

func (c *Collector) GetDeliveryRateTimeSeries(duration time.Duration) []Sample {
	return c.totalDeliveries.GetSamples()
}

func (c *Collector) GetAckRateTimeSeries(duration time.Duration) []Sample {
	return c.totalAcks.GetSamples()
}

func (c *Collector) GetConnectionRateTimeSeries(duration time.Duration) []Sample {
	return c.connectionRate.GetSamples()
}

func (c *Collector) GetPublishRate() float64 {
	return c.totalPublishes.Rate()
}

func (c *Collector) GetDeliveryRate() float64 {
	return c.totalDeliveries.Rate()
}

func (c *Collector) GetTotalAcksRate() float64 {
	return c.totalAcks.Rate()
}

// ExchangeSnapshot returns a snapshot of exchange metrics
type ExchangeSnapshot struct {
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	PublishRate   float64   `json:"publish_rate"`
	PublishCount  int64     `json:"publish_count"`
	DeliveryRate  float64   `json:"delivery_rate"`
	DeliveryCount int64     `json:"delivery_count"`
	Uptime        float64   `json:"uptime_seconds"`
	CreatedAt     time.Time `json:"created_at"`
}

func (c *Collector) GetExchangeSnapshot(exchangeName string) *ExchangeSnapshot {
	if em := c.GetExchangeMetrics(exchangeName); em != nil {
		return em.Snapshot()
	}
	return nil
}

// Snapshot returns a snapshot of this exchange's metrics
func (em *ExchangeMetrics) Snapshot() *ExchangeSnapshot {
	em.mu.RLock()
	defer em.mu.RUnlock()

	return &ExchangeSnapshot{
		Name:          em.Name,
		Type:          em.Type,
		PublishRate:   em.PublishRate.Rate(),
		PublishCount:  em.PublishCount.Load(),
		DeliveryRate:  em.DeliveryRate.Rate(),
		DeliveryCount: em.DeliveryCount.Load(),
		Uptime:        time.Since(em.CreatedAt).Seconds(),
		CreatedAt:     em.CreatedAt,
	}
}

// QueueSnapshot returns a snapshot of queue metrics
type QueueSnapshot struct {
	Name          string    `json:"name"`
	MessageRate   float64   `json:"message_rate"`
	DeliveryRate  float64   `json:"delivery_rate"`
	AckRate       float64   `json:"ack_rate"`
	AckCount      int64     `json:"ack_count"`
	UnackedCount  int64     `json:"unacked_count"`
	MessageCount  int64     `json:"message_count"`
	ConsumerCount int64     `json:"consumer_count"`
	Uptime        float64   `json:"uptime_seconds"`
	CreatedAt     time.Time `json:"created_at"`
}

func (c *Collector) GetQueueSnapshot(queueName string) *QueueSnapshot {
	if qm := c.GetQueueMetrics(queueName); qm != nil {
		return qm.Snapshot()
	}
	return nil
}

// Snapshot returns a snapshot of this queue's metrics
func (qm *QueueMetrics) Snapshot() *QueueSnapshot {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	return &QueueSnapshot{
		Name:          qm.Name,
		MessageRate:   qm.MessageRate.Rate(),
		DeliveryRate:  qm.DeliveryRate.Rate(),
		AckRate:       qm.AckRate.Rate(),
		AckCount:      qm.AckCount.Load(),
		UnackedCount:  qm.UnackedCount.Load(),
		MessageCount:  qm.MessageCount.Load(),
		ConsumerCount: qm.ConsumerCount.Load(),
		Uptime:        time.Since(qm.CreatedAt).Seconds(),
		CreatedAt:     qm.CreatedAt,
	}
}

// ========================================
// Utility Methods
// ========================================

// Clear resets all metrics (useful for testing)
func (c *Collector) Clear() {
	c.exchangeMetrics = sync.Map{}
	c.queueMetrics = sync.Map{}
	c.totalPublishes.Clear()
	c.totalDeliveries.Clear()
	c.totalAcks.Clear()
	c.totalNacks.Clear()
	c.connectionRate.Clear()
	c.channelRate.Clear()
	c.totalReadyDepth.Clear()
	c.totalUnackedDepth.Clear()
	c.totalDepth.Clear()

	c.messageCount.Store(0)
	c.consumerCount.Store(0)
	c.connectionCount.Store(0)
	c.channelCount.Store(0)
	c.queueCount.Store(0)
	c.exchangeCount.Store(0)
}

// IsEnabled returns whether metrics collection is enabled
func (c *Collector) IsEnabled() bool {
	return c.config.Enabled
}
