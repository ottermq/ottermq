# Prometheus Exporter Architecture

## Design Principle: Single Source of Truth

The broker code should **never** call Prometheus exporter methods directly. The exporter acts as a **bridge** that periodically reads from `pkg/metrics/Collector` and syncs to Prometheus.

## Data Flow

```code
┌─────────────────────────────────────────────────────────┐
│                    Broker Code                          │
│                                                         │
│  On message publish:                                    │
│    collector.RecordPublish(size)  ← ONLY THIS           │
│                                                         │
│  On message delivery:                                   │
│    collector.RecordDelivery(latency)                    │
│                                                         │
│  On ack:                                                │
│    collector.RecordAck()                                │
└─────────────────┬───────────────────────────────────────┘
                  │
                  │ Broker calls collector ONLY
                  │ (never calls exporter directly)
                  ▼
┌─────────────────────────────────────────────────────────┐
│           pkg/metrics/Collector                         │
│                                                         │
│  publishedTotal    = 15,234  (atomic counter)           │
│  deliveredTotal    = 15,100  (atomic counter)           │
│  ackedTotal        = 14,950  (atomic counter)           │
│  publishRate       = 125.3   (current rate)             │
│  messageCount      = 284     (current depth)            │
│                                                         │
│  Methods:                                               │
│  • RecordPublish()   - increments publishedTotal        │
│  • RecordDelivery()  - increments deliveredTotal        │
│  • GetBrokerSnapshot() - returns current state          │
└─────────────────┬───────────────────────────────────────┘
                  │
                  │ Exporter polls every 5 seconds
                  │
                  ▼
┌────────────────────────────────────────────────────────┐
│        web/prometheus/Exporter                         │
│                                                        │
│  updateLoop() runs every 5s:                           │
│    1. snapshot := collector.GetBrokerSnapshot()        │
│    2. Calculate deltas for counters                    │
│    3. Update Prometheus metrics                        │
│                                                        │
│  Delta Tracking Example:                               │
│  ┌────────────────────────────────────────────────┐    │
│  │ Update #1 (t=0s):                              │    │
│  │   snapshot.PublishedTotal = 15,234             │    │
│  │   lastPublished = 0                            │    │
│  │   delta = 15,234 - 0 = 15,234                  │    │
│  │   prometheusCounter.Add(15,234)                │    │
│  │   lastPublished = 15,234                       │    │
│  └────────────────────────────────────────────────┘    │
│  ┌────────────────────────────────────────────────┐    │
│  │ Update #2 (t=5s):                              │    │
│  │   snapshot.PublishedTotal = 15,789             │    │
│  │   lastPublished = 15,234                       │    │
│  │   delta = 15,789 - 15,234 = 555                │    │
│  │   prometheusCounter.Add(555)                   │    │
│  │   lastPublished = 15,789                       │    │
│  └────────────────────────────────────────────────┘    │
│                                                        │
│  Gauges (no delta needed):                             │
│    publishRate.Set(125.3)  ← Direct assignment         │
│    messageCount.Set(284)   ← Direct assignment         │
└─────────────────┬──────────────────────────────────────┘
                  │
                  │ Prometheus scrapes /metrics
                  │
                  ▼
┌─────────────────────────────────────────────────────────┐
│               Prometheus Server                         │
│                                                         │
│  Scrapes http://localhost:9090/metrics every 15s        │
│                                                         │
│  # TYPE ottermq_messages_published_total counter        │
│  ottermq_messages_published_total 15789                 │
│                                                         │
│  # TYPE ottermq_messages_publish_rate gauge             │
│  ottermq_messages_publish_rate 125.3                    │
└─────────────────────────────────────────────────────────┘
```

## Counter vs Gauge Strategy

### Counters (Monotonically Increasing)

- **What**: Cumulative totals that only go up
- **Examples**: `messages_published_total`, `messages_acked_total`
- **How**: Calculate delta from last sync, add to Prometheus counter
- **Why**: Prometheus counters can't be set to arbitrary values

```go
// In collector
atomic.AddInt64(&c.publishedTotal, 1) // Increments cumulative total

// In exporter update loop
currentTotal := snapshot.PublishedTotal  // e.g., 15,789
delta := currentTotal - e.lastPublished  // 15,789 - 15,234 = 555
e.publishedTotal.Add(float64(delta))     // Add delta to Prometheus
e.lastPublished = currentTotal           // Remember for next time
```

### Gauges (Can Go Up or Down)

- **What**: Current state values
- **Examples**: `connection_count`, `queue_depth`, `publish_rate`
- **How**: Directly set from snapshot
- **Why**: Gauges represent point-in-time values

```go
// In exporter update loop
e.messageCount.Set(float64(snapshot.MessageCount))    // Just set it
e.connectionCount.Set(float64(snapshot.ConnectionCount))
```

## Required Changes to pkg/metrics/Collector

The collector needs to expose **cumulative totals** for the exporter to calculate deltas:

```go
// pkg/metrics/collector.go
type Collector struct {
    // Cumulative counters (for Prometheus deltas)
    publishedTotal int64  // Total messages published since startup
    deliveredTotal int64  // Total messages delivered since startup
    ackedTotal     int64  // Total acks since startup
    nackedTotal    int64  // Total nacks since startup
    
    // Rate trackers (for rates and time series)
    publishRate    *RateTracker
    deliveryRate   *RateTracker
    
    // Current state gauges
    messageCount   atomic.Int64
    queueCount     atomic.Int64
    // ...
}

func (c *Collector) RecordPublish(size int) {
    atomic.AddInt64(&c.publishedTotal, 1)  // Increment cumulative
    c.publishRate.Record(1)                // Track for rate calculation
    c.messageSize.Record(float64(size))
}

func (c *Collector) GetBrokerSnapshot() *BrokerSnapshot {
    return &BrokerSnapshot{
        // Cumulative totals (for Prometheus counters)
        PublishedTotal:  atomic.LoadInt64(&c.publishedTotal),
        DeliveredTotal:  atomic.LoadInt64(&c.deliveredTotal),
        AckedTotal:      atomic.LoadInt64(&c.ackedTotal),
        NackedTotal:     atomic.LoadInt64(&c.nackedTotal),
        
        // Rates (for gauges and UI charts)
        PublishRate:     c.publishRate.CurrentRate(),
        DeliveryRate:    c.deliveryRate.CurrentRate(),
        
        // Current state (for gauges)
        MessageCount:    c.messageCount.Load(),
        QueueCount:      c.queueCount.Load(),
        ConnectionCount: c.connectionCount.Load(),
    }
}
```

## Benefits of This Approach

1. **Decoupling**: Broker code has zero knowledge of Prometheus
2. **Optional**: Can disable Prometheus without changing broker code
3. **Single Source**: Collector is the authority for all metrics
4. **Flexibility**: Easy to add other exporters (StatsD, InfluxDB, etc.)
5. **Testing**: Can test broker metrics without Prometheus running
6. **Performance**: Prometheus sync is async, doesn't block broker

## Example Broker Code

```go
// internal/core/broker/vhost/queue.go
func (q *Queue) Publish(msg *Message) error {
    // Business logic
    q.messages <- msg
    
    // Metrics recording (ONLY collector, never exporter)
    if q.metricsCollector != nil {
        q.metricsCollector.RecordPublish(len(msg.Body))
    }
    
    // NO: q.prometheusExporter.RecordPublish() ← NEVER DO THIS
    
    return nil
}
```

## Why Not Call Exporter Directly?

### ❌ Anti-Pattern: Direct Prometheus Calls

```go
// DON'T DO THIS
func (q *Queue) Publish(msg *Message) error {
    q.messages <- msg
    
    // Problem: Broker is now coupled to Prometheus
    if q.prometheusExporter != nil {
        q.prometheusExporter.RecordPublish()  // ❌ Bad!
    }
    
    return nil
}
```

**Problems:**

- Broker code knows about Prometheus (tight coupling)
- Can't disable Prometheus without changing broker code
- Hard to add other monitoring systems
- Two calls per event (collector + exporter)
- If Prometheus is slow, it blocks the broker

### ✅ Correct Pattern: Single Source

```go
// DO THIS
func (q *Queue) Publish(msg *Message) error {
    q.messages <- msg
    
    // Only call collector
    if q.metricsCollector != nil {
        q.metricsCollector.RecordPublish(len(msg.Body))  // ✅ Good!
    }
    
    return nil
}

// Exporter syncs periodically in background
// updateLoop() reads from collector every 5s
```

**Benefits:**

- Broker doesn't know about Prometheus
- Can enable/disable Prometheus via config
- Can add StatsD, InfluxDB, etc. without changing broker
- Single write per event (fast)
- Sync happens async in background

## Summary

**Golden Rule**: The broker should only talk to `pkg/metrics/Collector`. The Prometheus exporter is just another observer that periodically syncs metrics from the collector to Prometheus.

This is the standard pattern used by mature systems like:

- RabbitMQ (internal stats → Prometheus plugin)
- NATS (internal stats → Prometheus exporter)
- Kafka (JMX metrics → Prometheus exporter)
