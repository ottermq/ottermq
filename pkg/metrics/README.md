# OtterMQ Metrics Package

The `metrics` package provides a comprehensive, high-performance metrics collection system for the OtterMQ message broker. It tracks broker-wide, exchange-level, and queue-level statistics with time-series capabilities for monitoring and observability.

## Overview

This package implements a lock-free, thread-safe metrics collection system using atomic operations and sync.Map for concurrent access. It provides:

- **Real-time metrics**: Current counts and rates for connections, channels, messages, consumers
- **Time-series data**: Historical samples for trending and visualization
- **Rate calculations**: Automatic rate tracking over configurable time windows
- **Granular tracking**: Separate metrics for exchanges, queues, and broker-level operations
- **Zero-allocation sampling**: Efficient ring buffer implementation for time-series storage

## Key Components

### Collector

The `Collector` is the central aggregation point for all broker metrics. It manages:

- Exchange-level metrics (publish rates, delivery rates per exchange)
- Queue-level metrics (message rates, ack rates, queue depths per queue)
- Broker-level metrics (total publish rate, connection count, channel count)

### RateTracker

A ring buffer-based time-series tracker that:

- Records samples at regular intervals
- Calculates rates over configurable time windows
- Provides access to historical samples for charting

### Metric Types

#### Exchange Metrics

- **Publish Rate**: Messages published to the exchange per second
- **Publish Count**: Total messages published to the exchange
- **Delivery Rate**: Messages routed from the exchange per second
- **Delivery Count**: Total messages routed from the exchange

#### Queue Metrics
- **Message Rate**: Messages enqueued per second
- **Message Count**: Current queue depth (ready messages)
- **Delivery Rate**: Messages delivered to consumers per second
- **Ack Rate**: Acknowledgments per second
- **Ack Count**: Total acknowledgments
- **Unacked Count**: Current unacknowledged messages
- **Consumer Count**: Active consumers on the queue

#### Broker Metrics
- **Publish Rate**: Total publishes across all exchanges per second
- **Delivery Rate**: Total deliveries (auto-ack and manual-ack) per second
- **Ack Rate**: Total acknowledgments per second
- **Nack Rate**: Total negative acknowledgments per second
- **Connection Rate**: Active connection count
- **Channel Rate**: Active channel count
- **Total Ready Depth**: Sum of all queue depths
- **Total Unacked Depth**: Sum of all unacknowledged messages
- **Message Count**: Total messages in the system
- **Consumer Count**: Total active consumers
- **Queue Count**: Total queues
- **Exchange Count**: Total exchanges

## Usage

### Initialization

```go
import (
    "context"
    "github.com/ottermq/ottermq/pkg/metrics"
)

// Use default configuration (5-minute window, 60 samples, 5-second intervals)
collector := metrics.NewCollector(metrics.DefaultConfig(), context.Background())

// Or use custom configuration
config := &metrics.Config{
    Enabled:         true,
    WindowSize:      10 * time.Minute,
    MaxSamples:      120,
    SamplesInterval: 5,  // seconds
}
collector := metrics.NewCollector(config, context.Background())

// Start periodic sampling (required for time-series tracking)
collector.StartPeriodicSampling()
```

### Recording Metrics

#### Exchange Operations

```go
// Record a message published to an exchange
collector.RecordExchangePublish("my-exchange", "direct")

// Record a message routed from an exchange
collector.RecordExchangeDelivery("my-exchange")

// Remove exchange metrics when exchange is deleted
collector.RemoveExchange("my-exchange")
```

#### Queue Operations

```go
// Record a message enqueued to a queue
collector.RecordQueuePublish("my-queue")

// Record a message delivered from a queue
autoAck := false
collector.RecordQueueDelivery("my-queue", autoAck)

// Record a message requeued (e.g., after NACK with requeue=true)
collector.RecordQueueRequeue("my-queue")

// Record an acknowledgment
collector.RecordQueueAck("my-queue")

// Record a negative acknowledgment
collector.RecordQueueNack("my-queue")

// Track consumer lifecycle
collector.RecordConsumerAdded("my-queue")
collector.RecordConsumerRemoved("my-queue")

// Remove queue metrics when queue is deleted
collector.RemoveQueue("my-queue")
```

#### Connection and Channel Lifecycle

```go
// Track connections
collector.RecordConnection()
collector.RecordConnectionClose()

// Track channels
collector.RecordChannelOpen()
collector.RecordChannelClose()
```

### Retrieving Metrics

#### Snapshots (Current State)

```go
// Get broker-wide snapshot
snapshot := collector.GetBrokerSnapshot()
fmt.Printf("Publish Rate: %.2f msg/s\n", snapshot.PublishRate)
fmt.Printf("Total Messages: %d\n", snapshot.MessageCount)
fmt.Printf("Active Connections: %d\n", snapshot.ConnectionCount)

// Get exchange snapshot
exchangeSnapshot := collector.GetExchangeSnapshot("my-exchange")
if exchangeSnapshot != nil {
    fmt.Printf("Exchange %s: %.2f msg/s\n", 
        exchangeSnapshot.Name, exchangeSnapshot.PublishRate)
}

// Get queue snapshot
queueSnapshot := collector.GetQueueSnapshot("my-queue")
if queueSnapshot != nil {
    fmt.Printf("Queue %s: %d ready, %d unacked\n",
        queueSnapshot.Name, 
        queueSnapshot.MessageCount,
        queueSnapshot.UnackedCount)
}
```

#### Time-Series Data

```go
// Get historical samples for charts/graphs
duration := 5 * time.Minute

publishSamples := collector.GetPublishRateTimeSeries(duration)
deliverySamples := collector.GetDeliveryAutoAckRateTimeSeries(duration)
ackSamples := collector.GetAckRateTimeSeries(duration)

for _, sample := range publishSamples {
    fmt.Printf("Time: %v, Value: %d, Rate: %.2f\n",
        sample.Timestamp, sample.Value, sample.Rate)
}
```

#### All Exchanges and Queues

```go
// Get metrics for all exchanges
allExchanges := collector.GetAllExchangeMetrics()
for _, em := range allExchanges {
    snapshot := em.Snapshot()
    fmt.Printf("%s: %.2f msg/s\n", snapshot.Name, snapshot.PublishRate)
}

// Get metrics for all queues
allQueues := collector.GetAllQueueMetrics()
for _, qm := range allQueues {
    snapshot := qm.Snapshot()
    fmt.Printf("%s: %d messages\n", snapshot.Name, snapshot.MessageCount)
}
```

## Configuration Options

```go
type Config struct {
    // Enable or disable metrics collection
    Enabled bool

    // Time window for rate calculations (e.g., 5 minutes)
    WindowSize time.Duration

    // Maximum number of samples to keep in ring buffer
    // For 5-minute window with 5-second intervals: 60 samples
    MaxSamples int

    // Interval between samples in seconds
    // Determines how often StartPeriodicSampling records new samples
    SamplesInterval uint8
}
```

**Default Configuration:**
- `Enabled`: `true`
- `WindowSize`: `5 minutes`
- `MaxSamples`: `60`
- `SamplesInterval`: `5 seconds`

This provides 60 samples over a 5-minute rolling window, sampled every 5 seconds.

## Architecture

### Concurrency Model

- **Lock-free counters**: Uses `atomic.Int64` for all counters
- **Concurrent maps**: Uses `sync.Map` for exchange and queue metrics
- **RWMutex**: Only used for snapshot operations to ensure consistency
- **Background sampling**: Single goroutine samples all metrics periodically

### Sampling Behavior

The `StartPeriodicSampling()` method launches a background goroutine that:

1. **Samples cumulative gauges** (queue depths, connection counts) directly
2. **Samples and resets counters** (publish count, ack count) to calculate rates
3. **Records samples** to RateTracker ring buffers for time-series access
4. **Iterates all exchanges and queues** to sample their individual metrics

### Rate Calculation

Rates are calculated using a sliding window approach:

```
Rate = (Current Value - Oldest Value in Window) / Window Duration
```

For counters that reset after each sample, the rate represents the delta over the last sample interval.

## Performance Considerations

- **Zero allocations** in hot path (recording metrics)
- **Minimal locking**: Only during snapshot generation
- **Efficient sampling**: Single background goroutine, configurable interval
- **Ring buffer**: Fixed memory footprint per RateTracker
- **Lazy initialization**: Metrics are created on first use

## Integration with OtterMQ

The metrics package is integrated throughout the broker:

- **Connection handler**: Records connection/channel lifecycle events
- **AMQP processors**: Records publishes, deliveries, acks, nacks
- **VHost**: Records exchange and queue operations
- **Web API**: Exposes snapshots and time-series via REST endpoints

## Testing

```go
// Clear all metrics (useful for testing)
collector.Clear()

// Disable metrics collection
config := &metrics.Config{Enabled: false}
collector := metrics.NewCollector(config, ctx)

// Check if metrics are enabled
if collector.IsEnabled() {
    // Record metrics
}
```

## Future Enhancements

- **Percentile tracking**: p50, p95, p99 latencies for message processing
- **Histograms**: Message size distribution
- **Alerting hooks**: Callbacks when metrics exceed thresholds
- **Prometheus export**: Native Prometheus metrics endpoint
- **Custom aggregations**: User-defined metric calculations

## See Also

- [RateTracker Implementation](rate_tracker.go)
- [Collector Tests](collector_test.go)
- [OtterMQ Web API](../../web/)
