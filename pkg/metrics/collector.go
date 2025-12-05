package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

type Collector struct {
	exchangeMetrics sync.Map // map[string]*ExchangeMetrics

	queueMetrics sync.Map // map[string]*QueueMetrics

	// Broker-wide metrics

	totalPublishes  *RateTracker
	totalDeliveries *RateTracker
	connections     *RateTracker
	channels        *RateTracker

	// Configuration

	enabled    bool
	windowSize time.Duration
	maxSamples int
}

type ExchangeMetrics struct {
	PublishRate  *RateTracker
	DeliverRate  *RateTracker
	PublishCount atomic.Int64
	DeliverCount atomic.Int64
}

type QueueMetrics struct {
	MessageRate   *RateTracker
	ConsumerRate  *RateTracker
	MessageCount  atomic.Int64
	ConsumerCount atomic.Int64
}
