# Exchange Statistics Implementation Guide

**Status**: Planned for future milestone  
**Created**: 2025-11-19  
**Target**: Management UI Charts and Monitoring  
**Related**: Management API Refactoring (`feature/management-api-refactoring`)

## Overview

This document describes the implementation of exchange message rate statistics using a ring buffer (sliding window) approach, matching RabbitMQ's management plugin behavior. This is required for generating rate charts in the management UI.

## Why Ring Buffer Approach

The ring buffer approach provides:

1. **Accurate Rates** - Independent of polling frequency
2. **Smooth Graphs** - Averages out traffic spikes over time windows
3. **RabbitMQ Compatibility** - Matches the behavior users expect
4. **Historical Data** - Maintains samples for charting over time
5. **Predictable Memory** - Fixed buffer size regardless of traffic

### Alternative Considered

Simple counter-based calculation (Option 1) was considered but has a critical limitation:

```go
// Problem: Rate accuracy depends on when GetStats() is called
elapsed := now.Sub(lastSampleTime).Seconds()
rate = messageCount / elapsed  // ❌ Inaccurate if polling is irregular
```

If the UI doesn't poll for 60 seconds, then suddenly polls again, the calculated rate will be artificially low, making graphs unreliable.

## Architecture

### Components

```code
┌──────────────────────────────────────────────────────────┐
│                        VHost                             │
│  ┌────────────────────────────────────────────────────┐  │
│  │         Background Stats Sampler                   │  │
│  │         (goroutine, 5s interval)                   │  │
│  └────────────────────┬───────────────────────────────┘  │
│                       │                                  │
│                       ▼                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │ Exchange 1  │  │ Exchange 2  │  │ Exchange N  │       │
│  │             │  │             │  │             │       │
│  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │       │
│  │ │ Counter │ │  │ │ Counter │ │  │ │ Counter │ │       │
│  │ │ (atomic)│ │  │ │ (atomic)│ │  │ │ (atomic)│ │       │
│  │ └────┬────┘ │  │ └────┬────┘ │  │ └────┬────┘ │       │
│  │      │      │  │      │      │  │      │      │       │
│  │      ▼      │  │      ▼      │  │      ▼      │       │
│  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │       │
│  │ │  Ring   │ │  │ │  Ring   │ │  │ │  Ring   │ │       │
│  │ │ Buffer  │ │  │ │ Buffer  │ │  │ │ Buffer  │ │       │
│  │ └─────────┘ │  │ └─────────┘ │  │ └─────────┘ │       │
│  └─────────────┘  └─────────────┘  └─────────────┘       │
└──────────────────────────────────────────────────────────┘
                       │
                       ▼
              ┌────────────────┐
              │ Management API │
              │   GetStats()   │
              └────────────────┘
                       │
                       ▼
              ┌────────────────┐
              │   UI Charts    │
              └────────────────┘
```

### Data Flow

1. **Message Routing**: Exchange increments atomic counters (publishCount, deliverCount)
2. **Background Sampling**: VHost goroutine samples counters every 5 seconds
3. **Ring Buffer Storage**: Samples stored with timestamps, old samples pruned
4. **Rate Calculation**: Management API queries ring buffer for msg/sec rate
5. **UI Display**: Frontend renders charts using rate data

## Implementation Plan

### Phase 1: Stats Package (Core Infrastructure)

Create `pkg/stats/rate_tracker.go`:

```go
package stats

import (
    "sync"
    "time"
)

// RateTracker maintains a sliding window of samples for rate calculation
type RateTracker struct {
    mu         sync.RWMutex
    samples    []Sample
    windowSize time.Duration
    maxSamples int
}

// Sample represents a point-in-time measurement
type Sample struct {
    Count     int64     // Cumulative count at this time
    Timestamp time.Time
}

// NewRateTracker creates a tracker with specified window
func NewRateTracker(windowSize time.Duration, maxSamples int) *RateTracker {
    return &RateTracker{
        samples:    make([]Sample, 0, maxSamples),
        windowSize: windowSize,
        maxSamples: maxSamples,
    }
}

// Record adds a new sample and prunes old ones
func (rt *RateTracker) Record(totalCount int64) {
    rt.mu.Lock()
    defer rt.mu.Unlock()
    
    now := time.Now()
    
    // Add new sample
    rt.samples = append(rt.samples, Sample{
        Count:     totalCount,
        Timestamp: now,
    })
    
    // Prune samples outside window
    cutoff := now.Add(-rt.windowSize)
    validIdx := 0
    for i, sample := range rt.samples {
        if sample.Timestamp.After(cutoff) {
            validIdx = i
            break
        }
    }
    
    // Keep only valid samples
    if validIdx > 0 {
        copy(rt.samples, rt.samples[validIdx:])
        rt.samples = rt.samples[:len(rt.samples)-validIdx]
    }
    
    // Cap at maxSamples to prevent unbounded growth
    if len(rt.samples) > rt.maxSamples {
        excess := len(rt.samples) - rt.maxSamples
        copy(rt.samples, rt.samples[excess:])
        rt.samples = rt.samples[:rt.maxSamples]
    }
}

// Rate calculates messages per second over the window
func (rt *RateTracker) Rate() float64 {
    rt.mu.RLock()
    defer rt.mu.RUnlock()
    
    if len(rt.samples) < 2 {
        return 0.0
    }
    
    oldest := rt.samples[0]
    newest := rt.samples[len(rt.samples)-1]
    
    elapsed := newest.Timestamp.Sub(oldest.Timestamp).Seconds()
    if elapsed <= 0 {
        return 0.0
    }
    
    countDelta := newest.Count - oldest.Count
    return float64(countDelta) / elapsed
}

// GetStats returns both current count and rate
func (rt *RateTracker) GetStats() (count int64, rate float64) {
    rt.mu.RLock()
    defer rt.mu.RUnlock()
    
    if len(rt.samples) == 0 {
        return 0, 0.0
    }
    
    newest := rt.samples[len(rt.samples)-1]
    count = newest.Count
    
    if len(rt.samples) >= 2 {
        oldest := rt.samples[0]
        elapsed := newest.Timestamp.Sub(oldest.Timestamp).Seconds()
        if elapsed > 0 {
            rate = float64(newest.Count - oldest.Count) / elapsed
        }
    }
    
    return count, rate
}

// GetSamples returns a copy of all samples (for charting)
func (rt *RateTracker) GetSamples() []Sample {
    rt.mu.RLock()
    defer rt.mu.RUnlock()
    
    samples := make([]Sample, len(rt.samples))
    copy(samples, rt.samples)
    return samples
}

// Clear resets all samples
func (rt *RateTracker) Clear() {
    rt.mu.Lock()
    defer rt.mu.Unlock()
    rt.samples = rt.samples[:0]
}
```

**Testing**: Create `pkg/stats/rate_tracker_test.go` with:

- Sample recording and pruning
- Rate calculation accuracy
- Concurrent access (race detector)
- Edge cases (empty buffer, single sample, time boundaries)

### Phase 2: Exchange Integration

Modify `internal/core/broker/vhost/exchange.go`:

```go
package vhost

import (
    "sync/atomic"
    "time"
    
    "github.com/andrelcunha/ottermq/pkg/stats"
)

type Exchange struct {
    Name       string
    Type       string
    Durable    bool
    AutoDelete bool
    Internal   bool
    Arguments  map[string]any
    Bindings   []*Binding
    
    // Statistics (always enabled)
    publishCount atomic.Int64  // Cumulative publish counter
    deliverCount atomic.Int64  // Cumulative deliver counter
    
    // Rate tracking (optional, nil if disabled)
    publishRate *stats.RateTracker
    deliverRate *stats.RateTracker
}

// ExchangeOptions configures exchange creation
type ExchangeOptions struct {
    EnableStats   bool
    StatsWindow   time.Duration // Default: 5s
    MaxSamples    int           // Default: 60 (5 minutes at 5s intervals)
}

// NewExchange creates exchange with optional stats tracking
func NewExchange(name, exchangeType string, durable, autoDelete, internal bool, 
                 arguments map[string]any, opts ExchangeOptions) *Exchange {
    
    ex := &Exchange{
        Name:       name,
        Type:       exchangeType,
        Durable:    durable,
        AutoDelete: autoDelete,
        Internal:   internal,
        Arguments:  arguments,
        Bindings:   make([]*Binding, 0),
    }
    
    if opts.EnableStats {
        windowSize := opts.StatsWindow
        if windowSize == 0 {
            windowSize = 5 * time.Second
        }
        
        maxSamples := opts.MaxSamples
        if maxSamples == 0 {
            maxSamples = 60
        }
        
        ex.publishRate = stats.NewRateTracker(windowSize, maxSamples)
        ex.deliverRate = stats.NewRateTracker(windowSize, maxSamples)
    }
    
    return ex
}

// RecordPublish increments publish counter (called on every basic.publish)
func (e *Exchange) RecordPublish() {
    e.publishCount.Add(1)
}

// RecordDeliver increments deliver counter (called after routing)
// numQueues is the number of queues the message was routed to
func (e *Exchange) RecordDeliver(numQueues int) {
    if numQueues > 0 {
        e.deliverCount.Add(int64(numQueues))
    }
}

// SampleStats records current counters for rate calculation
// Must be called periodically by background goroutine
func (e *Exchange) SampleStats() {
    if e.publishRate == nil {
        return // Stats disabled
    }
    
    pubCount := e.publishCount.Load()
    delCount := e.deliverCount.Load()
    
    e.publishRate.Record(pubCount)
    e.deliverRate.Record(delCount)
}

// GetStats returns current statistics
func (e *Exchange) GetStats() ExchangeStats {
    stats := ExchangeStats{
        PublishCount: e.publishCount.Load(),
        DeliverCount: e.deliverCount.Load(),
    }
    
    if e.publishRate != nil {
        _, stats.PublishRate = e.publishRate.GetStats()
        _, stats.DeliverRate = e.deliverRate.GetStats()
        stats.PublishSamples = e.publishRate.GetSamples()
        stats.DeliverSamples = e.deliverRate.GetSamples()
    }
    
    return stats
}

// ExchangeStats contains all statistical information
type ExchangeStats struct {
    PublishCount int64
    DeliverCount int64
    PublishRate  float64         // msg/sec
    DeliverRate  float64         // msg/sec
    
    // For charting (optional, only if stats enabled)
    PublishSamples []stats.Sample
    DeliverSamples []stats.Sample
}
```

### Phase 3: VHost Background Sampler

Add to `internal/core/broker/vhost/vhost.go`:

```go
package vhost

import (
    "context"
    "time"
    
    "github.com/rs/zerolog/log"
)

type VHost struct {
    // ...existing fields...
    
    ctx        context.Context
    cancelFunc context.CancelFunc
    
    // Stats configuration
    statsEnabled      bool
    statsSampleInterval time.Duration
}

// VHostOptions extended with stats configuration
type VHostOptions struct {
    // ...existing fields...
    
    EnableStats         bool
    StatsSampleInterval time.Duration
}

// NewVhost creates vhost with optional stats tracking
func NewVhost(vhostName string, options VHostOptions) *VHost {
    ctx, cancel := context.WithCancel(context.Background())
    
    vh := &VHost{
        // ...existing initialization...
        
        ctx:               ctx,
        cancelFunc:        cancel,
        statsEnabled:      options.EnableStats,
        statsSampleInterval: options.StatsSampleInterval,
    }
    
    if vh.statsSampleInterval == 0 {
        vh.statsSampleInterval = 5 * time.Second
    }
    
    // Start background sampler if stats enabled
    if vh.statsEnabled {
        go vh.startStatsSampler()
    }
    
    return vh
}

// startStatsSampler runs background goroutine to sample exchange stats
func (vh *VHost) startStatsSampler() {
    ticker := time.NewTicker(vh.statsSampleInterval)
    defer ticker.Stop()
    
    log.Info().
        Str("vhost", vh.Name).
        Dur("interval", vh.statsSampleInterval).
        Msg("Exchange stats sampler started")
    
    for {
        select {
        case <-vh.ctx.Done():
            log.Info().
                Str("vhost", vh.Name).
                Msg("Exchange stats sampler stopped")
            return
            
        case <-ticker.C:
            vh.sampleAllExchanges()
        }
    }
}

// sampleAllExchanges samples stats for all exchanges
func (vh *VHost) sampleAllExchanges() {
    vh.mu.RLock()
    exchanges := make([]*Exchange, 0, len(vh.Exchanges))
    for _, ex := range vh.Exchanges {
        exchanges = append(exchanges, ex)
    }
    vh.mu.RUnlock()
    
    // Sample without holding vhost lock
    for _, ex := range exchanges {
        ex.SampleStats()
    }
}

// Close gracefully shuts down vhost
func (vh *VHost) Close() {
    if vh.cancelFunc != nil {
        vh.cancelFunc()
    }
}
```

### Phase 4: Configuration

Add to `config/config.go`:

```go
type Config struct {
    // ...existing fields...
    
    // Exchange statistics
    EnableExchangeStats      bool          `env:"OTTERMQ_ENABLE_EXCHANGE_STATS" envDefault:"true"`
    ExchangeStatsSampleInterval time.Duration `env:"OTTERMQ_EXCHANGE_STATS_INTERVAL" envDefault:"5s"`
    ExchangeStatsMaxSamples  int           `env:"OTTERMQ_EXCHANGE_STATS_MAX_SAMPLES" envDefault:"60"`
}
```

Add to `.env.example`:

```bash
# Exchange Statistics Configuration
OTTERMQ_ENABLE_EXCHANGE_STATS=true
OTTERMQ_EXCHANGE_STATS_INTERVAL=5s    # Sample interval
OTTERMQ_EXCHANGE_STATS_MAX_SAMPLES=60 # Keep 5 minutes of history
```

### Phase 5: Management API Integration

Update `internal/core/broker/management/exchange.go`:

```go
package management

import (
    "github.com/andrelcunha/ottermq/internal/core/broker/vhost"
    "github.com/andrelcunha/ottermq/internal/core/models"
)

func (s *Service) exchangeToDTO(vh *vhost.VHost, exchange *vhost.Exchange) models.ExchangeDTO {
    stats := exchange.GetStats()
    
    dto := models.ExchangeDTO{
        VHost:      vh.Name,
        Name:       exchange.Name,
        Type:       exchange.Type,
        Durable:    exchange.Durable,
        AutoDelete: exchange.AutoDelete,
        Internal:   exchange.Internal,
        Arguments:  exchange.Arguments,
    }
    
    // Add stats if available
    if stats.PublishCount > 0 || stats.DeliverCount > 0 {
        dto.MessageStatsIn = &models.MessageStats{
            PublishCount: stats.PublishCount,
            PublishRate:  stats.PublishRate,
        }
        dto.MessageStatsOut = &models.MessageStats{
            DeliverCount: stats.DeliverCount,
            DeliverRate:  stats.DeliverRate,
        }
    }
    
    return dto
}

// GetExchangeHistory returns historical rate data for charting
func (s *Service) GetExchangeHistory(vhostName, exchangeName string) (*models.ExchangeHistoryDTO, error) {
    vh := s.broker.GetVHost(vhostName)
    if vh == nil {
        return nil, fmt.Errorf("vhost not found")
    }
    
    exchange := vh.GetExchange(exchangeName)
    if exchange == nil {
        return nil, fmt.Errorf("exchange not found")
    }
    
    stats := exchange.GetStats()
    
    return &models.ExchangeHistoryDTO{
        VHost:    vhostName,
        Exchange: exchangeName,
        PublishHistory: convertSamplesToTimeSeries(stats.PublishSamples),
        DeliverHistory: convertSamplesToTimeSeries(stats.DeliverSamples),
    }, nil
}

func convertSamplesToTimeSeries(samples []stats.Sample) []models.TimeSeriesPoint {
    points := make([]models.TimeSeriesPoint, len(samples))
    for i, sample := range samples {
        points[i] = models.TimeSeriesPoint{
            Timestamp: sample.Timestamp,
            Value:     sample.Count,
        }
    }
    return points
}
```

### Phase 6: REST API Endpoints

Add to `web/handlers/api/exchanges.go`:

```go
// GET /api/exchanges/{vhost}/{exchange}/history
func (h *ExchangeHandler) GetHistory(c *fiber.Ctx) error {
    vhost := c.Params("vhost")
    exchange := c.Params("exchange")
    
    history, err := h.mgmt.GetExchangeHistory(vhost, exchange)
    if err != nil {
        return c.Status(404).JSON(fiber.Map{"error": err.Error()})
    }
    
    return c.JSON(history)
}
```

### Phase 7: Message Routing Integration

Update `internal/core/broker/vhost/router.go`:

```go
// RouteMessage routes a message through an exchange to queues
func (vh *VHost) RouteMessage(exchangeName string, msg *Message) error {
    exchange := vh.GetExchange(exchangeName)
    if exchange == nil {
        return fmt.Errorf("exchange not found: %s", exchangeName)
    }
    
    // Record publish to exchange
    exchange.RecordPublish()
    
    // Route to queues
    queues := vh.findMatchingQueues(exchange, msg.RoutingKey, msg.Headers)
    
    // Record delivery (one count per target queue)
    exchange.RecordDeliver(len(queues))
    
    // Push to queues
    for _, queue := range queues {
        queue.Push(msg)
    }
    
    return nil
}
```

## Performance Considerations

### Memory Usage

Per exchange with stats enabled:

- **Atomic counters**: 16 bytes (2 × int64)
- **Ring buffer**: ~1.5 KB (60 samples × 24 bytes per sample)
- **Total per exchange**: ~1.5 KB

For 1000 exchanges: **~1.5 MB** of stats memory

### CPU Usage

- **Message routing**: ~5ns per atomic increment (negligible)
- **Background sampling**: 1 goroutine per vhost, wakes every 5s
- **Rate calculation**: O(1) amortized (fixed window size)

### Sampling Interval Trade-offs

| Interval | Pros | Cons |
|----------|------|------|
| 1 second | Responsive, detailed | Higher CPU/memory, noisier |
| 5 seconds | RabbitMQ standard, balanced | Default choice |
| 10 seconds | Lower overhead | Less responsive |

## Testing Strategy

### Unit Tests

1. **RateTracker Tests** (`pkg/stats/rate_tracker_test.go`)
   - Sample recording and pruning
   - Rate calculation accuracy
   - Window boundaries
   - Concurrent access (race detector)

2. **Exchange Stats Tests** (`internal/core/broker/vhost/exchange_test.go`)
   - RecordPublish/RecordDeliver counting
   - SampleStats integration
   - GetStats output format

### Integration Tests

3. **Sampler Tests** (`internal/core/broker/vhost/vhost_stats_test.go`)
   - Background goroutine lifecycle
   - Multiple exchanges sampling
   - Graceful shutdown

### E2E Tests

4. **API Tests** (`tests/e2e/stats_test.go`)
   - Publish messages and verify stats increment
   - Rate calculation over time
   - History endpoint returns valid data

## Migration Path

### Phase 1: Infrastructure Only

- Implement `pkg/stats` package
- Add tests
- **No changes to existing code**

### Phase 2: Exchange Integration (Feature Flag OFF)

- Add stats fields to Exchange
- Wire up RecordPublish/RecordDeliver
- Add config flag (default: false)
- **Stats collection disabled by default**

### Phase 3: Enable for Testing

- Set `OTTERMQ_ENABLE_EXCHANGE_STATS=true` in development
- Validate performance impact
- Test with high traffic scenarios

### Phase 4: Management API

- Add stats to DTOs
- Implement history endpoints
- Document API changes

### Phase 5: UI Integration

- Update ottermq_ui to fetch stats
- Implement rate charts
- Add historical graphs

### Phase 6: Enable by Default

- Set default to `true` after validation
- Document in release notes

## Rollout Checklist

- [ ] Create `pkg/stats` package with RateTracker
- [ ] Add comprehensive unit tests (pkg/stats)
- [ ] Extend Exchange struct with stats fields
- [ ] Add ExchangeOptions for configuration
- [ ] Implement RecordPublish/RecordDeliver/SampleStats
- [ ] Add VHost background sampler goroutine
- [ ] Add config flags (OTTERMQ_ENABLE_EXCHANGE_STATS, etc.)
- [ ] Update exchange creation to use options
- [ ] Wire up RouteMessage to call Record methods
- [ ] Add GetStats to Exchange
- [ ] Update management service to use GetStats
- [ ] Add ExchangeHistoryDTO to models
- [ ] Implement GetExchangeHistory in management
- [ ] Add /api/exchanges/{vhost}/{exchange}/history endpoint
- [ ] Add integration tests
- [ ] Add E2E tests
- [ ] Update documentation (README, API docs)
- [ ] Performance testing (1000+ exchanges, high traffic)
- [ ] Memory profiling
- [ ] Update CHANGELOG.md
- [ ] Update GitHub Pages docs

## Documentation Updates

### README.md

Add to features section:

```markdown
### Exchange Statistics

OtterMQ tracks message rates for all exchanges:

- **Message rate in**: Messages published to exchange per second
- **Message rate out**: Messages routed from exchange to queues per second
- **Historical data**: 5 minutes of rate history for charting
- **RabbitMQ compatible**: Matches management UI behavior

Configuration:

```bash
OTTERMQ_ENABLE_EXCHANGE_STATS=true
OTTERMQ_EXCHANGE_STATS_INTERVAL=5s
OTTERMQ_EXCHANGE_STATS_MAX_SAMPLES=60
\```

```

### CHANGELOG.md

```markdown
## [Unreleased]

### Added
- **Exchange Statistics**: Message rate tracking (in/out) for exchange monitoring
  - Per-exchange publish and delivery counters (atomic, zero-overhead)
  - Ring buffer rate calculation with 5-second sliding window (RabbitMQ-compatible)
  - Configurable stats sampling interval (default: 5s)
  - Historical rate data API for charting (`/api/exchanges/{vhost}/{exchange}/history`)
  - Management API integration for RabbitMQ UI compatibility
  - Feature flag: `OTTERMQ_ENABLE_EXCHANGE_STATS=true` (default: true)

### Performance
- Ring buffer stats: ~1.5 KB memory per exchange, 1 sampling goroutine per vhost
- Atomic counter operations: < 5ns overhead per message publish/deliver
- Stats calculation: O(1) amortized (fixed window size)
```

## Future Enhancements

### v1: Basic Stats (This Document)

- Message rate in/out per exchange
- 5-second sliding window
- Management API integration

### v2: Queue Stats

- Apply same pattern to queues
- Consumer delivery rates
- Queue depth trends

### v3: Connection/Channel Stats

- Per-connection throughput
- Channel-level metrics
- Client profiling

### v4: Advanced Analytics

- Percentile latencies (p50, p95, p99)
- Message size histograms
- Exchange topology graph

### v5: External Metrics

- Prometheus exporter
- StatsD integration
- OpenTelemetry support

## References

- [RabbitMQ Management HTTP API](https://rawcdn.githack.com/rabbitmq/rabbitmq-server/v3.12.0/deps/rabbitmq_management/priv/www/api/index.html)
- [RabbitMQ Management Plugin](https://github.com/rabbitmq/rabbitmq-server/tree/main/deps/rabbitmq_management)
- [Go sync/atomic Package](https://pkg.go.dev/sync/atomic)
- [Keep a Changelog](https://keepachangelog.com/)

## Questions / Decisions Needed

- [ ] Should stats be opt-in or opt-out by default?
  - **Recommendation**: Opt-out (enabled by default) after performance validation
  
- [ ] Should we persist historical samples across restarts?
  - **Recommendation**: No for MVP, evaluate in v2 if users request it
  
- [ ] What sampling interval should be default?
  - **Recommendation**: 5 seconds (matches RabbitMQ)
  
- [ ] Should we support per-exchange configuration?
  - **Recommendation**: No for MVP, global config only

## Contact

For questions or implementation help:

- Create issue: https://github.com/ottermq/ottermq/issues
- Reference this doc: `docs/exchange-stats-implementation.md`

