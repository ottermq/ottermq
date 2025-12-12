# OtterMQ Management Service

The `management` package provides a comprehensive management API layer for the OtterMQ message broker. It exposes broker operations, statistics, and monitoring capabilities through a clean service interface, primarily consumed by the REST API and management UI.

## Overview

This package serves as the **business logic layer** between the broker's core components and external management interfaces (REST API, CLI tools, UI). It provides:

- **Resource Management**: CRUD operations for queues, exchanges, bindings, vhosts
- **Monitoring & Statistics**: Real-time metrics, connection/channel tracking, consumer information
- **Message Operations**: Publishing and consuming messages via management interface
- **Metrics Integration**: Deep integration with [`pkg/metrics`](../../../../pkg/metrics) for observability

## Architecture

### Core Components

```sh
┌─────────────────────────────────────────────────────┐
│                  Web API Handlers                   │
│              (web/handlers/api/*.go)                │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│              Management Service                     │
│              (management.Service)                   │
├─────────────────────────────────────────────────────┤
│  • Queue operations      • Exchange operations      │
│  • Binding management    • Consumer tracking        │
│  • Connection/Channel    • Message operations       │
│  • Overview & Statistics • Metrics integration      │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│              BrokerProvider Interface               │
│          (abstraction over core broker)             │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│          Core Broker & VHost Components             │
│        (vhost.VHost, amqp.Connection, etc.)         │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│              Metrics Collector                      │
│            (pkg/metrics.Collector)                  │
└─────────────────────────────────────────────────────┘
```

### Key Interfaces

#### ManagementService

The primary interface exposing all management operations:

```go
type ManagementService interface {
    // Queues
    ListQueues() []models.QueueDTO
    GetQueue(vhost, queue string) (*models.QueueDTO, error)
    CreateQueue(vhost, queue string, req models.CreateQueueRequest) (*models.QueueDTO, error)
    DeleteQueue(vhost, queue string, ifUnused, ifEmpty bool) error
    PurgeQueue(vhost, queue string) (int, error)
    
    // Exchanges
    ListExchanges() ([]models.ExchangeDTO, error)
    GetExchange(vhost, exchange string) (*models.ExchangeDTO, error)
    CreateExchange(vhost, exchange string, req models.CreateExchangeRequest) (*models.ExchangeDTO, error)
    DeleteExchange(vhost, exchange string, ifUnused bool) error
    
    // Bindings
    ListBindings() ([]models.BindingDTO, error)
    CreateBinding(req models.CreateBindingRequest) (*models.BindingDTO, error)
    DeleteBinding(req models.DeleteBindingRequest) error
    
    // Consumers
    ListConsumers() ([]models.ConsumerDTO, error)
    ListQueueConsumers(vhost, queue string) ([]models.ConsumerDTO, error)
    
    // Connections & Channels
    ListConnections() ([]models.ConnectionInfoDTO, error)
    GetConnection(name string) (*models.ConnectionInfoDTO, error)
    CloseConnection(name string, reason string) error
    ListChannels(vhost string) ([]models.ChannelDetailDTO, error)
    
    // Messages
    PublishMessage(vhost, exchange string, req models.PublishMessageRequest) error
    GetMessages(vhost, queue string, count int, ackMode models.AckType) ([]models.MessageDTO, error)
    
    // VHosts
    ListVHosts() ([]models.VHostDTO, error)
    GetVHost(name string) (*models.VHostDTO, error)
    
    // Overview & Metrics
    GetOverview() (*models.OverviewDTO, error)
    GetBrokerInfo() models.OverviewBrokerDetails
    GetOverviewCharts() (*models.OverviewChartsDTO, error)
}
```

#### BrokerProvider

Abstraction layer defining what the management service needs from the broker:

```go
type BrokerProvider interface {
    GetVHost(vhostName string) *vhost.VHost
    ListVHosts() []*vhost.VHost
    ListConnections() []amqp.ConnectionInfo
    ListChannels(vhost string) ([]models.ChannelInfo, error)
    GetConnectionByName(name string) (*amqp.ConnectionInfo, error)
    CloseConnection(name string, reason string) error
    
    // Overview/Stats methods
    GetOverviewConnStats() models.OverviewConnectionStats
    GetBrokerOverviewConfig() models.BrokerConfigOverview
    GetBrokerOverviewDetails() models.OverviewBrokerDetails
    GetOverviewNodeDetails() models.OverviewNodeDetails
    GetObjectTotalsOverview() models.OverviewObjectTotals
    
    // Metrics integration
    GetCollector() metrics.MetricsCollector
}
```

This interface decouples the management service from the concrete broker implementation, enabling testing and future refactoring.

## Overview & Metrics Integration

The **overview functionality** is the heart of the management UI's dashboard, providing real-time broker statistics and historical time-series data for visualization.

### GetOverview()

Returns a comprehensive snapshot of the broker's current state:

```go
func (s *Service) GetOverview() (*models.OverviewDTO, error) {
    collector := s.broker.GetCollector()
    snapshot := *collector.GetBrokerSnapshot()
    
    return &models.OverviewDTO{
        BrokerDetails:   s.broker.GetBrokerOverviewDetails(),
        NodeDetails:     s.broker.GetOverviewNodeDetails(),
        ObjectTotals:    s.broker.GetObjectTotalsOverview(),
        MessageStats:    s.GetMessageStats(),
        ConnectionStats: s.broker.GetOverviewConnStats(),
        Configuration:   s.broker.GetBrokerOverviewConfig(),
        Metrics:         snapshot,  // ← From pkg/metrics
    }, nil
}
```

**Components:**

- **BrokerDetails**: Version, product name, cluster info
- **NodeDetails**: Uptime, memory usage, file descriptors
- **ObjectTotals**: Count of queues, exchanges, connections, channels, consumers
- **MessageStats**: Breakdown of ready/unacked messages by queue
- **ConnectionStats**: Active connections, channels per connection
- **Configuration**: Queue buffer size, persistence settings
- **Metrics**: Real-time rates and gauges from `pkg/metrics`

### GetMessageStats()

Aggregates message statistics across all queues:

```go
func (s *Service) GetMessageStats() models.OverviewMessageStats {
    var mStats models.OverviewMessageStats
    qStats := []models.QueueMessageBreakdown{}
    dtos := s.ListQueues()
    
    for _, dto := range dtos {
        mStats.MessagesReady += dto.MessagesReady
        mStats.MessagesUnacked += dto.MessagesUnacked
        mStats.MessagesTotal += dto.MessagesTotal
        qStats = append(qStats, models.QueueMessageBreakdown{
            VHost:           dto.VHost,
            QueueName:       dto.Name,
            MessagesReady:   dto.MessagesReady,
            MessagesUnacked: dto.MessagesUnacked,
        })
    }
    
    mStats.QueueStats = qStats
    return mStats
}
```

Provides per-queue breakdown for detailed inspection in the UI.

### GetOverviewCharts()

Returns time-series data for dashboard charts and graphs:

```go
func (s *Service) GetOverviewCharts() (*models.OverviewChartsDTO, error) {
    collector := s.broker.GetCollector()
    bm := collector.GetBrokerMetrics()
    duration := time.Duration(60 * time.Second) // last 60 seconds
    
    return &models.OverviewChartsDTO{
        MessageStats: models.MessageStatsTimeSeriesDTO{
            Ready:   samplesToTimeSeries(bm.TotalReadyDepth().GetSamplesForDuration(duration)),
            Unacked: samplesToTimeSeries(bm.TotalUnackedDepth().GetSamplesForDuration(duration)),
            Total:   samplesToTimeSeries(bm.TotalDepth().GetSamplesForDuration(duration)),
        },
        MessageRates: models.MessageRatesTimeSeriesDTO{
            Publish:          samplesToRates(collector.GetPublishRateTimeSeries(duration)),
            DeliverAutoAck:   samplesToRates(collector.GetDeliveryAutoAckRateTimeSeries(duration)),
            DeliverManualAck: samplesToRates(collector.GetDeliveryManualAckRateTimeSeries(duration)),
            Ack:              samplesToRates(collector.GetAckRateTimeSeries(duration)),
        },
    }, nil
}
```

**Chart Data Includes:**

- **Message Depths** (cumulative): Ready, Unacked, Total messages over time
- **Message Rates** (deltas): Publish rate, delivery rates (auto-ack & manual-ack), ack rate

#### Helper Functions

```go
// samplesToTimeSeries converts raw sample counts to time-series
func samplesToTimeSeries(samples []metrics.Sample) []models.TimeSeriesDTO {
    result := make([]models.TimeSeriesDTO, len(samples))
    for i, s := range samples {
        result[i] = models.TimeSeriesDTO{
            Timestamp: s.Timestamp,
            Value:     float64(s.Count),
        }
    }
    return result
}

// samplesToRates calculates rate-per-second from cumulative counts
func samplesToRates(samples []metrics.Sample) []models.TimeSeriesDTO {
    if len(samples) < 2 {
        return []models.TimeSeriesDTO{}
    }
    
    result := make([]models.TimeSeriesDTO, 0, len(samples)-1)
    for i := 1; i < len(samples); i++ {
        prev := samples[i-1]
        curr := samples[i]
        
        elapsed := curr.Timestamp.Sub(prev.Timestamp).Seconds()
        if elapsed > 0 {
            rate := float64(curr.Count - prev.Count) / elapsed
            result = append(result, models.TimeSeriesDTO{
                Timestamp: curr.Timestamp,
                Value:     rate,
            })
        }
    }
    return result
}
```

The `samplesToRates()` function is critical for displaying **rate charts** in the UI, converting cumulative counters into per-second rates by calculating deltas between consecutive samples.

## Metrics Integration Throughout

The management service integrates with `pkg/metrics` in several key areas:

### 1. Queue Statistics

[queue.go](queue.go):

```go
func (*Service) fetchQueueStatistics(vh *vhost.VHost, queue *vhost.Queue) *QueueStats {
    var messages, unackedCount, consumerCount int
    collector := vh.GetCollector()
    
    if collector != nil && collector.GetQueueSnapshot(queue.Name) != nil {
        snapshot := collector.GetQueueSnapshot(queue.Name)
        messages = int(snapshot.MessageCount)
        unackedCount = int(snapshot.UnackedCount)
        consumerCount = int(snapshot.ConsumerCount)
    } else {
        // Fallback to direct vhost queries
        messages = queue.Len()
        unackedCounts := vh.GetUnackedMessageCountsAllQueues()
        consumerCounts := vh.GetConsumerCountsAllQueues()
        unackedCount = unackedCounts[queue.Name]
        consumerCount = consumerCounts[queue.Name]
    }
    return &QueueStats{
        Messages:      messages,
        UnackedCount:  unackedCount,
        ConsumerCount: consumerCount,
    }
}
```

**Why Metrics?** Direct queue queries can be slow and require locks. The metrics collector provides lock-free, cached snapshots for fast API responses.

### 2. Exchange Statistics

[exchange.go](exchange.go):

```go
func (s *Service) exchangeToDTO(vh *vhost.VHost, exchange *vhost.Exchange) *models.ExchangeDTO {
    var messageStatsIn, messageStatsOut *models.MessageStats
    
    if snapshot := s.broker.GetCollector().GetExchangeSnapshot(exchange.NameOrAlias()); snapshot != nil {
        messageStatsIn = &models.MessageStats{
            PublishCount: int(snapshot.PublishCount),
            PublishRate:  snapshot.PublishRate,
        }
        messageStatsOut = &models.MessageStats{
            DeliverCount: snapshot.DeliveryCount,
            DeliverRate:  snapshot.DeliveryRate,
        }
    } else {
        messageStatsIn = &models.MessageStats{}
        messageStatsOut = &models.MessageStats{}
    }
    
    return &models.ExchangeDTO{
        Name:            exchange.NameOrAlias(),
        Type:            string(exchange.Typ),
        MessageStatsIn:  messageStatsIn,
        MessageStatsOut: messageStatsOut,
        // ... other fields
    }
}
```

Enriches exchange DTOs with publish/delivery rates and counts from the metrics collector.

### 3. Broker-Level Metrics

[overview.go](overview.go):

The entire `GetOverview()` and `GetOverviewCharts()` rely on the metrics collector to provide:

- **Current rates**: publish/sec, delivery/sec, ack/sec
- **Gauges**: connection count, channel count, message count
- **Time-series**: historical samples for charting

## Usage Examples

### Initializing the Service

```go
import (
    "github.com/andrelcunha/ottermq/internal/core/broker/management"
)

// broker implements BrokerProvider interface
service := management.NewService(broker)

// Service is now ready for management operations
```

### Queue Management

```go
// List all queues
queues := service.ListQueues()
for _, q := range queues {
    fmt.Printf("Queue: %s, Messages: %d, Consumers: %d\n", 
        q.Name, q.MessagesTotal, q.Consumers)
}

// Create a queue with DLX and TTL
createReq := models.CreateQueueRequest{
    Durable:    true,
    AutoDelete: false,
    Arguments: map[string]any{
        "x-message-ttl":            300000, // 5 minutes
        "x-dead-letter-exchange":   "dlx",
        "x-dead-letter-routing-key": "dead.messages",
        "x-max-length":             10000,
    },
}
dto, err := service.CreateQueue("/", "my-queue", createReq)
if err != nil {
    log.Fatal(err)
}

// Get queue details
queue, err := service.GetQueue("/", "my-queue")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Ready: %d, Unacked: %d\n", queue.MessagesReady, queue.MessagesUnacked)

// Purge queue
count, err := service.PurgeQueue("/", "my-queue")
fmt.Printf("Purged %d messages\n", count)

// Delete queue
err = service.DeleteQueue("/", "my-queue", false, false)
```

### Exchange Management

```go
// Create an exchange
exchangeReq := models.CreateExchangeRequest{
    ExchangeType: "topic",
    Durable:      true,
    AutoDelete:   false,
}
dto, err := service.CreateExchange("/", "my-topic-exchange", exchangeReq)

// List all exchanges
exchanges, err := service.ListExchanges()
for _, ex := range exchanges {
    fmt.Printf("Exchange: %s (%s), Publish Rate: %.2f msg/s\n",
        ex.Name, ex.Type, ex.MessageStatsIn.PublishRate)
}

// Get exchange details
exchange, err := service.GetExchange("/", "my-topic-exchange")
fmt.Printf("Published: %d, Delivered: %d\n",
    exchange.MessageStatsIn.PublishCount,
    exchange.MessageStatsOut.DeliverCount)

// Delete exchange
err = service.DeleteExchange("/", "my-topic-exchange", true) // ifUnused=true
```

### Binding Management

```go
// Create a binding
bindReq := models.CreateBindingRequest{
    VHost:       "/",
    Source:      "my-topic-exchange",
    Destination: "my-queue",
    RoutingKey:  "events.#",
    Arguments:   nil,
}
binding, err := service.CreateBinding(bindReq)

// List bindings for a queue
bindings, err := service.ListQueueBindings("/", "my-queue")
for _, b := range bindings {
    fmt.Printf("Binding: %s -> %s [%s]\n", b.Source, b.Destination, b.RoutingKey)
}

// Delete binding
deleteReq := models.DeleteBindingRequest{
    VHost:       "/",
    Source:      "my-topic-exchange",
    Destination: "my-queue",
    RoutingKey:  "events.#",
}
err = service.DeleteBinding(deleteReq)
```

### Consumer Tracking

```go
// List all consumers
consumers, err := service.ListConsumers()
for _, c := range consumers {
    fmt.Printf("Consumer: %s, Queue: %s, Prefetch: %d\n",
        c.ConsumerTag, c.QueueName, c.PrefetchCount)
}

// List consumers for a specific queue
queueConsumers, err := service.ListQueueConsumers("/", "my-queue")
for _, c := range queueConsumers {
    fmt.Printf("Tag: %s, Channel: %d, Ack Required: %t\n",
        c.ConsumerTag, c.ChannelDetails.Number, c.AckRequired)
}
```

### Connection & Channel Management

```go
// List all connections
connections, err := service.ListConnections()
for _, conn := range connections {
    fmt.Printf("Connection: %s, User: %s, Channels: %d\n",
        conn.Name, conn.User, conn.Channels)
}

// Get connection details
conn, err := service.GetConnection("127.0.0.1:54321")
fmt.Printf("State: %s, VHost: %s, Sent: %d bytes\n",
    conn.State, conn.VHost, conn.SendOct)

// Close connection
err = service.CloseConnection("127.0.0.1:54321", "Admin requested close")

// List channels in vhost
channels, err := service.ListChannels("/")
for _, ch := range channels {
    fmt.Printf("Channel %d, Connection: %s, Prefetch: %d\n",
        ch.Number, ch.ConnectionName, ch.PrefetchCount)
}
```

### Message Operations

```go
// Publish a message via management API
pubReq := models.PublishMessageRequest{
    RoutingKey:   "test.message",
    Payload:      "Hello from management API",
    ContentType:  "text/plain",
    DeliveryMode: 2, // persistent
    Priority:     5,
    Headers: map[string]any{
        "X-Custom-Header": "value",
    },
}
err := service.PublishMessage("/", "my-exchange", pubReq)

// Get messages from a queue (for inspection)
messages, err := service.GetMessages("/", "my-queue", 10, models.AckRequeue)
for _, msg := range messages {
    fmt.Printf("Message ID: %s, Delivery Tag: %d, Payload: %s\n",
        msg.ID, msg.DeliveryTag, string(msg.Payload))
}
```

### Overview & Metrics

```go
// Get broker overview
overview, err := service.GetOverview()
fmt.Printf("Broker: %s %s\n", overview.BrokerDetails.Product, overview.BrokerDetails.Version)
fmt.Printf("Uptime: %d seconds\n", overview.NodeDetails.Uptime)
fmt.Printf("Queues: %d, Exchanges: %d, Connections: %d\n",
    overview.ObjectTotals.Queues,
    overview.ObjectTotals.Exchanges,
    overview.ObjectTotals.Connections)
fmt.Printf("Total Messages: Ready=%d, Unacked=%d\n",
    overview.MessageStats.MessagesReady,
    overview.MessageStats.MessagesUnacked)

// Current rates from metrics
metrics := overview.Metrics
fmt.Printf("Publish Rate: %.2f msg/s\n", metrics.PublishRate)
fmt.Printf("Delivery Rate: %.2f msg/s\n", metrics.DeliveryRate)
fmt.Printf("Active Connections: %d\n", metrics.ConnectionCount)

// Get time-series data for charts
charts, err := service.GetOverviewCharts()

// Message depth time-series (last 60 seconds)
for _, sample := range charts.MessageStats.Total {
    fmt.Printf("Time: %s, Total Messages: %.0f\n",
        sample.Timestamp.Format("15:04:05"), sample.Value)
}

// Publish rate time-series
for _, sample := range charts.MessageRates.Publish {
    fmt.Printf("Time: %s, Publish Rate: %.2f msg/s\n",
        sample.Timestamp.Format("15:04:05"), sample.Value)
}
```

## Integration with Web API

The management service is consumed by the REST API handlers in [`web/handlers/api/`](../../../../web/handlers/api/):

```go
// In web API handler
func (h *Handler) GetOverview(c *fiber.Ctx) error {
    overview, err := h.managementService.GetOverview()
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }
    return c.JSON(overview)
}

func (h *Handler) GetOverviewCharts(c *fiber.Ctx) error {
    charts, err := h.managementService.GetOverviewCharts()
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }
    return c.JSON(charts)
}
```

The UI polls these endpoints periodically to update dashboard charts and statistics in real-time.

## Testing

The management service is designed for testability:

```go
// Mock the BrokerProvider interface
type MockBrokerProvider struct {
    vhosts    map[string]*vhost.VHost
    collector *metrics.MockCollector
}

func (m *MockBrokerProvider) GetVHost(name string) *vhost.VHost {
    return m.vhosts[name]
}

func (m *MockBrokerProvider) GetCollector() metrics.MetricsCollector {
    return m.collector
}

// ... implement other interface methods

// Use in tests
func TestGetQueue(t *testing.T) {
    mockBroker := &MockBrokerProvider{
        vhosts: map[string]*vhost.VHost{
            "/": testVHost,
        },
        collector: metrics.NewMockCollector(),
    }
    
    service := management.NewService(mockBroker)
    queue, err := service.GetQueue("/", "test-queue")
    
    assert.NoError(t, err)
    assert.Equal(t, "test-queue", queue.Name)
}
```

See [`*_test.go`](.) files for comprehensive test examples.

## File Structure

```sh
management/
├── service.go             # ManagementService interface & Service struct
├── broker_provider.go     # BrokerProvider interface (abstraction)
├── overview.go            # Overview & metrics integration ⭐
├── queue.go               # Queue management operations
├── exchange.go            # Exchange management operations
├── binding.go             # Binding management operations
├── consumer.go            # Consumer tracking
├── connection.go          # Connection management
├── channel.go             # Channel listing
├── message.go             # Message publish/get operations
├── vhost.go               # VHost operations
└── *_test.go              # Test files
```

## Key Design Patterns

### 1. DTO Mapping

All internal broker types are mapped to DTOs (Data Transfer Objects) defined in [`internal/core/models`](../models/):

- `*vhost.Queue` → `models.QueueDTO`
- `*vhost.Exchange` → `models.ExchangeDTO`
- `amqp.ConnectionInfo` → `models.ConnectionInfoDTO`
- etc.

This decouples the API layer from internal broker structures.

### 2. Metrics-First Approach

The service **prefers metrics snapshots** over direct queries:

```go
// Try metrics first (fast, lock-free)
if snapshot := collector.GetQueueSnapshot(queueName); snapshot != nil {
    return snapshot.MessageCount
}

// Fallback to direct query (slower, requires locks)
return queue.Len()
```

This ensures the management API remains performant even under heavy load.

### 3. Error Handling

- **Idempotent deletes**: Deleting non-existent resources returns `nil` (success)
- **Conditional operations**: `ifUnused`, `ifEmpty` flags prevent accidental deletions
- **Validation**: Request validation before broker operations

### 4. MANAGEMENT_CONNECTION_ID

Operations originating from the management API use a special connection ID:

```go
vhost.MANAGEMENT_CONNECTION_ID
```

This allows the broker to differentiate management operations from client AMQP connections for:

- Exclusive queue handling (management can create non-exclusive queues)
- Audit logging
- Permission checks

## Performance Considerations

- **Metrics snapshots** avoid expensive queue traversals
- **Lock-free reads** for most metrics operations
- **Batched queries**: `ListQueues()` fetches all queues in one pass
- **Lazy evaluation**: Statistics only computed when requested

## Dependencies

- [`pkg/metrics`](../../../../pkg/metrics): Metrics collection and time-series tracking
- [`internal/core/models`](../models/): DTO definitions
- [`internal/core/broker/vhost`](../vhost/): VHost and queue/exchange core logic
- [`internal/core/amqp`](../../amqp/): AMQP protocol types and connection info

## Future Enhancements

- **Permissions & ACLs**: User-based access control for management operations
- **Audit logging**: Track all management API calls
- **Rate limiting**: Prevent API abuse
- **Bulk operations**: Batch create/delete for multiple resources
- **Queue/Exchange statistics history**: Persistent storage of historical metrics
- **Alerting**: Trigger alerts based on queue depth, consumer count, etc.

## See Also

- [Metrics Package README](../../../../pkg/metrics/README.md) - Deep dive into metrics collection
- [Web API Documentation](../../../../web/) - REST API handlers using this service
- [VHost Package](../vhost/) - Core broker logic
- [AMQP Package](../../amqp/) - Protocol implementation
