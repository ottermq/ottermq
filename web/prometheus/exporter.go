package prometheus

import (
	"sync"
	"time"

	"github.com/ottermq/ottermq/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Exporter struct {
	collector *metrics.Collector
	config    *Config

	// Prometheus metrics
	publishedTotal prometheus.Counter
	deliveredTotal prometheus.Counter
	ackedTotal     prometheus.Counter
	nackedTotal    prometheus.Counter

	publishedRate prometheus.Gauge
	deliveredRate prometheus.Gauge

	messageCount    prometheus.Gauge
	queueCount      prometheus.Gauge
	connectionCount prometheus.Gauge

	publishLatency prometheus.Histogram
	deliverLatency prometheus.Histogram

	// Per-queue metrics
	queueDepth       *prometheus.GaugeVec
	queuePublishRate *prometheus.CounterVec

	// Delta tracking for counters
	// We need to track the last value we saw to calculate deltas
	// because Prometheus counters must be monotonically increasing
	lastPublished int64
	lastDelivered int64
	lastAcked     int64
	lastNacked    int64

	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

func NewExporter(collector *metrics.Collector, config *Config) *Exporter {
	e := &Exporter{
		collector: collector,
		config:    config,
		stopChan:  make(chan struct{}),
	}
	e.registerMetrics()
	return e
}

func (e *Exporter) registerMetrics() {
	e.publishedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ottermq_messages_published_total",
		Help: "Total number of messages published",
	})
	e.deliveredTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ottermq_messages_delivered_total",
		Help: "Total number of messages delivered",
	})
	e.ackedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ottermq_messages_acked_total",
		Help: "Total number of messages acknowledged",
	})
	e.nackedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ottermq_messages_nacked_total",
		Help: "Total number of messages not acknowledged",
	})

	e.publishedRate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ottermq_messages_publish_rate",
		Help: "Rate of messages published per second",
	})
	e.deliveredRate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ottermq_messages_deliver_rate",
		Help: "Rate of messages delivered per second",
	})

	e.messageCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ottermq_message_count",
		Help: "Current number of messages in the broker",
	})
	e.queueCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ottermq_queue_count",
		Help: "Current number of queues in the broker",
	})
	e.connectionCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ottermq_connection_count",
		Help: "Current number of connections to the broker",
	})

	e.publishLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ottermq_publish_latency_milliseconds",
		Help:    "Latency of message publishing in milliseconds",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
	})

	e.deliverLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ottermq_deliver_latency_milliseconds",
		Help:    "Latency of message delivery in milliseconds",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
	})

	e.queueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ottermq_queue_message_depth",
		Help: "Number of messages in the queue",
	}, []string{"queue"},
	)

	e.queuePublishRate = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ottermq_queue_publish_rate_total",
		Help: "Total number of messages published to the queue",
	}, []string{"queue"},
	)
}

func (e *Exporter) Start() {
	e.wg.Add(1)
	go e.updateLoop()
}

func (e *Exporter) Stop() {
	close(e.stopChan)
	e.wg.Wait()
}

func (e *Exporter) updateLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.update()
		case <-e.stopChan:
			return
		}
	}
}

func (e *Exporter) update() {
	snapshot := e.collector.GetBrokerSnapshot()

	// Update gauges directly (current state)
	// Gauges can be set to absolute values
	e.publishedRate.Set(float64(snapshot.PublishRate))
	e.deliveredRate.Set(float64(snapshot.DeliveryRate))
	e.messageCount.Set(float64(snapshot.MessageCount))
	e.queueCount.Set(float64(snapshot.QueueCount))
	e.connectionCount.Set(float64(snapshot.ConnectionCount))

	// Update counters by calculating deltas
	// Prometheus counters must be monotonically increasing
	// We track the delta since last update
	e.mu.Lock()
	defer e.mu.Unlock()

	// Calculate deltas from the collector's internal counters
	// The collector should expose cumulative totals
	currentPublished := snapshot.PublishedTotal // Assuming snapshot has these
	currentDelivered := snapshot.DeliveredTotal
	currentAcked := snapshot.AckedTotal
	currentNacked := snapshot.NackedTotal

	// Add the delta to Prometheus counters
	if currentPublished > e.lastPublished {
		delta := currentPublished - e.lastPublished
		e.publishedTotal.Add(float64(delta))
		e.lastPublished = currentPublished
	}

	if currentDelivered > e.lastDelivered {
		delta := currentDelivered - e.lastDelivered
		e.deliveredTotal.Add(float64(delta))
		e.lastDelivered = currentDelivered
	}

	if currentAcked > e.lastAcked {
		delta := currentAcked - e.lastAcked
		e.ackedTotal.Add(float64(delta))
		e.lastAcked = currentAcked
	}

	if currentNacked > e.lastNacked {
		delta := currentNacked - e.lastNacked
		e.nackedTotal.Add(float64(delta))
		e.lastNacked = currentNacked
	}

	// Update per-queue metrics
	for _, qm := range e.collector.GetAllQueueMetrics() {
		e.queueDepth.WithLabelValues(qm.Name).Set(float64(qm.Depth.Load()))

		// For queue publish rate, we need deltas too
		// This assumes QueueMetrics has a cumulative PublishedTotal field
		// e.queuePublishRate.WithLabelValues(qm.Name).Add(delta)
	}
}
