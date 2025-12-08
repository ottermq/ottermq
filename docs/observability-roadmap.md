# OtterMQ Observability & Real-Time Metrics Roadmap

## Overview

This document outlines the implementation plan for comprehensive observability in OtterMQ, enabling real-time charts in the management UI, Prometheus/Grafana integration, and distributed tracing capabilities.

**Key Principle:** Build in layers - start with internal metrics for UI charts, then add external monitoring integrations.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    OtterMQ Broker                           │
│  ┌──────────────────────────────────────────────────────┐  │
│  │           Message Flow Instrumentation                │  │
│  │  • Publish events    • Delivery events                │  │
│  │  • ACK/NACK events   • Connection events              │  │
│  └────────────────┬─────────────────────────────────────┘  │
│                   │                                          │
│  ┌────────────────▼─────────────────────────────────────┐  │
│  │         pkg/metrics/Collector                         │  │
│  │  • Rate tracking (messages/sec, connections/sec)      │  │
│  │  • Gauges (queue depths, consumer counts)             │  │
│  │  • Histograms (latencies, message sizes)              │  │
│  │  • Time-series ring buffers (5-60 min history)        │  │
│  └──────────────┬─────────────────┬─────────────────────┘  │
│                 │                 │                          │
└─────────────────┼─────────────────┼──────────────────────────┘
                  │                 │
        ┌─────────▼────────┐  ┌────▼──────────────┐
        │  Internal REST   │  │ Prometheus        │
        │  /api/metrics/*  │  │ /metrics          │
        │                  │  │ (future)          │
        └─────────┬────────┘  └────┬──────────────┘
                  │                │
        ┌─────────▼────────┐  ┌────▼──────────────┐
        │  Management UI   │  │  Grafana          │
        │  (Chart.js)      │  │  Dashboards       │
        └──────────────────┘  └───────────────────┘
```

## Phase 1: Internal Metrics Collection (Foundation) ⭐

**Timeline:** 1-2 weeks  
**Priority:** HIGH - Enables real-time UI charts  
**Dependencies:** None

### Goals

1. Provide time-series data for real-time charts in management UI
2. Track message rates, queue depths, and connection statistics
3. Store recent history (5-60 minutes) in memory
4. Calculate rates without external dependencies
5. Foundation for future Prometheus integration

### Components to Build

#### 1.1 Core Metrics Package

**`pkg/metrics/collector.go`**

```go
package metrics

import (
    "sync"
    "sync/atomic"
    "time"
)

// Collector is the central metrics aggregation point
type Collector struct {
    // Rate metrics (events per second)
    publishRate      *RateTracker
    deliveryRate     *RateTracker
    ackRate          *RateTracker
    nackRate         *RateTracker
    rejectRate       *RateTracker
    connectionRate   *RateTracker
    
    // Gauge metrics (current values)
    messageCount     atomic.Int64  // Total messages in all queues
    consumerCount    atomic.Int64  // Total active consumers
    connectionCount  atomic.Int64  // Current connections
    channelCount     atomic.Int64  // Current channels
    queueCount       atomic.Int64  // Total queues
    exchangeCount    atomic.Int64  // Total exchanges
    
    // Histogram metrics (distributions)
    publishLatency   *Histogram
    deliveryLatency  *Histogram
    messageSize      *Histogram
    
    // Per-queue metrics
    queueMetrics     map[string]*QueueMetrics
    queueMetricsMu   sync.RWMutex
    
    // Per-exchange metrics
    exchangeMetrics  map[string]*ExchangeMetrics
    exchangeMetricsMu sync.RWMutex
    
    // Configuration
    config           *Config
}

type Config struct {
    // Time-series retention
    RetentionWindow  time.Duration // e.g., 5 minutes
    SampleInterval   time.Duration // e.g., 5 seconds
    
    // Ring buffer sizes
    RateBufferSize   int           // Number of samples to keep
    
    // Histogram buckets
    LatencyBuckets   []float64     // Microsecond buckets
    SizeBuckets      []float64     // Byte buckets
}

func NewCollector(config *Config) *Collector {
    if config == nil {
        config = DefaultConfig()
    }
    
    return &Collector{
        publishRate:     NewRateTracker(config),
        deliveryRate:    NewRateTracker(config),
        ackRate:         NewRateTracker(config),
        nackRate:        NewRateTracker(config),
        rejectRate:      NewRateTracker(config),
        connectionRate:  NewRateTracker(config),
        publishLatency:  NewHistogram(config.LatencyBuckets),
        deliveryLatency: NewHistogram(config.LatencyBuckets),
        messageSize:     NewHistogram(config.SizeBuckets),
        queueMetrics:    make(map[string]*QueueMetrics),
        exchangeMetrics: make(map[string]*ExchangeMetrics),
        config:          config,
    }
}

func DefaultConfig() *Config {
    return &Config{
        RetentionWindow: 5 * time.Minute,
        SampleInterval:  5 * time.Second,
        RateBufferSize:  60, // 5 minutes at 5s intervals
        LatencyBuckets: []float64{
            100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000,
        },
        SizeBuckets: []float64{
            1024, 10240, 102400, 1024000, 10240000, 104857600,
        },
    }
}

// Recording methods
func (c *Collector) RecordPublish(size int) {
    c.publishRate.Record(1)
    c.messageSize.Record(float64(size))
}

func (c *Collector) RecordDelivery(latencyMicros int64) {
    c.deliveryRate.Record(1)
    c.deliveryLatency.Record(float64(latencyMicros))
}

func (c *Collector) RecordAck() {
    c.ackRate.Record(1)
}

func (c *Collector) RecordNack() {
    c.nackRate.Record(1)
}

func (c *Collector) RecordReject() {
    c.rejectRate.Record(1)
}

func (c *Collector) RecordConnection() {
    c.connectionRate.Record(1)
    c.connectionCount.Add(1)
}

func (c *Collector) RecordConnectionClose() {
    c.connectionCount.Add(-1)
}

func (c *Collector) RecordChannelOpen() {
    c.channelCount.Add(1)
}

func (c *Collector) RecordChannelClose() {
    c.channelCount.Add(-1)
}

// Gauge updates
func (c *Collector) SetMessageCount(count int64) {
    c.messageCount.Store(count)
}

func (c *Collector) SetConsumerCount(count int64) {
    c.consumerCount.Store(count)
}

func (c *Collector) SetQueueCount(count int64) {
    c.queueCount.Store(count)
}

func (c *Collector) SetExchangeCount(count int64) {
    c.exchangeCount.Store(count)
}

// Snapshot for API responses
func (c *Collector) Snapshot() *Snapshot {
    return &Snapshot{
        Timestamp:       time.Now(),
        
        // Rates (current)
        PublishRate:     c.publishRate.CurrentRate(),
        DeliveryRate:    c.deliveryRate.CurrentRate(),
        AckRate:         c.ackRate.CurrentRate(),
        NackRate:        c.nackRate.CurrentRate(),
        RejectRate:      c.rejectRate.CurrentRate(),
        ConnectionRate:  c.connectionRate.CurrentRate(),
        
        // Gauges
        MessageCount:    c.messageCount.Load(),
        ConsumerCount:   c.consumerCount.Load(),
        ConnectionCount: c.connectionCount.Load(),
        ChannelCount:    c.channelCount.Load(),
        QueueCount:      c.queueCount.Load(),
        ExchangeCount:   c.exchangeCount.Load(),
        
        // Histograms (percentiles)
        PublishLatencyP50:  c.publishLatency.Percentile(0.50),
        PublishLatencyP95:  c.publishLatency.Percentile(0.95),
        PublishLatencyP99:  c.publishLatency.Percentile(0.99),
        DeliveryLatencyP50: c.deliveryLatency.Percentile(0.50),
        DeliveryLatencyP95: c.deliveryLatency.Percentile(0.95),
        DeliveryLatencyP99: c.deliveryLatency.Percentile(0.99),
        MessageSizeP50:     c.messageSize.Percentile(0.50),
        MessageSizeP95:     c.messageSize.Percentile(0.95),
        MessageSizeP99:     c.messageSize.Percentile(0.99),
    }
}

type Snapshot struct {
    Timestamp          time.Time `json:"timestamp"`
    
    // Rates (messages/sec)
    PublishRate        float64   `json:"publish_rate"`
    DeliveryRate       float64   `json:"delivery_rate"`
    AckRate            float64   `json:"ack_rate"`
    NackRate           float64   `json:"nack_rate"`
    RejectRate         float64   `json:"reject_rate"`
    ConnectionRate     float64   `json:"connection_rate"`
    
    // Gauges
    MessageCount       int64     `json:"message_count"`
    ConsumerCount      int64     `json:"consumer_count"`
    ConnectionCount    int64     `json:"connection_count"`
    ChannelCount       int64     `json:"channel_count"`
    QueueCount         int64     `json:"queue_count"`
    ExchangeCount      int64     `json:"exchange_count"`
    
    // Latency percentiles (microseconds)
    PublishLatencyP50  float64   `json:"publish_latency_p50"`
    PublishLatencyP95  float64   `json:"publish_latency_p95"`
    PublishLatencyP99  float64   `json:"publish_latency_p99"`
    DeliveryLatencyP50 float64   `json:"delivery_latency_p50"`
    DeliveryLatencyP95 float64   `json:"delivery_latency_p95"`
    DeliveryLatencyP99 float64   `json:"delivery_latency_p99"`
    
    // Message size percentiles (bytes)
    MessageSizeP50     float64   `json:"message_size_p50"`
    MessageSizeP95     float64   `json:"message_size_p95"`
    MessageSizeP99     float64   `json:"message_size_p99"`
}
```

#### 1.2 Rate Tracker (Time-Series Data)

**`pkg/metrics/rate_tracker.go`**

```go
package metrics

import (
    "sync"
    "time"
)

// RateTracker tracks events over time and calculates rates
type RateTracker struct {
    samples    []Sample
    head       int
    count      int
    capacity   int
    
    interval   time.Duration
    
    currentSum atomic.Int64  // Sum in current interval
    ticker     *time.Ticker
    stopChan   chan struct{}
    
    mu         sync.RWMutex
}

type Sample struct {
    Timestamp time.Time `json:"timestamp"`
    Value     float64   `json:"value"`
}

func NewRateTracker(config *Config) *RateTracker {
    rt := &RateTracker{
        samples:  make([]Sample, config.RateBufferSize),
        capacity: config.RateBufferSize,
        interval: config.SampleInterval,
        stopChan: make(chan struct{}),
    }
    
    rt.ticker = time.NewTicker(config.SampleInterval)
    go rt.samplingLoop()
    
    return rt
}

func (rt *RateTracker) Record(count int64) {
    rt.currentSum.Add(count)
}

func (rt *RateTracker) samplingLoop() {
    for {
        select {
        case <-rt.ticker.C:
            rt.sample()
        case <-rt.stopChan:
            return
        }
    }
}

func (rt *RateTracker) sample() {
    // Get current sum and reset
    sum := rt.currentSum.Swap(0)
    
    // Calculate rate (events per second)
    rate := float64(sum) / rt.interval.Seconds()
    
    // Store sample
    rt.mu.Lock()
    rt.samples[rt.head] = Sample{
        Timestamp: time.Now(),
        Value:     rate,
    }
    rt.head = (rt.head + 1) % rt.capacity
    if rt.count < rt.capacity {
        rt.count++
    }
    rt.mu.Unlock()
}

func (rt *RateTracker) CurrentRate() float64 {
    rt.mu.RLock()
    defer rt.mu.RUnlock()
    
    if rt.count == 0 {
        return 0
    }
    
    // Return most recent sample
    idx := (rt.head - 1 + rt.capacity) % rt.capacity
    return rt.samples[idx].Value
}

func (rt *RateTracker) TimeSeries(duration time.Duration) []Sample {
    rt.mu.RLock()
    defer rt.mu.RUnlock()
    
    cutoff := time.Now().Add(-duration)
    result := make([]Sample, 0, rt.count)
    
    // Read samples in chronological order
    start := (rt.head - rt.count + rt.capacity) % rt.capacity
    for i := 0; i < rt.count; i++ {
        idx := (start + i) % rt.capacity
        sample := rt.samples[idx]
        
        if sample.Timestamp.After(cutoff) {
            result = append(result, sample)
        }
    }
    
    return result
}

func (rt *RateTracker) Average(duration time.Duration) float64 {
    samples := rt.TimeSeries(duration)
    
    if len(samples) == 0 {
        return 0
    }
    
    sum := 0.0
    for _, s := range samples {
        sum += s.Value
    }
    
    return sum / float64(len(samples))
}

func (rt *RateTracker) Stop() {
    rt.ticker.Stop()
    close(rt.stopChan)
}
```

#### 1.3 Histogram for Latencies

**`pkg/metrics/histogram.go`**

```go
package metrics

import (
    "math"
    "sort"
    "sync"
)

// Histogram tracks distribution of values using buckets
type Histogram struct {
    buckets []float64
    counts  []int64
    sum     float64
    count   int64
    mu      sync.RWMutex
}

func NewHistogram(buckets []float64) *Histogram {
    // Ensure buckets are sorted
    sorted := make([]float64, len(buckets))
    copy(sorted, buckets)
    sort.Float64s(sorted)
    
    return &Histogram{
        buckets: sorted,
        counts:  make([]int64, len(sorted)+1),
    }
}

func (h *Histogram) Record(value float64) {
    h.mu.Lock()
    defer h.mu.Unlock()
    
    // Find bucket
    idx := sort.SearchFloat64s(h.buckets, value)
    h.counts[idx]++
    
    h.sum += value
    h.count++
}

func (h *Histogram) Percentile(p float64) float64 {
    h.mu.RLock()
    defer h.mu.RUnlock()
    
    if h.count == 0 {
        return 0
    }
    
    target := int64(math.Ceil(float64(h.count) * p))
    cumulative := int64(0)
    
    for i, count := range h.counts {
        cumulative += count
        if cumulative >= target {
            if i == 0 {
                return h.buckets[0]
            } else if i == len(h.buckets) {
                return h.buckets[len(h.buckets)-1]
            }
            return h.buckets[i]
        }
    }
    
    return h.buckets[len(h.buckets)-1]
}

func (h *Histogram) Mean() float64 {
    h.mu.RLock()
    defer h.mu.RUnlock()
    
    if h.count == 0 {
        return 0
    }
    
    return h.sum / float64(h.count)
}

func (h *Histogram) Count() int64 {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.count
}

func (h *Histogram) Reset() {
    h.mu.Lock()
    defer h.mu.Unlock()
    
    for i := range h.counts {
        h.counts[i] = 0
    }
    h.sum = 0
    h.count = 0
}
```

#### 1.4 Queue-Level Metrics

**`pkg/metrics/queue_metrics.go`**

```go
package metrics

import (
    "sync/atomic"
    "time"
)

// QueueMetrics tracks per-queue statistics
type QueueMetrics struct {
    QueueName string
    VHost     string
    
    // Rates
    publishRate   *RateTracker
    deliveryRate  *RateTracker
    ackRate       *RateTracker
    
    // Gauges
    depth         atomic.Int64
    consumerCount atomic.Int64
    
    // Created timestamp
    createdAt     time.Time
}

func NewQueueMetrics(vhost, queueName string, config *Config) *QueueMetrics {
    return &QueueMetrics{
        QueueName:    queueName,
        VHost:        vhost,
        publishRate:  NewRateTracker(config),
        deliveryRate: NewRateTracker(config),
        ackRate:      NewRateTracker(config),
        createdAt:    time.Now(),
    }
}

func (qm *QueueMetrics) RecordPublish() {
    qm.publishRate.Record(1)
    qm.depth.Add(1)
}

func (qm *QueueMetrics) RecordDelivery() {
    qm.deliveryRate.Record(1)
}

func (qm *QueueMetrics) RecordAck() {
    qm.ackRate.Record(1)
    qm.depth.Add(-1)
}

func (qm *QueueMetrics) SetDepth(depth int64) {
    qm.depth.Store(depth)
}

func (qm *QueueMetrics) SetConsumerCount(count int64) {
    qm.consumerCount.Store(count)
}

func (qm *QueueMetrics) Snapshot() *QueueSnapshot {
    return &QueueSnapshot{
        QueueName:     qm.QueueName,
        VHost:         qm.VHost,
        PublishRate:   qm.publishRate.CurrentRate(),
        DeliveryRate:  qm.deliveryRate.CurrentRate(),
        AckRate:       qm.ackRate.CurrentRate(),
        Depth:         qm.depth.Load(),
        ConsumerCount: qm.consumerCount.Load(),
        Uptime:        time.Since(qm.createdAt),
    }
}

type QueueSnapshot struct {
    QueueName     string        `json:"queue_name"`
    VHost         string        `json:"vhost"`
    PublishRate   float64       `json:"publish_rate"`
    DeliveryRate  float64       `json:"delivery_rate"`
    AckRate       float64       `json:"ack_rate"`
    Depth         int64         `json:"depth"`
    ConsumerCount int64         `json:"consumer_count"`
    Uptime        time.Duration `json:"uptime"`
}
```

#### 1.5 Exchange-Level Metrics

**`pkg/metrics/exchange_metrics.go`**

```go
package metrics

import (
    "time"
)

// ExchangeMetrics tracks per-exchange statistics
// Based on docs/exchange-stats-implementation.md pattern
type ExchangeMetrics struct {
    ExchangeName string
    VHost        string
    Type         string
    
    // Rates
    publishRate  *RateTracker
    
    // Counters
    publishCount atomic.Int64
    routedCount  atomic.Int64
    
    // Created timestamp
    createdAt    time.Time
}

func NewExchangeMetrics(vhost, exchangeName, exchangeType string, config *Config) *ExchangeMetrics {
    return &ExchangeMetrics{
        ExchangeName: exchangeName,
        VHost:        vhost,
        Type:         exchangeType,
        publishRate:  NewRateTracker(config),
        createdAt:    time.Now(),
    }
}

func (em *ExchangeMetrics) RecordPublish(routedToQueues int) {
    em.publishRate.Record(1)
    em.publishCount.Add(1)
    em.routedCount.Add(int64(routedToQueues))
}

func (em *ExchangeMetrics) Snapshot() *ExchangeSnapshot {
    return &ExchangeSnapshot{
        ExchangeName: em.ExchangeName,
        VHost:        em.VHost,
        Type:         em.Type,
        PublishRate:  em.publishRate.CurrentRate(),
        PublishCount: em.publishCount.Load(),
        RoutedCount:  em.routedCount.Load(),
        Uptime:       time.Since(em.createdAt),
    }
}

type ExchangeSnapshot struct {
    ExchangeName string        `json:"exchange_name"`
    VHost        string        `json:"vhost"`
    Type         string        `json:"type"`
    PublishRate  float64       `json:"publish_rate"`
    PublishCount int64         `json:"publish_count"`
    RoutedCount  int64         `json:"routed_count"`
    Uptime       time.Duration `json:"uptime"`
}
```

### 1.6 Broker Integration

**`internal/core/broker/broker.go`** (modifications)

```go
type Broker struct {
    // ...existing fields...
    
    metrics *metrics.Collector
}

func NewBroker(...) *Broker {
    b := &Broker{
        // ...existing initialization...
        metrics: metrics.NewCollector(metrics.DefaultConfig()),
    }
    return b
}

// Add metrics to GetMetrics() method
func (b *Broker) GetMetrics() *metrics.Snapshot {
    return b.metrics.Snapshot()
}
```

**Hook into message flow:**

```go
// In VHost publish method
func (v *VHost) Publish(exchange, routingKey string, msg *Message) error {
    start := time.Now()
    
    // Record publish
    v.broker.metrics.RecordPublish(len(msg.Body))
    
    // Existing publish logic...
    err := v.messageController.Publish(exchange, routingKey, msg)
    
    // Record latency
    latency := time.Since(start).Microseconds()
    v.broker.metrics.publishLatency.Record(float64(latency))
    
    return err
}

// In Queue delivery method
func (q *Queue) Deliver(consumer *Consumer) error {
    start := time.Now()
    
    // Existing delivery logic...
    err := q.deliverMessage(consumer)
    
    if err == nil {
        // Record delivery
        latency := time.Since(start).Microseconds()
        q.broker.metrics.RecordDelivery(latency)
    }
    
    return err
}

// In Channel ACK handler
func (ch *Channel) HandleAck(deliveryTag uint64, multiple bool) error {
    // Record ACK
    ch.broker.metrics.RecordAck()
    
    // Existing ACK logic...
    return ch.processAck(deliveryTag, multiple)
}
```

### 1.7 REST API Endpoints

**`web/handlers/api/metrics.go`** (new file)

```go
package api

import (
    "time"
    
    "github.com/gofiber/fiber/v2"
    "github.com/ottermq/ottermq/internal/core/broker"
)

type MetricsHandler struct {
    broker *broker.Broker
}

func NewMetricsHandler(b *broker.Broker) *MetricsHandler {
    return &MetricsHandler{broker: b}
}

// GET /api/metrics/overview
// Returns current snapshot of all metrics
func (h *MetricsHandler) GetOverview(c *fiber.Ctx) error {
    snapshot := h.broker.GetMetrics()
    return c.JSON(snapshot)
}

// GET /api/metrics/timeseries/publish-rate?duration=5m
// Returns time-series data for publish rate
func (h *MetricsHandler) GetPublishRateTimeSeries(c *fiber.Ctx) error {
    duration := parseDuration(c.Query("duration", "5m"))
    
    metrics := h.broker.GetMetrics()
    timeSeries := metrics.PublishRateTimeSeries(duration)
    
    return c.JSON(fiber.Map{
        "metric":     "publish_rate",
        "unit":       "messages/sec",
        "duration":   duration.String(),
        "samples":    timeSeries,
        "current":    metrics.PublishRate,
        "average":    calculateAverage(timeSeries),
    })
}

// GET /api/metrics/timeseries/delivery-rate?duration=5m
func (h *MetricsHandler) GetDeliveryRateTimeSeries(c *fiber.Ctx) error {
    duration := parseDuration(c.Query("duration", "5m"))
    
    metrics := h.broker.GetMetrics()
    timeSeries := metrics.DeliveryRateTimeSeries(duration)
    
    return c.JSON(fiber.Map{
        "metric":     "delivery_rate",
        "unit":       "messages/sec",
        "duration":   duration.String(),
        "samples":    timeSeries,
        "current":    metrics.DeliveryRate,
        "average":    calculateAverage(timeSeries),
    })
}

// GET /api/metrics/timeseries/queue-depths?duration=5m
// Returns aggregated queue depths over time
func (h *MetricsHandler) GetQueueDepthsTimeSeries(c *fiber.Ctx) error {
    duration := parseDuration(c.Query("duration", "5m"))
    
    // Get all queue metrics
    queues := h.broker.GetAllQueueMetrics()
    
    // Build time-series for each queue
    result := make(map[string][]metrics.Sample)
    for _, q := range queues {
        result[q.QueueName] = q.DepthTimeSeries(duration)
    }
    
    return c.JSON(fiber.Map{
        "metric":   "queue_depths",
        "unit":     "messages",
        "duration": duration.String(),
        "queues":   result,
    })
}

// GET /api/metrics/queues/:vhost/:queue
// Returns detailed metrics for specific queue
func (h *MetricsHandler) GetQueueMetrics(c *fiber.Ctx) error {
    vhost := c.Params("vhost")
    queue := c.Params("queue")
    
    queueMetrics := h.broker.GetQueueMetrics(vhost, queue)
    if queueMetrics == nil {
        return c.Status(404).JSON(fiber.Map{
            "error": "Queue not found",
        })
    }
    
    return c.JSON(queueMetrics.Snapshot())
}

// GET /api/metrics/exchanges/:vhost/:exchange
// Returns detailed metrics for specific exchange
func (h *MetricsHandler) GetExchangeMetrics(c *fiber.Ctx) error {
    vhost := c.Params("vhost")
    exchange := c.Params("exchange")
    
    exchangeMetrics := h.broker.GetExchangeMetrics(vhost, exchange)
    if exchangeMetrics == nil {
        return c.Status(404).JSON(fiber.Map{
            "error": "Exchange not found",
        })
    }
    
    return c.JSON(exchangeMetrics.Snapshot())
}

// Helper functions
func parseDuration(s string) time.Duration {
    d, err := time.ParseDuration(s)
    if err != nil {
        return 5 * time.Minute
    }
    return d
}

func calculateAverage(samples []metrics.Sample) float64 {
    if len(samples) == 0 {
        return 0
    }
    
    sum := 0.0
    for _, s := range samples {
        sum += s.Value
    }
    return sum / float64(len(samples))
}
```

**Register routes in `web/server.go`:**

```go
func SetupRoutes(app *fiber.App, broker *broker.Broker) {
    // ...existing routes...
    
    // Metrics endpoints
    metricsHandler := api.NewMetricsHandler(broker)
    api.GET("/metrics/overview", metricsHandler.GetOverview)
    api.GET("/metrics/timeseries/publish-rate", metricsHandler.GetPublishRateTimeSeries)
    api.GET("/metrics/timeseries/delivery-rate", metricsHandler.GetDeliveryRateTimeSeries)
    api.GET("/metrics/timeseries/queue-depths", metricsHandler.GetQueueDepthsTimeSeries)
    api.GET("/metrics/queues/:vhost/:queue", metricsHandler.GetQueueMetrics)
    api.GET("/metrics/exchanges/:vhost/:exchange", metricsHandler.GetExchangeMetrics)
}
```

### 1.8 UI Integration (Vue/Quasar)

**`ottermq_ui/src/pages/OverviewPage.vue`** (enhanced with charts)

```vue
<template>
  <q-page padding>
    <div class="row q-col-gutter-md">
      <!-- Current Stats Cards (existing) -->
      <div class="col-12 col-md-6 col-lg-3">
        <q-card>
          <q-card-section>
            <div class="text-h6">Publish Rate</div>
            <div class="text-h3">{{ metrics.publish_rate.toFixed(2) }}</div>
            <div class="text-caption">messages/sec</div>
          </q-card-section>
        </q-card>
      </div>
      
      <!-- More stat cards... -->
      
      <!-- Real-Time Charts -->
      <div class="col-12 col-lg-6">
        <q-card>
          <q-card-section>
            <div class="text-h6">Message Rate</div>
            <LineChart
              :data="publishRateData"
              :options="chartOptions"
              height="300"
            />
          </q-card-section>
        </q-card>
      </div>
      
      <div class="col-12 col-lg-6">
        <q-card>
          <q-card-section>
            <div class="text-h6">Queue Depths</div>
            <LineChart
              :data="queueDepthsData"
              :options="chartOptions"
              height="300"
            />
          </q-card-section>
        </q-card>
      </div>
    </div>
  </q-page>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { Line as LineChart } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend
} from 'chart.js'
import api from 'src/services/api'

// Register Chart.js components
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend
)

const metrics = ref({
  publish_rate: 0,
  delivery_rate: 0,
  message_count: 0,
})

const publishRateData = ref({
  labels: [],
  datasets: [{
    label: 'Publish Rate',
    data: [],
    borderColor: 'rgb(75, 192, 192)',
    tension: 0.1
  }, {
    label: 'Delivery Rate',
    data: [],
    borderColor: 'rgb(255, 99, 132)',
    tension: 0.1
  }]
})

const queueDepthsData = ref({
  labels: [],
  datasets: []
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  scales: {
    y: {
      beginAtZero: true
    }
  },
  animation: {
    duration: 0 // Disable animation for real-time updates
  }
}

let refreshInterval = null

async function fetchMetrics() {
  try {
    // Fetch current snapshot
    const overview = await api.get('/metrics/overview')
    metrics.value = overview.data
    
    // Fetch time-series data
    const publishRate = await api.get('/metrics/timeseries/publish-rate?duration=5m')
    const deliveryRate = await api.get('/metrics/timeseries/delivery-rate?duration=5m')
    const queueDepths = await api.get('/metrics/timeseries/queue-depths?duration=5m')
    
    // Update charts
    updatePublishRateChart(publishRate.data, deliveryRate.data)
    updateQueueDepthsChart(queueDepths.data)
    
  } catch (error) {
    console.error('Failed to fetch metrics:', error)
  }
}

function updatePublishRateChart(publishData, deliveryData) {
  // Extract timestamps and values
  const labels = publishData.samples.map(s => 
    new Date(s.timestamp).toLocaleTimeString()
  )
  
  publishRateData.value = {
    labels,
    datasets: [{
      label: 'Publish Rate (msg/s)',
      data: publishData.samples.map(s => s.value),
      borderColor: 'rgb(75, 192, 192)',
      backgroundColor: 'rgba(75, 192, 192, 0.1)',
      tension: 0.1,
      fill: true
    }, {
      label: 'Delivery Rate (msg/s)',
      data: deliveryData.samples.map(s => s.value),
      borderColor: 'rgb(255, 99, 132)',
      backgroundColor: 'rgba(255, 99, 132, 0.1)',
      tension: 0.1,
      fill: true
    }]
  }
}

function updateQueueDepthsChart(depthsData) {
  // Get unique timestamps
  const allTimestamps = new Set()
  Object.values(depthsData.queues).forEach(samples => {
    samples.forEach(s => allTimestamps.add(s.timestamp))
  })
  
  const labels = Array.from(allTimestamps)
    .sort()
    .map(ts => new Date(ts).toLocaleTimeString())
  
  // Create dataset for each queue
  const datasets = Object.entries(depthsData.queues).map(([queueName, samples]) => ({
    label: queueName,
    data: samples.map(s => s.value),
    borderColor: getRandomColor(),
    tension: 0.1
  }))
  
  queueDepthsData.value = { labels, datasets }
}

function getRandomColor() {
  const colors = [
    'rgb(75, 192, 192)',
    'rgb(255, 99, 132)',
    'rgb(54, 162, 235)',
    'rgb(255, 206, 86)',
    'rgb(153, 102, 255)',
  ]
  return colors[Math.floor(Math.random() * colors.length)]
}

onMounted(() => {
  // Initial fetch
  fetchMetrics()
  
  // Poll every 5 seconds
  refreshInterval = setInterval(fetchMetrics, 5000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})
</script>
```

**Install Chart.js dependency:**

```bash
cd ottermq_ui
npm install chart.js vue-chartjs
```

### Phase 1 Deliverables

- ✅ `pkg/metrics/` package with collector, rate tracker, histogram
- ✅ Per-queue and per-exchange metrics tracking
- ✅ REST API endpoints for time-series data
- ✅ Real-time charts in management UI
- ✅ 5-minute history in memory (configurable)
- ✅ No external dependencies

### Phase 1 Success Metrics

- Charts update every 5 seconds in UI
- Memory usage < 50MB for metrics (5 min retention)
- API response time < 10ms for time-series queries
- Accurate rate calculations (within 1% of actual)

---

## Phase 2: Prometheus Integration 🎯

**Timeline:** 1 week  
**Priority:** MEDIUM - Enables external monitoring  
**Dependencies:** Phase 1 complete

### Goals

1. Export metrics in Prometheus format
2. Enable long-term metric storage
3. Support Grafana dashboards
4. Enable alerting capabilities
5. Prepare for multi-node monitoring (clustering)

### Implementation

#### 2.1 Prometheus Exporter

**`pkg/metrics/prometheus.go`**

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Counters (monotonically increasing)
    messagesPublishedTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "ottermq_messages_published_total",
        Help: "Total number of messages published",
    })
    
    messagesDeliveredTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "ottermq_messages_delivered_total",
        Help: "Total number of messages delivered to consumers",
    })
    
    messagesAckedTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "ottermq_messages_acked_total",
        Help: "Total number of messages acknowledged",
    })
    
    messagesNackedTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "ottermq_messages_nacked_total",
        Help: "Total number of messages rejected (NACK)",
    })
    
    connectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "ottermq_connections_total",
        Help: "Total number of client connections",
    })
    
    // Gauges (can go up and down)
    connectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "ottermq_connections_active",
        Help: "Current number of active connections",
    })
    
    channelsActive = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "ottermq_channels_active",
        Help: "Current number of active channels",
    })
    
    queuesTotal = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "ottermq_queues_total",
        Help: "Current number of queues",
    })
    
    exchangesTotal = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "ottermq_exchanges_total",
        Help: "Current number of exchanges",
    })
    
    messagesReady = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "ottermq_messages_ready",
        Help: "Total number of messages ready for delivery",
    })
    
    messagesUnacked = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "ottermq_messages_unacked",
        Help: "Total number of messages delivered but not yet acknowledged",
    })
    
    consumersTotal = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "ottermq_consumers_total",
        Help: "Current number of active consumers",
    })
    
    // Per-queue metrics with labels
    queueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "ottermq_queue_depth",
        Help: "Number of messages in queue",
    }, []string{"vhost", "queue"})
    
    queueConsumers = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "ottermq_queue_consumers",
        Help: "Number of consumers on queue",
    }, []string{"vhost", "queue"})
    
    queuePublishRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "ottermq_queue_publish_rate",
        Help: "Messages published per second to queue",
    }, []string{"vhost", "queue"})
    
    queueDeliveryRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "ottermq_queue_delivery_rate",
        Help: "Messages delivered per second from queue",
    }, []string{"vhost", "queue"})
    
    // Per-exchange metrics
    exchangePublishRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "ottermq_exchange_publish_rate",
        Help: "Messages published per second to exchange",
    }, []string{"vhost", "exchange", "type"})
    
    // Histograms (distributions)
    publishLatency = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "ottermq_publish_latency_microseconds",
        Help:    "Publish latency distribution in microseconds",
        Buckets: prometheus.ExponentialBuckets(100, 2, 10), // 100µs to ~100ms
    })
    
    deliveryLatency = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "ottermq_delivery_latency_microseconds",
        Help:    "Delivery latency distribution in microseconds",
        Buckets: prometheus.ExponentialBuckets(100, 2, 10),
    })
    
    messageSize = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "ottermq_message_size_bytes",
        Help:    "Message size distribution in bytes",
        Buckets: prometheus.ExponentialBuckets(1024, 2, 10), // 1KB to ~1MB
    })
)

// PrometheusExporter wraps the internal metrics collector
// and exports to Prometheus
type PrometheusExporter struct {
    collector *Collector
}

func NewPrometheusExporter(collector *Collector) *PrometheusExporter {
    return &PrometheusExporter{
        collector: collector,
    }
}

// Update syncs internal metrics to Prometheus
// Call this periodically (e.g., every 5 seconds)
func (pe *PrometheusExporter) Update() {
    snapshot := pe.collector.Snapshot()
    
    // Update gauges
    connectionsActive.Set(float64(snapshot.ConnectionCount))
    channelsActive.Set(float64(snapshot.ChannelCount))
    queuesTotal.Set(float64(snapshot.QueueCount))
    exchangesTotal.Set(float64(snapshot.ExchangeCount))
    messagesReady.Set(float64(snapshot.MessageCount))
    consumersTotal.Set(float64(snapshot.ConsumerCount))
    
    // Update per-queue metrics
    pe.updateQueueMetrics()
    
    // Update per-exchange metrics
    pe.updateExchangeMetrics()
}

func (pe *PrometheusExporter) updateQueueMetrics() {
    queues := pe.collector.GetAllQueueMetrics()
    
    for _, q := range queues {
        snap := q.Snapshot()
        labels := prometheus.Labels{
            "vhost": snap.VHost,
            "queue": snap.QueueName,
        }
        
        queueDepth.With(labels).Set(float64(snap.Depth))
        queueConsumers.With(labels).Set(float64(snap.ConsumerCount))
        queuePublishRate.With(labels).Set(snap.PublishRate)
        queueDeliveryRate.With(labels).Set(snap.DeliveryRate)
    }
}

func (pe *PrometheusExporter) updateExchangeMetrics() {
    exchanges := pe.collector.GetAllExchangeMetrics()
    
    for _, ex := range exchanges {
        snap := ex.Snapshot()
        labels := prometheus.Labels{
            "vhost":    snap.VHost,
            "exchange": snap.ExchangeName,
            "type":     snap.Type,
        }
        
        exchangePublishRate.With(labels).Set(snap.PublishRate)
    }
}

// Recording methods (called from broker)
func RecordPublish(size int) {
    messagesPublishedTotal.Inc()
    messageSize.Observe(float64(size))
}

func RecordDelivery(latencyMicros int64) {
    messagesDeliveredTotal.Inc()
    deliveryLatency.Observe(float64(latencyMicros))
}

func RecordAck() {
    messagesAckedTotal.Inc()
}

func RecordNack() {
    messagesNackedTotal.Inc()
}

func RecordConnection() {
    connectionsTotal.Inc()
}

func RecordPublishLatency(latencyMicros int64) {
    publishLatency.Observe(float64(latencyMicros))
}
```

#### 2.2 Prometheus Endpoint

**`web/handlers/prometheus.go`**

```go
package handlers

import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/adaptor/v2"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServePrometheusMetrics returns Fiber handler for /metrics
func ServePrometheusMetrics() fiber.Handler {
    return adaptor.HTTPHandler(promhttp.Handler())
}
```

**Register in `web/server.go`:**

```go
import (
    "time"
    "github.com/ottermq/ottermq/pkg/metrics"
    "github.com/ottermq/ottermq/web/handlers"
)

func SetupRoutes(app *fiber.App, broker *broker.Broker) {
    // ...existing routes...
    
    // Prometheus metrics endpoint
    app.Get("/metrics", handlers.ServePrometheusMetrics())
    
    // Start Prometheus exporter update loop
    prometheusExporter := metrics.NewPrometheusExporter(broker.GetMetricsCollector())
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        for range ticker.C {
            prometheusExporter.Update()
        }
    }()
}
```

#### 2.3 Grafana Dashboard (JSON)

**`grafana/ottermq-dashboard.json`**

```json
{
  "dashboard": {
    "title": "OtterMQ Overview",
    "panels": [
      {
        "title": "Message Rate",
        "targets": [{
          "expr": "rate(ottermq_messages_published_total[1m])",
          "legendFormat": "Published"
        }, {
          "expr": "rate(ottermq_messages_delivered_total[1m])",
          "legendFormat": "Delivered"
        }],
        "type": "graph"
      },
      {
        "title": "Queue Depths",
        "targets": [{
          "expr": "ottermq_queue_depth",
          "legendFormat": "{{queue}}"
        }],
        "type": "graph"
      },
      {
        "title": "Active Connections",
        "targets": [{
          "expr": "ottermq_connections_active"
        }],
        "type": "stat"
      },
      {
        "title": "Message Latency (p99)",
        "targets": [{
          "expr": "histogram_quantile(0.99, rate(ottermq_delivery_latency_microseconds_bucket[5m]))"
        }],
        "type": "graph"
      }
    ]
  }
}
```

#### 2.4 Prometheus Configuration Example

**`prometheus.yml`**

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'ottermq'
    static_configs:
      - targets: ['localhost:3000']  # OtterMQ web server
    metrics_path: '/metrics'
```

#### 2.5 Docker Compose for Monitoring Stack

**`docker-compose.monitoring.yml`**

```yaml
version: '3.8'

services:
  ottermq:
    build: .
    ports:
      - "5672:5672"
      - "3000:3000"
    environment:
      - OTTERMQ_BROKER_PORT=5672
      - OTTERMQ_WEB_PORT=3000

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3001:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./grafana/datasources:/etc/grafana/provisioning/datasources

volumes:
  prometheus_data:
  grafana_data:
```

### Phase 2 Deliverables

- ✅ `/metrics` endpoint in Prometheus format
- ✅ Prometheus scrape configuration
- ✅ Pre-built Grafana dashboards
- ✅ Docker Compose monitoring stack
- ✅ Documentation for setup

### Phase 2 Success Metrics

- Prometheus successfully scrapes metrics every 15s
- Grafana dashboards display real-time data
- Historical data retained (Prometheus retention policy)
- Alerts can be configured (e.g., queue depth > threshold)

---

## Phase 3: OpenTelemetry & Distributed Tracing 🔭

**Timeline:** 2-3 weeks  
**Priority:** LOW - Advanced observability  
**Dependencies:** Phase 2 complete

### Goals

1. Trace message flow end-to-end
2. Identify latency bottlenecks
3. Visualize cross-node routing (for clustering)
4. Debug complex message flows

### Implementation Overview

```go
// pkg/observability/tracing.go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func TracePublish(ctx context.Context, exchange, routingKey string) (context.Context, trace.Span) {
    tracer := otel.Tracer("ottermq")
    ctx, span := tracer.Start(ctx, "amqp.publish",
        trace.WithAttributes(
            attribute.String("exchange", exchange),
            attribute.String("routing_key", routingKey),
        ),
    )
    return ctx, span
}

// Propagate context through message properties
// Extract trace context on delivery
// Export to Jaeger/Tempo
```

### Phase 3 Deliverables

- ✅ OpenTelemetry SDK integration
- ✅ Span creation for publish/route/deliver
- ✅ Context propagation via message headers
- ✅ Jaeger/Tempo export configuration
- ✅ Distributed tracing documentation

---

## Configuration Examples

### Metrics Configuration

**`config.yaml`**

```yaml
metrics:
  enabled: true
  
  # Internal metrics (for UI charts)
  internal:
    retention_window: 5m
    sample_interval: 5s
    rate_buffer_size: 60
    
  # Prometheus export
  prometheus:
    enabled: true
    update_interval: 5s
    
  # OpenTelemetry tracing
  tracing:
    enabled: false  # Phase 3
    endpoint: "localhost:4317"
    sample_rate: 0.1  # 10% sampling
```

### Environment Variables

```bash
# Enable/disable features
OTTERMQ_METRICS_ENABLED=true
OTTERMQ_PROMETHEUS_ENABLED=true
OTTERMQ_TRACING_ENABLED=false

# Retention settings
OTTERMQ_METRICS_RETENTION=5m
OTTERMQ_METRICS_SAMPLE_INTERVAL=5s

# Prometheus
OTTERMQ_PROMETHEUS_UPDATE_INTERVAL=5s

# Tracing
OTTERMQ_TRACING_ENDPOINT=localhost:4317
OTTERMQ_TRACING_SAMPLE_RATE=0.1
```

---

## Testing Strategy

### Unit Tests

```go
// pkg/metrics/rate_tracker_test.go
func TestRateTracker_Recording(t *testing.T) {
    config := &Config{
        RetentionWindow: 1 * time.Minute,
        SampleInterval:  100 * time.Millisecond,
        RateBufferSize:  10,
    }
    
    rt := NewRateTracker(config)
    defer rt.Stop()
    
    // Record events
    for i := 0; i < 100; i++ {
        rt.Record(1)
    }
    
    // Wait for sample
    time.Sleep(150 * time.Millisecond)
    
    // Check rate (100 events / 0.1s = 1000 events/sec)
    rate := rt.CurrentRate()
    assert.InDelta(t, 1000.0, rate, 50.0)
}
```

### Integration Tests

```go
// tests/integration/metrics_test.go
func TestMetricsEndpoint(t *testing.T) {
    // Start broker
    broker := setupTestBroker(t)
    defer broker.Close()
    
    // Publish messages
    publishTestMessages(t, broker, 100)
    
    // Query metrics API
    resp := httpGet(t, "http://localhost:3000/api/metrics/overview")
    
    var snapshot metrics.Snapshot
    json.Unmarshal(resp, &snapshot)
    
    // Verify
    assert.Greater(t, snapshot.PublishRate, 0.0)
    assert.Equal(t, int64(100), snapshot.MessageCount)
}
```

### Load Tests

```go
// tests/load/metrics_overhead_test.go
func BenchmarkMetricsOverhead(b *testing.B) {
    collector := metrics.NewCollector(metrics.DefaultConfig())
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        collector.RecordPublish(1024)
    }
    
    // Should be < 100ns per recording
}
```

---

## Migration Path

### Step 1: Add Metrics Package (Week 1)
- Create `pkg/metrics/` with core types
- Unit tests for rate tracker and histogram
- No broker integration yet

### Step 2: Instrument Broker (Week 1-2)
- Add metrics collector to broker
- Hook into publish/deliver/ack flows
- Verify metrics are recorded correctly

### Step 3: REST API (Week 2)
- Add `/api/metrics/*` endpoints
- Time-series data endpoints
- API documentation

### Step 4: UI Charts (Week 2)
- Install Chart.js in UI
- Create chart components
- Real-time updates via polling

### Step 5: Prometheus (Week 3)
- Add Prometheus exporter
- `/metrics` endpoint
- Grafana dashboards

### Step 6: Documentation (Ongoing)
- API documentation
- Prometheus setup guide
- Grafana dashboard usage
- Troubleshooting guide

---

## Performance Considerations

### Memory Usage

**Estimated memory per metric:**
- Rate tracker (60 samples): ~1KB
- Histogram (10 buckets): ~200 bytes
- Total for all metrics: ~50MB (5min retention)

**Optimization strategies:**
- Configurable retention window
- Circular buffers (no allocation after init)
- Atomic operations (lock-free where possible)

### CPU Overhead

**Estimated overhead:**
- Recording: ~50ns per event (atomic increment)
- Sampling: ~1µs per sample (periodic, not per-message)
- Total: < 1% CPU overhead at 100k msg/sec

**Optimization strategies:**
- Batch sampling (every 5s, not per message)
- Lock-free data structures
- Minimal allocations

### Network Bandwidth

**Prometheus scrape:**
- Metric payload: ~10KB per scrape
- Scrape interval: 15s
- Bandwidth: < 1KB/s

**API queries:**
- Time-series response: ~5KB per query
- Polling interval: 5s (UI)
- Bandwidth: ~1KB/s per client

---

## Monitoring the Monitor

### Key Metrics to Track

1. **Metrics Collection Performance**
   - Recording latency
   - Sampling latency
   - Memory usage

2. **API Performance**
   - Request latency (p50, p95, p99)
   - Request rate
   - Error rate

3. **Prometheus Export**
   - Scrape duration
   - Scrape failures
   - Stale metrics

### Health Checks

```go
// pkg/metrics/health.go
func (c *Collector) HealthCheck() error {
    // Check if sampling is working
    lastSample := c.publishRate.GetLastSampleTime()
    if time.Since(lastSample) > 2*c.config.SampleInterval {
        return fmt.Errorf("sampling stalled")
    }
    
    // Check memory usage
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    if m.Alloc > 100*1024*1024 { // 100MB
        return fmt.Errorf("excessive memory usage")
    }
    
    return nil
}
```

---

## Documentation Checklist

- [ ] README update with metrics features
- [ ] API documentation for `/api/metrics/*`
- [ ] Prometheus setup guide
- [ ] Grafana dashboard documentation
- [ ] Configuration reference
- [ ] Troubleshooting guide
- [ ] Performance tuning guide
- [ ] Migration guide from no-metrics

---

## Future Enhancements

### Advanced Metrics
- Per-consumer metrics
- Per-connection metrics
- Message flow tracing
- Error rate tracking

### Advanced Visualizations
- Heatmaps for latency distribution
- Topology graphs
- Flow diagrams
- Custom dashboards

### Alerting
- Queue depth thresholds
- Latency thresholds
- Error rate thresholds
- Connection limits

### Multi-Tenancy
- Per-vhost metrics isolation
- Per-user metrics
- Quota tracking
- Usage billing

---

## References

- **Prometheus Best Practices**: https://prometheus.io/docs/practices/naming/
- **Grafana Dashboards**: https://grafana.com/docs/grafana/latest/dashboards/
- **OpenTelemetry Go**: https://opentelemetry.io/docs/instrumentation/go/
- **Chart.js**: https://www.chartjs.org/docs/latest/
- **RabbitMQ Metrics**: https://www.rabbitmq.com/monitoring.html
- **Kafka Metrics**: https://kafka.apache.org/documentation/#monitoring

---

## Summary

This observability roadmap provides a comprehensive path from basic real-time charts to full Prometheus/Grafana integration:

1. **Phase 1** solves your immediate need (UI charts) in 1-2 weeks
2. **Phase 2** adds professional monitoring (Prometheus/Grafana) in 1 week
3. **Phase 3** enables advanced tracing (optional, 2-3 weeks)

The architecture is layered - each phase builds on the previous, and you can stop at any point based on your needs. Phase 1 alone will give you impressive real-time dashboards without external dependencies.
