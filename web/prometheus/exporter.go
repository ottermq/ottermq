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

	// Broker-wide counters (monotonically increasing)
	publishedTotal prometheus.Counter
	deliveredTotal prometheus.Counter
	ackedTotal     prometheus.Counter
	nackedTotal    prometheus.Counter

	// Broker-wide gauges (current state)
	publishedRate   prometheus.Gauge
	deliveredRate   prometheus.Gauge
	messageCount    prometheus.Gauge
	queueCount      prometheus.Gauge
	connectionCount prometheus.Gauge
	channelCount    prometheus.Gauge
	consumerCount   prometheus.Gauge

	// Per-queue gauges
	queueDepth        *prometheus.GaugeVec
	queueMessageRate  *prometheus.GaugeVec
	queueDeliveryRate *prometheus.GaugeVec
	queueAckRate      *prometheus.GaugeVec
	queueConsumers    *prometheus.GaugeVec
	queueUnacked      *prometheus.GaugeVec

	// Per-exchange gauges and counters
	exchangePublishRate    *prometheus.GaugeVec
	exchangeDeliveryRate   *prometheus.GaugeVec
	exchangePublishedTotal *prometheus.CounterVec

	// Delta tracking for broker-wide counters
	lastPublished int64
	lastDelivered int64
	lastAcked     int64
	lastNacked    int64

	// Delta tracking for per-exchange counters (keyed by exchange name)
	lastExchangePublished map[string]int64

	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

func NewExporter(collector *metrics.Collector, config *Config) *Exporter {
	e := &Exporter{
		collector:             collector,
		config:                config,
		lastExchangePublished: make(map[string]int64),
		stopChan:              make(chan struct{}),
	}
	e.registerMetrics()
	return e
}

func (e *Exporter) registerMetrics() {
	// Broker-wide counters
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

	// Broker-wide gauges
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
		Help: "Current number of queues",
	})
	e.connectionCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ottermq_connection_count",
		Help: "Current number of connections",
	})
	e.channelCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ottermq_channel_count",
		Help: "Current number of open channels",
	})
	e.consumerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ottermq_consumer_count",
		Help: "Current number of active consumers",
	})

	// Per-queue gauges
	e.queueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ottermq_queue_depth",
		Help: "Number of ready messages in the queue",
	}, []string{"queue"})

	e.queueMessageRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ottermq_queue_message_rate",
		Help: "Rate of messages enqueued per second",
	}, []string{"queue"})

	e.queueDeliveryRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ottermq_queue_delivery_rate",
		Help: "Rate of messages delivered per second from the queue",
	}, []string{"queue"})

	e.queueAckRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ottermq_queue_ack_rate",
		Help: "Rate of message acknowledgements per second",
	}, []string{"queue"})

	e.queueConsumers = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ottermq_queue_consumers",
		Help: "Number of consumers on the queue",
	}, []string{"queue"})

	e.queueUnacked = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ottermq_queue_unacked",
		Help: "Number of unacknowledged messages in the queue",
	}, []string{"queue"})

	// Per-exchange metrics
	e.exchangePublishRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ottermq_exchange_publish_rate",
		Help: "Rate of messages published to the exchange per second",
	}, []string{"exchange", "type"})

	e.exchangeDeliveryRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ottermq_exchange_delivery_rate",
		Help: "Rate of messages routed from the exchange per second",
	}, []string{"exchange", "type"})

	e.exchangePublishedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ottermq_exchange_published_total",
		Help: "Total number of messages published to the exchange",
	}, []string{"exchange", "type"})
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
	e.mu.Lock()
	defer e.mu.Unlock()

	e.updateBroker()
	e.updateQueues()
	e.updateExchanges()
}

func (e *Exporter) updateBroker() {
	snapshot := e.collector.GetBrokerSnapshot()

	e.publishedRate.Set(float64(snapshot.PublishRate))
	e.deliveredRate.Set(float64(snapshot.DeliveryRate))
	e.messageCount.Set(float64(snapshot.MessageCount))
	e.queueCount.Set(float64(snapshot.QueueCount))
	e.connectionCount.Set(float64(snapshot.ConnectionCount))
	e.channelCount.Set(float64(snapshot.ChannelCount))
	e.consumerCount.Set(float64(snapshot.ConsumerCount))

	if delta := snapshot.PublishedTotal - e.lastPublished; delta > 0 {
		e.publishedTotal.Add(float64(delta))
		e.lastPublished = snapshot.PublishedTotal
	}
	if delta := snapshot.DeliveredTotal - e.lastDelivered; delta > 0 {
		e.deliveredTotal.Add(float64(delta))
		e.lastDelivered = snapshot.DeliveredTotal
	}
	if delta := snapshot.AckedTotal - e.lastAcked; delta > 0 {
		e.ackedTotal.Add(float64(delta))
		e.lastAcked = snapshot.AckedTotal
	}
	if delta := snapshot.NackedTotal - e.lastNacked; delta > 0 {
		e.nackedTotal.Add(float64(delta))
		e.lastNacked = snapshot.NackedTotal
	}
}

func (e *Exporter) updateQueues() {
	for _, qm := range e.collector.GetAllQueueMetrics() {
		name := qm.Name
		e.queueDepth.WithLabelValues(name).Set(float64(qm.Depth.Load()))
		e.queueMessageRate.WithLabelValues(name).Set(qm.PublishRate.Rate())
		e.queueDeliveryRate.WithLabelValues(name).Set(qm.DeliveryRate.Rate())
		e.queueAckRate.WithLabelValues(name).Set(qm.AckRate.Rate())
		e.queueConsumers.WithLabelValues(name).Set(float64(qm.ConsumerCount.Load()))
		e.queueUnacked.WithLabelValues(name).Set(float64(qm.UnackedCount.Load()))
	}
}

func (e *Exporter) updateExchanges() {
	seen := make(map[string]struct{})

	for _, em := range e.collector.GetAllExchangeMetrics() {
		name := em.Name
		typ := em.Type
		seen[name] = struct{}{}

		e.exchangePublishRate.WithLabelValues(name, typ).Set(em.PublishRate.Rate())
		e.exchangeDeliveryRate.WithLabelValues(name, typ).Set(em.DeliveryRate.Rate())

		current := em.PublishCount.Load()
		if delta := current - e.lastExchangePublished[name]; delta > 0 {
			e.exchangePublishedTotal.WithLabelValues(name, typ).Add(float64(delta))
			e.lastExchangePublished[name] = current
		}
	}

	// Clean up stale delta entries for deleted exchanges
	for name := range e.lastExchangePublished {
		if _, ok := seen[name]; !ok {
			delete(e.lastExchangePublished, name)
		}
	}
}
