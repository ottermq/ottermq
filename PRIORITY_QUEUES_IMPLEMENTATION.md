# Priority Queues - Implementation Guide

**Status**: 🚧 In Progress  
**Created**: December 4, 2025  
**Target Completion**: December 18, 2025  
**Priority**: High  
**Effort**: 2 weeks  

---

## 📋 Executive Summary

This document outlines the implementation of **Priority Queues** for OtterMQ. Priority queues are a core AMQP 0.9.1 feature that allows messages to be delivered in priority order rather than strict FIFO order.

### What Are Priority Queues?

Priority queues allow messages to be ordered by priority level (0-9 or 0-255) before delivery to consumers:

- **Higher priority messages** are delivered before lower priority messages
- **Within the same priority level**, FIFO order is maintained
- Messages without explicit priority are treated as priority 0 (lowest)

### Goals

1. ✅ **AMQP 0.9.1 Compliance**: Implement priority field as specified in the protocol
2. ✅ **RabbitMQ Compatibility**: Support `x-max-priority` queue argument (1-255 range)
3. ✅ **Performance Optimized**: Limit to 10 priority levels by default (configurable)
4. ✅ **Map-of-Channels Architecture**: Use proven Go channel semantics with priority ordering
5. ✅ **Backward Compatible**: Queues without `x-max-priority` remain FIFO (no breaking changes)

### Architecture Decision

**Implementation Strategy**: Map of channels indexed by priority level

```go
type Queue struct {
    // Non-priority queues (current, backward compatible)
    messages chan Message
    
    // Priority queues (new)
    priorityMessages map[uint8]chan Message
    maxPriority      uint8
    messageSignal    chan struct{}  // Signal when any priority has messages
}
```

**Benefits**:

- ✅ Reuses Go's proven channel semantics
- ✅ Automatic FIFO within each priority level
- ✅ Simple to implement and reason about
- ✅ Memory efficient with lazy channel allocation
- ✅ Fast path for non-priority queues (zero overhead)

---

## 🎯 Goals & Benefits

### Goals

1. **AMQP Compliance**: Proper `priority` field support in BasicProperties
2. **Queue Configuration**: `x-max-priority` argument during queue declaration
3. **Priority Range**: Support 0-255 (RabbitMQ compatible) with configurable limit (default: 10)
4. **Delivery Order**: Higher priority messages delivered first, FIFO within priority
5. **Backward Compatible**: Existing queues and tests continue to work unchanged
6. **Performance**: Minimal overhead for non-priority queues

### Benefits

- 🚀 **Better Message Prioritization**: Critical messages delivered before routine ones
- 📊 **Production Ready**: Common pattern for task queues (high/normal/low priority)
- 🔄 **RabbitMQ Compatible**: Existing RabbitMQ clients work without modification
- 🛡️ **Resource Control**: Configurable max priority limits memory usage
- 🧪 **Testable**: Clean separation between priority and non-priority paths

---

## 📂 File Structure

### Files to Create

```code
docs/
└── priority-queues.md              # User-facing documentation

internal/core/broker/vhost/
├── priority.go                     # Priority queue implementation
└── priority_test.go                # Priority queue unit tests

tests/e2e/
└── priority_test.go                # End-to-end priority tests
```

### Files to Modify

```code
config/
├── config.go                       # Add MaxPriority configuration
└── config_test.go                  # Test priority configuration

internal/core/broker/
└── broker.go                       # Pass MaxPriority to VHost

internal/core/broker/vhost/
├── vhost.go                        # Add MaxPriority to VHostOptions
├── queue.go                        # Update Queue struct and methods
├── message.go                      # Clamp priority on publish
└── helpers.go                      # Add parseMaxPriorityArgument

internal/core/models/
└── dto.go                          # Add MaxPriority to QueueDTO

web/handlers/api/
└── queues.go                       # Display max-priority in API

ROADMAP.md                          # Update implementation status
CHANGELOG.md                        # Document feature in next release
```

---

## 📊 Configuration

### 1. Add to Config Struct

**File**: `config/config.go`

```go
type Config struct {
    // ... existing fields ...
    
    // AMQP Settings
    HeartbeatIntervalMax uint16
    ChannelMax           uint16
    FrameMax             uint32
    Version              string
    Ssl                  bool
    QueueBufferSize      int
    MaxPriority          uint8  // NEW: Maximum priority level supported (default: 10)
    
    // ... rest of config ...
}
```

### 2. Load from Environment

**File**: `config/config.go` (in LoadConfig function)

```go
func LoadConfig(version string) *Config {
    _ = godotenv.Load()
    
    return &Config{
        // ... existing fields ...
        
        QueueBufferSize:      getEnvAsInt("OTTERMQ_QUEUE_BUFFER_SIZE", 100000),
        MaxPriority:          getEnvAsUint8WithMax("OTTERMQ_MAX_PRIORITY", 10, 255),  // NEW
        
        // ... rest of config ...
    }
}
```

### 3. Add Helper Function

**File**: `config/config.go`

```go
// getEnvAsUint8WithMax parses uint8 with validation and max value clamping
func getEnvAsUint8WithMax(key string, defaultValue, maxValue uint8) uint8 {
    valueStr := os.Getenv(key)
    if valueStr == "" {
        return defaultValue
    }
    value, err := strconv.ParseUint(valueStr, 10, 8)
    if err != nil {
        fmt.Printf("Warning: Invalid value for %s: %s, using default: %d\n", key, valueStr, defaultValue)
        return defaultValue
    }
    
    result := uint8(value)
    if result > maxValue {
        fmt.Printf("Warning: Value for %s (%d) exceeds maximum (%d), clamping to max\n", key, result, maxValue)
        return maxValue
    }
    
    return result
}
```

### 4. Configuration Tests

**File**: `config/config_test.go`

```go
func TestMaxPriorityConfiguration(t *testing.T) {
    tests := []struct {
        name     string
        envValue string
        expected uint8
    }{
        {"default", "", 10},
        {"valid low", "3", 3},
        {"valid high", "10", 10},
        {"max allowed", "255", 255},
        {"exceeds max", "300", 255},  // Should clamp
        {"invalid", "abc", 10},       // Should fallback
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            os.Clearenv()
            if tt.envValue != "" {
                os.Setenv("OTTERMQ_MAX_PRIORITY", tt.envValue)
            }
            
            config := LoadConfig("test")
            if config.MaxPriority != tt.expected {
                t.Errorf("Expected MaxPriority=%d, got %d", tt.expected, config.MaxPriority)
            }
        })
    }
}
```

### 5. Environment Variable Documentation

**File**: `README.md` and `.env.example`

```env
# Priority Queue Configuration
OTTERMQ_MAX_PRIORITY=10              # Maximum priority level (1-255, default: 10)
                                     # Recommended: 3-10 for performance
                                     # Higher values use more memory per queue
```

---

## 🔧 Priority Queue Implementation

### 1. Queue Struct Updates

**File**: `internal/core/broker/vhost/queue.go`

```go
type Queue struct {
    Name      string           `json:"name"`
    Props     *QueueProperties `json:"properties"`
    
    // FIFO queue (non-priority, backward compatible)
    messages  chan Message     `json:"-"`
    
    // Priority queue (when x-max-priority > 0)
    priorityMessages map[uint8]chan Message `json:"-"`
    maxPriority      uint8                  `json:"-"`
    messageSignal    chan struct{}          `json:"-"`  // Unblocks delivery loop
    
    count     int              `json:"-"`
    mu        sync.Mutex       `json:"-"`
    OwnerConn ConnectionID     `json:"-"`
    
    vh *VHost `json:"-"`
    
    // ... rest of fields ...
}
```

### 2. NewQueue Constructor

**File**: `internal/core/broker/vhost/queue.go`

```go
func NewQueue(name string, bufferSize int, vh *VHost) *Queue {
    return &Queue{
        Name:             name,
        Props:            &QueueProperties{},
        messages:         make(chan Message, bufferSize),  // Default FIFO
        priorityMessages: nil,                              // Lazy init
        maxPriority:      0,                                // No priority by default
        messageSignal:    nil,                              // Lazy init
        count:            0,
        delivering:       false,
        maxLength:        0,
        vh:               vh,
    }
}
```

### 3. Parse x-max-priority Argument

**File**: `internal/core/broker/vhost/helpers.go`

```go
// parseMaxPriorityArgument extracts x-max-priority from queue arguments
func parseMaxPriorityArgument(args map[string]interface{}) (uint8, bool) {
    maxPri, ok := args["x-max-priority"]
    if !ok {
        return 0, false
    }
    
    // Convert various numeric types to uint8
    value, ok := convertToPositiveInt64(maxPri)
    if !ok || value <= 0 {
        return 0, false
    }
    
    // Clamp to uint8 range (0-255)
    if value > 255 {
        value = 255
    }
    
    return uint8(value), true
}
```

### 4. Initialize Priority Queue

**File**: `internal/core/broker/vhost/queue.go` (in CreateQueue)

```go
func (vh *VHost) CreateQueue(name string, props *QueueProperties, connID ConnectionID) (*Queue, error) {
    // ... existing queue creation logic ...
    
    queue := NewQueue(name, vh.queueBufferSize, vh)
    queue.Props = props
    
    // NEW: Check for x-max-priority argument
    if maxPri, ok := parseMaxPriorityArgument(props.Arguments); ok {
        // Clamp to configured maximum
        if maxPri > vh.maxPriority {
            log.Warn().
                Str("queue", name).
                Uint8("requested", maxPri).
                Uint8("max", vh.maxPriority).
                Msg("x-max-priority exceeds configured maximum, clamping")
            maxPri = vh.maxPriority
        }
        
        if maxPri > 0 {
            log.Debug().
                Str("queue", name).
                Uint8("max_priority", maxPri).
                Msg("Initializing priority queue")
            
            queue.maxPriority = maxPri
            queue.priorityMessages = make(map[uint8]chan Message)
            queue.messageSignal = make(chan struct{}, 1)  // Buffered signal
            // Channels are lazily allocated on first push to each priority
        }
    }
    
    // ... rest of queue creation ...
}
```

### 5. Priority-Aware Push

**File**: `internal/core/broker/vhost/queue.go`

```go
func (q *Queue) Push(msg Message) {
    // QLL enforcement (existing)
    if q.maxLength > 0 && q.vh != nil {
        q.vh.QueueLengthLimiter.EnforceMaxLength(q)
    }
    
    // Route to priority or FIFO queue
    if q.maxPriority > 0 {
        q.pushPriority(msg)
    } else {
        q.push(msg)  // Existing FIFO implementation
    }
}

func (q *Queue) pushPriority(msg Message) {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    // Clamp priority to max
    priority := msg.Properties.Priority
    if priority > q.maxPriority {
        priority = q.maxPriority
    }
    
    // Lazy channel allocation
    ch, exists := q.priorityMessages[priority]
    if !exists {
        // Divide buffer among priorities
        bufferPerPriority := q.vh.queueBufferSize / int(q.maxPriority+1)
        if bufferPerPriority < 100 {
            bufferPerPriority = 100  // Minimum buffer
        }
        ch = make(chan Message, bufferPerPriority)
        q.priorityMessages[priority] = ch
        
        log.Debug().
            Str("queue", q.Name).
            Uint8("priority", priority).
            Int("buffer", bufferPerPriority).
            Msg("Allocated priority channel")
    }
    
    // Non-blocking push
    select {
    case ch <- msg:
        q.count++
        log.Debug().
            Str("queue", q.Name).
            Str("id", msg.ID).
            Uint8("priority", priority).
            Msg("Pushed message to priority queue")
        
        // Signal delivery loop (non-blocking)
        select {
        case q.messageSignal <- struct{}{}:
        default:
        }
    default:
        log.Debug().
            Str("queue", q.Name).
            Str("id", msg.ID).
            Uint8("priority", priority).
            Msg("Priority channel full, dropping message")
    }
}
```

### 6. Priority-Aware Pop

**File**: `internal/core/broker/vhost/queue.go`

```go
func (q *Queue) Pop() *Message {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    if q.maxPriority > 0 {
        return q.popPriorityUnlocked()
    }
    return q.popUnlocked()  // Existing FIFO implementation
}

func (q *Queue) popPriorityUnlocked() *Message {
    // Try from highest priority down to 0
    for p := q.maxPriority; ; p-- {
        if ch, exists := q.priorityMessages[p]; exists {
            select {
            case msg := <-ch:
                q.count--
                log.Debug().
                    Str("queue", q.Name).
                    Str("id", msg.ID).
                    Uint8("priority", p).
                    Msg("Popped message from priority queue")
                return &msg
            default:
                // This priority is empty, try next
            }
        }
        
        if p == 0 {
            break  // Checked all priorities
        }
    }
    
    log.Debug().Str("queue", q.Name).Msg("Priority queue is empty")
    return nil
}
```

### 7. Delivery Loop Updates

**File**: `internal/core/broker/vhost/queue.go`

```go
func (q *Queue) startDeliveryLoop(vh *VHost) {
    // ... existing setup ...
    
    go func() {
        defer q.deliveryWg.Done()
        
        for {
            select {
            case <-q.deliveryCtx.Done():
                log.Debug().Str("queue", q.Name).Msg("Stopping delivery loop")
                return
                
            default:
                // Priority queue: wait for signal then pop
                if q.maxPriority > 0 {
                    q.deliverFromPriorityQueue(vh)
                } else {
                    q.deliverFromFIFOQueue(vh)
                }
            }
        }
    }()
}

func (q *Queue) deliverFromPriorityQueue(vh *VHost) {
    select {
    case <-q.messageSignal:
        // Try to pop a message
        msg := q.Pop()
        if msg == nil {
            return  // Race condition: message was consumed elsewhere
        }
        
        // Existing delivery logic
        q.deliverMessage(vh, *msg)
        
    case <-q.deliveryCtx.Done():
        return
        
    case <-time.After(100 * time.Millisecond):
        // Periodic check for shutdown
        return
    }
}

func (q *Queue) deliverFromFIFOQueue(vh *VHost) {
    select {
    case msg := <-q.messages:
        q.mu.Lock()
        q.count--
        q.mu.Unlock()
        q.deliverMessage(vh, msg)
        
    case <-q.deliveryCtx.Done():
        return
    }
}

func (q *Queue) deliverMessage(vh *VHost, msg Message) {
    // Extract existing delivery logic into shared method
    // ... TTL check, consumer selection, delivery, requeue, etc ...
}
```

### 8. Purge Support

**File**: `internal/core/broker/vhost/queue.go`

```go
func (q *Queue) StreamPurge(process func(*Message)) uint32 {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    var purged uint32 = 0
    
    if q.maxPriority > 0 {
        // Purge priority queues (highest to lowest)
        for p := q.maxPriority; ; p-- {
            if ch, exists := q.priorityMessages[p]; exists {
                for {
                    select {
                    case msg := <-ch:
                        q.count--
                        process(&msg)
                        purged++
                    default:
                        goto nextPriority
                    }
                }
            }
        nextPriority:
            if p == 0 {
                break
            }
        }
    } else {
        // Existing FIFO purge
        for {
            msg := q.popUnlocked()
            if msg == nil {
                break
            }
            process(msg)
            purged++
        }
    }
    
    return purged
}
```

---

## 🌐 VHost Integration

### 1. Add MaxPriority to VHostOptions

**File**: `internal/core/broker/vhost/vhost.go`

```go
type VHostOptions struct {
    QueueBufferSize int
    Persistence     persistence.Persistence
    EnableDLX       bool
    EnableTTL       bool
    EnableQLL       bool
    MaxPriority     uint8  // NEW: Maximum priority level (default from config)
}
```

### 2. Store MaxPriority in VHost

**File**: `internal/core/broker/vhost/vhost.go`

```go
type VHost struct {
    Name               string
    Id                 string
    Exchanges          map[string]*Exchange
    Queues             map[string]*Queue
    // ... existing fields ...
    
    queueBufferSize     int
    maxPriority         uint8  // NEW: Max priority from config
    
    // ... rest of fields ...
}

func NewVhost(vhostName string, options VHostOptions) *VHost {
    vh := &VHost{
        // ... existing initialization ...
        queueBufferSize: options.QueueBufferSize,
        maxPriority:     options.MaxPriority,  // NEW
        // ... rest of initialization ...
    }
    // ... rest of function ...
}
```

### 3. Pass Config to VHost

**File**: `internal/core/broker/broker.go`

```go
func NewBroker(config *config.Config, rootCtx context.Context, rootCancel context.CancelFunc) *Broker {
    // ... existing setup ...
    
    options := vhost.VHostOptions{
        QueueBufferSize: config.QueueBufferSize,
        Persistence:     persist,
        EnableDLX:       config.EnableDLX,
        EnableTTL:       config.EnableTTL,
        EnableQLL:       config.EnableQLL,
        MaxPriority:     config.MaxPriority,  // NEW
    }
    
    // ... rest of function ...
}
```

---

## 📊 Data Model Updates

### Queue DTO Enhancement

**File**: `internal/core/models/dto.go`

```go
type QueueDTO struct {
    // ... existing fields ...
    
    // Queue Length Limit (QLL AKA Max Length)
    MaxLength *int32 `json:"max_length,omitempty"`
    
    // Priority Queue Configuration (NEW)
    MaxPriority *uint8 `json:"max_priority,omitempty"`  // x-max-priority
    
    // State
    State string `json:"state"` // "running", "idle", "flow"
    
    // ... rest of fields ...
}
```

### Management Service Updates

**File**: `internal/core/broker/management/queues.go`

```go
func (s *Service) queueToDTO(vh *vhost.VHost, queue *vhost.Queue, unackedCount, consumerCount int) models.QueueDTO {
    // ... existing DTO conversion ...
    
    // Extract max-priority (NEW)
    if queue.Props.Arguments != nil {
        if maxPri, ok := queue.Props.Arguments["x-max-priority"]; ok {
            switch v := maxPri.(type) {
            case uint8:
                dto.MaxPriority = &v
            case int:
                val := uint8(v)
                dto.MaxPriority = &val
            case int32:
                val := uint8(v)
                dto.MaxPriority = &val
            }
        }
    }
    
    return dto
}
```

---

## 🧪 Testing Strategy

### 1. Unit Tests

**File**: `internal/core/broker/vhost/priority_test.go`

```go
package vhost

import (
    "testing"
    "github.com/andrelcunha/ottermq/internal/core/amqp"
)

func TestPriorityQueue_PushPop_Order(t *testing.T) {
    vh := createTestVHostWithPriority(10)
    queue := createTestQueueWithMaxPriority(vh, "test-priority-queue", 10)
    
    // Push messages with different priorities
    pushMessageWithPriority(queue, "low", 1)
    pushMessageWithPriority(queue, "high", 9)
    pushMessageWithPriority(queue, "medium", 5)
    
    // Pop should return in priority order
    assertNextMessage(t, queue, "high")
    assertNextMessage(t, queue, "medium")
    assertNextMessage(t, queue, "low")
}

func TestPriorityQueue_FIFO_WithinPriority(t *testing.T) {
    vh := createTestVHostWithPriority(10)
    queue := createTestQueueWithMaxPriority(vh, "test-queue", 10)
    
    // Push multiple messages with same priority
    pushMessageWithPriority(queue, "first", 5)
    pushMessageWithPriority(queue, "second", 5)
    pushMessageWithPriority(queue, "third", 5)
    
    // Should maintain FIFO within priority
    assertNextMessage(t, queue, "first")
    assertNextMessage(t, queue, "second")
    assertNextMessage(t, queue, "third")
}

func TestPriorityQueue_PriorityClamp(t *testing.T) {
    vh := createTestVHostWithPriority(10)
    queue := createTestQueueWithMaxPriority(vh, "test-queue", 5)
    
    // Push with priority > max
    pushMessageWithPriority(queue, "clamped", 10)  // Should clamp to 5
    pushMessageWithPriority(queue, "normal", 3)
    
    // Clamped message should be delivered first (priority 5)
    assertNextMessage(t, queue, "clamped")
    assertNextMessage(t, queue, "normal")
}

func TestPriorityQueue_NoPriorityDefaultsToZero(t *testing.T) {
    vh := createTestVHostWithPriority(10)
    queue := createTestQueueWithMaxPriority(vh, "test-queue", 10)
    
    pushMessageWithPriority(queue, "priority-5", 5)
    pushMessageNoPriority(queue, "no-priority")  // Should be priority 0
    
    assertNextMessage(t, queue, "priority-5")
    assertNextMessage(t, queue, "no-priority")
}

func TestPriorityQueue_LazyChannelAllocation(t *testing.T) {
    vh := createTestVHostWithPriority(10)
    queue := createTestQueueWithMaxPriority(vh, "test-queue", 10)
    
    // Initially no channels allocated
    if len(queue.priorityMessages) != 0 {
        t.Errorf("Expected 0 allocated channels, got %d", len(queue.priorityMessages))
    }
    
    // Push to priority 5
    pushMessageWithPriority(queue, "msg", 5)
    
    // Should allocate only priority 5 channel
    if len(queue.priorityMessages) != 1 {
        t.Errorf("Expected 1 allocated channel, got %d", len(queue.priorityMessages))
    }
    if _, exists := queue.priorityMessages[5]; !exists {
        t.Error("Expected priority 5 channel to be allocated")
    }
}

func TestPriorityQueue_Purge(t *testing.T) {
    vh := createTestVHostWithPriority(10)
    queue := createTestQueueWithMaxPriority(vh, "test-queue", 10)
    
    // Push messages at different priorities
    pushMessageWithPriority(queue, "p9", 9)
    pushMessageWithPriority(queue, "p5", 5)
    pushMessageWithPriority(queue, "p1", 1)
    
    if queue.Len() != 3 {
        t.Errorf("Expected queue length 3, got %d", queue.Len())
    }
    
    // Purge
    purged := queue.StreamPurge(func(msg *Message) {})
    
    if purged != 3 {
        t.Errorf("Expected 3 purged messages, got %d", purged)
    }
    if queue.Len() != 0 {
        t.Errorf("Expected queue length 0 after purge, got %d", queue.Len())
    }
}

func TestNonPriorityQueue_BackwardCompatible(t *testing.T) {
    vh := createTestVHostWithPriority(10)
    queue := createTestQueue(vh, "fifo-queue")  // No x-max-priority
    
    // Should use FIFO channel
    if queue.maxPriority != 0 {
        t.Errorf("Expected maxPriority=0, got %d", queue.maxPriority)
    }
    if queue.priorityMessages != nil {
        t.Error("Expected nil priorityMessages for FIFO queue")
    }
    
    // Should work as before
    pushMessageNoPriority(queue, "first")
    pushMessageNoPriority(queue, "second")
    
    assertNextMessage(t, queue, "first")
    assertNextMessage(t, queue, "second")
}

// Helper functions
func createTestVHostWithPriority(maxPri uint8) *VHost {
    options := VHostOptions{
        QueueBufferSize: 1000,
        MaxPriority:     maxPri,
    }
    return NewVhost("test-vhost", options)
}

func createTestQueueWithMaxPriority(vh *VHost, name string, maxPri uint8) *Queue {
    props := &QueueProperties{
        Arguments: map[string]any{
            "x-max-priority": int(maxPri),
        },
    }
    queue, _ := vh.CreateQueue(name, props, nil)
    return queue
}

func pushMessageWithPriority(queue *Queue, body string, priority uint8) {
    msg := Message{
        ID:   generateID(),
        Body: []byte(body),
        Properties: amqp.BasicProperties{
            Priority: priority,
        },
    }
    queue.Push(msg)
}

func assertNextMessage(t *testing.T, queue *Queue, expectedBody string) {
    msg := queue.Pop()
    if msg == nil {
        t.Fatalf("Expected message '%s', got nil", expectedBody)
    }
    if string(msg.Body) != expectedBody {
        t.Errorf("Expected message body '%s', got '%s'", expectedBody, string(msg.Body))
    }
}
```

### 2. E2E Tests

**File**: `tests/e2e/priority_test.go`

```go
package e2e

import (
    "testing"
    "github.com/rabbitmq/amqp091-go"
    "github.com/stretchr/testify/require"
)

func TestPriorityQueue_DeliveryOrder(t *testing.T) {
    tc := NewTestConnection(t, brokerURL)
    defer tc.Close()
    
    // Declare queue with x-max-priority
    queue, err := tc.Ch.QueueDeclare(
        "test-priority-queue",
        false, false, false, false,
        amqp091.Table{
            "x-max-priority": int32(10),
        },
    )
    require.NoError(t, err)
    
    // Publish messages with different priorities
    publishWithPriority(t, tc.Ch, queue.Name, "low", 1)
    publishWithPriority(t, tc.Ch, queue.Name, "high", 9)
    publishWithPriority(t, tc.Ch, queue.Name, "medium", 5)
    
    // Consume and verify order
    msgs := consumeMessages(t, tc.Ch, queue.Name, 3)
    require.Equal(t, "high", string(msgs[0].Body))
    require.Equal(t, "medium", string(msgs[1].Body))
    require.Equal(t, "low", string(msgs[2].Body))
}

func TestPriorityQueue_FIFO_WithinPriority(t *testing.T) {
    tc := NewTestConnection(t, brokerURL)
    defer tc.Close()
    
    queue, err := tc.Ch.QueueDeclare(
        "test-fifo-priority",
        false, false, false, false,
        amqp091.Table{"x-max-priority": int32(10)},
    )
    require.NoError(t, err)
    
    // Publish multiple messages with same priority
    for i := 1; i <= 5; i++ {
        publishWithPriority(t, tc.Ch, queue.Name, fmt.Sprintf("msg-%d", i), 5)
    }
    
    // Should maintain FIFO within priority 5
    msgs := consumeMessages(t, tc.Ch, queue.Name, 5)
    for i := 0; i < 5; i++ {
        expected := fmt.Sprintf("msg-%d", i+1)
        require.Equal(t, expected, string(msgs[i].Body))
    }
}

func TestPriorityQueue_MixedPriorities(t *testing.T) {
    tc := NewTestConnection(t, brokerURL)
    defer tc.Close()
    
    queue, err := tc.Ch.QueueDeclare(
        "test-mixed-priority",
        false, false, false, false,
        amqp091.Table{"x-max-priority": int32(10)},
    )
    require.NoError(t, err)
    
    // Publish in random order
    publishWithPriority(t, tc.Ch, queue.Name, "p5-first", 5)
    publishWithPriority(t, tc.Ch, queue.Name, "p1", 1)
    publishWithPriority(t, tc.Ch, queue.Name, "p9", 9)
    publishWithPriority(t, tc.Ch, queue.Name, "p5-second", 5)
    publishWithPriority(t, tc.Ch, queue.Name, "p3", 3)
    
    // Expected order: p9, p5-first, p5-second, p3, p1
    msgs := consumeMessages(t, tc.Ch, queue.Name, 5)
    require.Equal(t, "p9", string(msgs[0].Body))
    require.Equal(t, "p5-first", string(msgs[1].Body))
    require.Equal(t, "p5-second", string(msgs[2].Body))
    require.Equal(t, "p3", string(msgs[3].Body))
    require.Equal(t, "p1", string(msgs[4].Body))
}

func TestPriorityQueue_NoPriorityIsZero(t *testing.T) {
    tc := NewTestConnection(t, brokerURL)
    defer tc.Close()
    
    queue, err := tc.Ch.QueueDeclare(
        "test-default-priority",
        false, false, false, false,
        amqp091.Table{"x-max-priority": int32(10)},
    )
    require.NoError(t, err)
    
    // Publish without priority (defaults to 0)
    err = tc.Ch.Publish("", queue.Name, false, false, amqp091.Publishing{
        Body: []byte("no-priority"),
    })
    require.NoError(t, err)
    
    publishWithPriority(t, tc.Ch, queue.Name, "priority-5", 5)
    
    // Priority 5 should come first
    msgs := consumeMessages(t, tc.Ch, queue.Name, 2)
    require.Equal(t, "priority-5", string(msgs[0].Body))
    require.Equal(t, "no-priority", string(msgs[1].Body))
}

func TestPriorityQueue_WithDLX(t *testing.T) {
    tc := NewTestConnection(t, brokerURL)
    defer tc.Close()
    
    // Setup DLX
    dlx := "test-dlx-priority"
    err := tc.Ch.ExchangeDeclare(dlx, "direct", false, true, false, false, nil)
    require.NoError(t, err)
    
    dlq, err := tc.Ch.QueueDeclare("test-dlq-priority", false, true, false, false, nil)
    require.NoError(t, err)
    
    err = tc.Ch.QueueBind(dlq.Name, "dlx-key", dlx, false, nil)
    require.NoError(t, err)
    
    // Main priority queue with DLX
    queue, err := tc.Ch.QueueDeclare(
        "test-priority-dlx",
        false, false, false, false,
        amqp091.Table{
            "x-max-priority":           int32(10),
            "x-dead-letter-exchange":   dlx,
            "x-dead-letter-routing-key": "dlx-key",
        },
    )
    require.NoError(t, err)
    
    // Publish with priority
    publishWithPriority(t, tc.Ch, queue.Name, "high-rejected", 9)
    
    // Consume and reject
    msgs := consumeMessages(t, tc.Ch, queue.Name, 1)
    msgs[0].Nack(false, false)  // Reject, should go to DLX
    
    // Check DLQ received message with priority preserved
    dlqMsgs := consumeMessages(t, tc.Ch, dlq.Name, 1)
    require.Equal(t, "high-rejected", string(dlqMsgs[0].Body))
    require.Equal(t, uint8(9), dlqMsgs[0].Priority)
}

// Helper functions
func publishWithPriority(t *testing.T, ch *amqp091.Channel, queue, body string, priority uint8) {
    err := ch.Publish("", queue, false, false, amqp091.Publishing{
        Body:     []byte(body),
        Priority: priority,
    })
    require.NoError(t, err)
}

func consumeMessages(t *testing.T, ch *amqp091.Channel, queue string, count int) []amqp091.Delivery {
    msgs, err := ch.Consume(queue, "", false, false, false, false, nil)
    require.NoError(t, err)
    
    result := make([]amqp091.Delivery, 0, count)
    for i := 0; i < count; i++ {
        select {
        case msg := <-msgs:
            result = append(result, msg)
            msg.Ack(false)
        case <-time.After(2 * time.Second):
            t.Fatalf("Timeout waiting for message %d/%d", i+1, count)
        }
    }
    return result
}
```

---

## 📚 Documentation

### 1. User-Facing Documentation

**File**: `docs/priority-queues.md`

```markdown
# Priority Queues

## Overview

Priority queues in OtterMQ allow messages to be delivered in priority order rather than strict FIFO order. This is a core AMQP 0.9.1 feature that enables important messages to jump ahead of routine messages.

## AMQP 0.9.1 Compliance

✅ **Priority queues are part of the AMQP 0.9.1 specification**

- **Priority field**: Standard AMQP BasicProperties field (0-9 recommended, 0-255 supported)
- **Queue declaration**: RabbitMQ extension argument `x-max-priority` (commonly used)
- **Delivery order**: Higher priority messages delivered first, FIFO within priority

## Configuration

### Global Priority Limit

Set the maximum priority level supported by the broker:

```sh
export OTTERMQ_MAX_PRIORITY=10  # Default: 10, Range: 1-255
```

**Recommendations**:

- **3-5 levels**: Typical use case (high/normal/low or critical/high/normal/low/background)
- **10 levels**: AMQP spec recommendation (default)
- **>10 levels**: Higher memory usage per queue, use only if needed

### Queue Declaration

Declare a queue with priority support using `x-max-priority` argument:

```go
queue, err := ch.QueueDeclare(
    "my-priority-queue",
    false, false, false, false,
    amqp091.Table{
        "x-max-priority": int32(10),  // Support priorities 0-10
    },
)
```

**Important**:

- Queues **without** `x-max-priority` remain FIFO (backward compatible)
- Priority value is clamped to broker's `OTTERMQ_MAX_PRIORITY` setting
- Higher `x-max-priority` values use more memory per queue

### Publishing with Priority

Set the `Priority` field when publishing:

```go
err = ch.Publish(
    "",           // exchange
    "my-priority-queue",
    false, false,
    amqp091.Publishing{
        Body:     []byte("Important message"),
        Priority: 9,  // 0-9 range (AMQP spec), 0-255 allowed
    },
)
```

**Priority Behavior**:

- **Priority 0**: Lowest (default if not specified)
- **Priority 9**: Highest (AMQP spec recommends 0-9 range)
- **Priority 255**: Maximum allowed (RabbitMQ extension)
- **Priority > queue's x-max-priority**: Clamped to queue's max

## How It Works

### Delivery Order

Messages are delivered in the following order:

1. **Primary**: Higher priority first
2. **Secondary**: FIFO within same priority level

**Example**:

```code
Published:     [P5, P1, P9, P5, P3]
Delivered:     [P9, P5, P5, P3, P1]  ← Highest priority first
```

### Memory Architecture

OtterMQ uses a **map-of-channels** architecture:

- One channel per priority level (lazy allocation)
- FIFO automatically maintained within each channel
- Efficient use of Go's channel semantics

**Memory Usage**:

- Queue with `x-max-priority=10`: Up to 10 channels (only allocated when used)
- Buffer divided among priorities: `OTTERMQ_QUEUE_BUFFER_SIZE / (max_priority + 1)`
- Typical: 100,000 buffer ÷ 11 priorities = ~9,090 messages per priority

## Use Cases

### Task Queue with Priority Levels

```go
// Critical tasks
publishTask(ch, "urgent-queue", criticalTask, 9)

// Normal tasks
publishTask(ch, "urgent-queue", normalTask, 5)

// Background tasks
publishTask(ch, "urgent-queue", backgroundTask, 1)
```

### Email Processing

```go
const (
    PriorityPassword  = 9  // Password reset emails
    PriorityTransactional = 7  // Order confirmations
    PriorityMarketing = 3  // Newsletters
)
```

### Monitoring Alerts

```go
const (
    PriorityCritical = 9   // System down
    PriorityWarning  = 6   // High CPU
    PriorityInfo     = 3   // Routine checks
)
```

## Integration with Extensions

### Priority + Dead Letter Exchange (DLX)

Priority is preserved when messages are dead-lettered:

```go
queue, _ := ch.QueueDeclare(
    "priority-with-dlx",
    false, false, false, false,
    amqp091.Table{
        "x-max-priority":        int32(10),
        "x-dead-letter-exchange": "my-dlx",
    },
)

// Priority 9 message rejected → goes to DLX with priority 9
```

### Priority + Message TTL

Expired messages are removed regardless of priority:

```go
ch.Publish("", "priority-ttl-queue", false, false, amqp091.Publishing{
    Body:       []byte("Expires in 5 seconds"),
    Priority:   9,         // High priority
    Expiration: "5000",    // But will expire if not consumed
})
```

### Priority + Queue Length Limit (QLL)

When queue is full, **oldest message** is dropped (FIFO within priority):

```go
queue, _ := ch.QueueDeclare(
    "priority-limited",
    false, false, false, false,
    amqp091.Table{
        "x-max-priority": int32(10),
        "x-max-length":   int32(1000),
    },
)
```

## Performance Considerations

### Memory Usage

**Non-priority queue**: 1 channel × buffer size  
**Priority queue**: Up to (max_priority + 1) channels

**Example** (default settings):

- `OTTERMQ_QUEUE_BUFFER_SIZE=100000`
- `x-max-priority=10`
- Memory: ~11 channels × ~9,090 buffer each = ~100K total message slots

**Recommendation**: Use 3-10 priority levels for optimal memory/performance balance

### CPU Overhead

**Priority queue**: O(P) priority scan on each pop (P = max_priority)  
**FIFO queue**: O(1) channel receive

**Impact**: Negligible for P ≤ 10 (typical: <1% CPU overhead)

### Lazy Allocation

Channels are only created when first message at that priority is pushed:

- Empty priorities use no memory
- Most queues use 1-3 priority levels in practice
- Memory footprint scales with actual usage

## Backward Compatibility

**Queues without `x-max-priority` remain FIFO**:

- No memory overhead
- No performance impact
- Existing code continues to work unchanged

**Explicit FIFO queue** (if x-max-priority is added later):

```go
queue, _ := ch.QueueDeclare(
    "fifo-only",
    false, false, false, false,
    nil,  // No x-max-priority argument
)
```

## RabbitMQ Compatibility

✅ **Fully compatible with RabbitMQ clients**

| Feature | RabbitMQ | OtterMQ |
|---------|----------|---------|
| `x-max-priority` argument | ✅ 1-255 | ✅ 1-255 (default limit: 10) |
| Priority field (0-9) | ✅ | ✅ |
| Priority field (0-255) | ✅ | ✅ |
| FIFO within priority | ✅ | ✅ |
| Priority clamping | ✅ | ✅ |
| DLX integration | ✅ | ✅ |

**Difference**: OtterMQ limits `x-max-priority` to `OTTERMQ_MAX_PRIORITY` (default: 10) for performance reasons. This is configurable up to 255.

## Examples

### Python (pika)

```python
import pika

connection = pika.BlockingConnection(pika.ConnectionParameters('localhost'))
channel = connection.channel()

# Declare priority queue
channel.queue_declare(
    queue='priority-queue',
    arguments={'x-max-priority': 10}
)

# Publish with priority
channel.basic_publish(
    exchange='',
    routing_key='priority-queue',
    body='High priority message',
    properties=pika.BasicProperties(priority=9)
)
```

### Node.js (amqplib)

```javascript
const amqp = require('amqplib');

const conn = await amqp.connect('amqp://localhost');
const ch = await conn.createChannel();

// Declare priority queue
await ch.assertQueue('priority-queue', {
    arguments: { 'x-max-priority': 10 }
});

// Publish with priority
ch.sendToQueue('priority-queue', Buffer.from('High priority'), {
    priority: 9
});
```

## Troubleshooting

### Messages not delivered in priority order

**Check**:

1. Queue declared with `x-max-priority` argument?
2. Messages published with `Priority` field set?
3. Priority value within 0-255 range?

### High memory usage

**Solution**: Reduce `x-max-priority` or `OTTERMQ_MAX_PRIORITY`

```sh
# Limit to 5 priorities globally
export OTTERMQ_MAX_PRIORITY=5
```

### Priority seems ignored

**Verify**: Queue must be declared with `x-max-priority` **before** messages are published

## References

- [AMQP 0-9-1 Specification](https://www.rabbitmq.com/resources/specs/amqp0-9-1.pdf)
- [RabbitMQ Priority Queues](https://www.rabbitmq.com/priority.html)
- [OtterMQ Configuration](../README.md#configuration)

---

**Last Updated**: December 4, 2025  
**Feature Status**: ✅ Implemented in v0.16.0

```markdown

### 2. README Updates

**File**: `README.md`

\```markdown
## ✨ Features

- AMQP-style Message Queuing
- Exchanges and Bindings (Direct, Fanout, Topic)
- **Priority Queues** - AMQP 0.9.1 priority message delivery (NEW)
- Dead Letter Exchange (DLX) - RabbitMQ-compatible error handling
- Message TTL and Expiration
- Quality of Service (QoS) with prefetch limits
- Transactions (TX class) - Atomic commit/rollback
- Channel Flow Control for backpressure management
- Pluggable Persistence Layer (JSON files, Memento WAL planned)
- Management Interface (Vue + Quasar)
- Docker Support via `docker-compose`
- RabbitMQ Client Compatibility

## ⚙️ Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `OTTERMQ_BROKER_PORT` | `5672` | AMQP broker port |
| `OTTERMQ_MAX_PRIORITY` | `10` | Maximum priority level (1-255) |
| `OTTERMQ_QUEUE_BUFFER_SIZE` | `100000` | Queue message buffer size |
| ... | ... | ... |
```

---

## 🔄 Migration Strategy

### Phase 1: Configuration & Foundation (Days 1-2) ✅

**Goals**: Add configuration support and basic structure

**Tasks**:

- [ ] Add `MaxPriority uint8` to `config.Config` struct
- [ ] Implement `getEnvAsUint8WithMax()` helper with clamping logic
- [ ] Add configuration tests (valid, invalid, clamping, defaults)
- [ ] Update `VHostOptions` with `MaxPriority` field
- [ ] Pass `MaxPriority` from broker to VHost in `NewBroker()`
- [ ] Update `.env.example` and `README.md` with new configuration

**Deliverables**:

- ✅ Configuration system ready
- ✅ Tests passing
- ✅ Documentation updated

### Phase 2: Queue Structure Updates (Days 3-4)

**Goals**: Add priority queue fields and initialization

**Tasks**:

- [ ] Add `priorityMessages`, `maxPriority`, `messageSignal` fields to `Queue` struct
- [ ] Implement `parseMaxPriorityArgument()` helper in `helpers.go`
- [ ] Update `CreateQueue()` to detect and initialize priority queues
- [ ] Add clamping logic (requested priority vs broker max)
- [ ] Update `QueueDTO` in `models/dto.go` with `MaxPriority` field
- [ ] Update management service `queueToDTO()` to extract x-max-priority

**Deliverables**:

- ✅ Queue struct supports both FIFO and priority modes
- ✅ Lazy initialization pattern ready
- ✅ API exposes max-priority information

### Phase 3: Priority Push/Pop (Days 5-7)

**Goals**: Implement priority-aware message storage and retrieval

**Tasks**:

- [ ] Create `priority.go` with priority queue operations
- [ ] Implement `pushPriority()` with lazy channel allocation
- [ ] Implement `popPriorityUnlocked()` with priority scanning
- [ ] Update `Push()` to route to priority or FIFO based on `maxPriority`
- [ ] Update `Pop()` to call correct implementation
- [ ] Add signal channel logic to unblock delivery loop
- [ ] Implement priority purge logic in `StreamPurge()`

**Deliverables**:

- ✅ Push/pop work correctly with priority ordering
- ✅ FIFO maintained within priority levels
- ✅ Backward compatible with non-priority queues

### Phase 4: Delivery Loop Integration (Days 8-9)

**Goals**: Update delivery loop to handle priority queues

**Tasks**:

- [ ] Refactor `startDeliveryLoop()` to support both modes
- [ ] Implement `deliverFromPriorityQueue()` with signal-based blocking
- [ ] Implement `deliverFromFIFOQueue()` (extract existing logic)
- [ ] Extract delivery logic into `deliverMessage()` helper
- [ ] Test delivery order with multiple priorities
- [ ] Test consumer assignment with priority messages

**Deliverables**:

- ✅ Delivery loop works for both queue types
- ✅ Priority order respected during delivery
- ✅ QoS and flow control still work

### Phase 5: Unit Testing (Days 10-11)

**Goals**: Comprehensive unit test coverage

**Tasks**:

- [ ] Create `priority_test.go` with 10+ test cases
- [ ] Test push/pop priority ordering
- [ ] Test FIFO within same priority
- [ ] Test priority clamping
- [ ] Test lazy channel allocation
- [ ] Test purge with priority queues
- [ ] Test backward compatibility (FIFO queues unchanged)
- [ ] Test edge cases (empty queues, single priority, etc.)

**Deliverables**:

- ✅ 100% unit test coverage for priority logic
- ✅ All tests passing

### Phase 6: E2E Testing (Days 12-13)

**Goals**: Validate with RabbitMQ clients

**Tasks**:

- [ ] Create `tests/e2e/priority_test.go`
- [ ] Test priority delivery order with real AMQP client
- [ ] Test FIFO within priority with rabbitmq/amqp091-go
- [ ] Test mixed priorities
- [ ] Test priority with DLX integration
- [ ] Test priority with TTL integration
- [ ] Test priority with QLL integration
- [ ] Test multiple consumers with priority queues

**Deliverables**:

- ✅ RabbitMQ client compatibility verified
- ✅ Integration with extensions working
- ✅ All E2E tests passing

### Phase 7: Documentation & Release (Days 14)

**Goals**: Complete documentation and prepare release

**Tasks**:

- [ ] Create `docs/priority-queues.md` user guide
- [ ] Update `README.md` with priority queue feature
- [ ] Update `ROADMAP.md` to mark priority queues as complete
- [ ] Update `CHANGELOG.md` for v0.16.0 release
- [ ] Update Swagger docs if API exposes priority info
- [ ] Add examples for Go, Python, Node.js clients

**Deliverables**:

- ✅ Complete user documentation
- ✅ CHANGELOG updated
- ✅ Ready for release

---

## ✅ Acceptance Criteria

### Must Have

- [ ] Queues can be declared with `x-max-priority` argument (1-255 range)
- [ ] Messages can be published with `priority` field (0-255)
- [ ] Higher priority messages delivered before lower priority
- [ ] FIFO order maintained within same priority level
- [ ] Messages without priority default to 0
- [ ] Priority clamped to queue's `x-max-priority` setting
- [ ] Configuration: `OTTERMQ_MAX_PRIORITY` (default: 10, max: 255)
- [ ] Non-priority queues remain FIFO (no breaking changes)
- [ ] Zero overhead for non-priority queues
- [ ] All existing tests continue to pass

### Should Have

- [ ] Lazy channel allocation (only used priorities allocate memory)
- [ ] Buffer divided among priorities intelligently
- [ ] Integration with DLX (priority preserved in dead letters)
- [ ] Integration with TTL (expiration works with priority)
- [ ] Integration with QLL (max-length works with priority)
- [ ] Unit tests: 10+ test cases with 100% coverage
- [ ] E2E tests: 8+ scenarios with RabbitMQ client

### Nice to Have

- [ ] Performance benchmarks vs FIFO queues
- [ ] Memory usage documentation with examples
- [ ] UI displays queue's max-priority setting
- [ ] API statistics for priority distribution

---

## 📈 Success Metrics

### Technical Metrics

- ✅ 100% AMQP 0.9.1 priority field support
- ✅ RabbitMQ client compatibility
- ✅ <5% CPU overhead for priority queues (vs FIFO)
- ✅ Memory usage proportional to active priorities (not max)
- ✅ 100% backward compatibility (zero breaking changes)
- ✅ 100% test coverage for priority logic

### User Experience Metrics

- ✅ Critical messages delivered immediately (priority 9)
- ✅ Routine messages don't block important ones
- ✅ Easy configuration (single queue argument)
- ✅ Clear documentation and examples
- ✅ Works with existing RabbitMQ tools/clients

---

## 🚀 Rollout Plan

### Development

1. Create feature branch: `feature/priority-queues`
2. Implement in phases (follow migration strategy)
3. Continuous testing and validation
4. Code review after each phase

### Testing

1. Unit tests for each priority queue operation
2. E2E tests with RabbitMQ clients
3. Integration tests with DLX/TTL/QLL
4. Performance testing and benchmarking

### Deployment

1. Merge to `main` after all tests pass
2. Tag release: `v0.16.0`
3. Update documentation
4. Announce in release notes

### Monitoring

1. Monitor priority queue usage
2. Track memory usage patterns
3. Validate priority ordering in production
4. Gather user feedback

---

## 🔗 Related Documentation

- [AMQP 0-9-1 Specification](https://www.rabbitmq.com/resources/specs/amqp0-9-1.pdf) - Section 4.2.6 (Basic.Properties.priority)
- [RabbitMQ Priority Queues](https://www.rabbitmq.com/priority.html)
- [Dead Letter Exchange](docs/dead-letter-exchange.md)
- [Message TTL](docs/message-ttl.md)
- [Queue Length Limiting](docs/qll-implementation.md)

---

## 📝 Notes & Considerations

### Why Map-of-Channels?

**Alternatives Considered**:

1. **Heap-based priority queue**: More complex, requires sequence numbering for FIFO
2. **Single channel with sorting**: O(n log n) per message
3. **Bucket array**: Similar to map, but pre-allocates all priorities

**Decision**: Map-of-channels provides:

- Simple implementation (reuse Go channels)
- Automatic FIFO guarantee
- Lazy allocation (memory efficient)
- Familiar Go patterns

### Memory Management

**Default Config**:

- `OTTERMQ_QUEUE_BUFFER_SIZE=100000`
- `OTTERMQ_MAX_PRIORITY=10`
- Per-priority buffer: ~9,090 messages

**Optimization**: Only allocate channels for used priorities (lazy allocation)

**Example**:

- Queue uses only priority 0, 5, 9 → Only 3 channels allocated
- Unused priorities: Zero memory overhead

### Performance Tuning

**For high-throughput systems**:

```sh
# Reduce priorities to 3 levels
export OTTERMQ_MAX_PRIORITY=3  # High/Normal/Low
```

**For memory-constrained systems**:

```sh
# Smaller buffer with fewer priorities
export OTTERMQ_QUEUE_BUFFER_SIZE=10000
export OTTERMQ_MAX_PRIORITY=3
```

### Backward Compatibility

**Guaranteed**:

- Existing queues work unchanged
- FIFO queues have zero overhead
- No API changes
- All existing tests pass

**Safe Migration**:

- Add priority queues incrementally
- Test with subset of queues first
- Monitor memory usage

---

## 👥 Contributors & Reviewers

**Author**: GitHub Copilot (AI Assistant)  
**Reviewers**: @andrelcunha  
**Status**: 🚧 In Progress

---

**Last Updated**: December 4, 2025  
**Document Version**: 1.0  
**Implementation Status**: In Progress - Phase 1
