# Dead Letter Exchange (DLX) Implementation

## Overview

Dead Letter Exchanges (DLX) are a **RabbitMQ extension** not part of the AMQP 0.9.1 specification. This document outlines OtterMQ's implementation approach to maintain compatibility with RabbitMQ clients while clearly documenting this as a feature flag/extension.

## Why Dead Letter Exchanges?

DLX provides a mechanism to handle messages that cannot be delivered or processed:

- Messages rejected by consumers (via `basic.reject` or `basic.nack` with `requeue=false`)
- Messages that expire due to per-message or per-queue TTL
- Messages dropped due to queue length limits (maxlen)

Without DLX, these messages are simply discarded. DLX allows them to be routed to another exchange for logging, retry logic, or debugging.

## AMQP 0.9.1 Compliance Note

⚠️ **This is a RabbitMQ extension feature** and is NOT part of the official AMQP 0.9.1 specification.

- **Specification**: RabbitMQ extensions to AMQP 0.9.1
- **Feature Flag**: `OTTERMQ_ENABLE_DLX` (default: `true` for RabbitMQ compatibility)
- **Compatibility**: Designed to match RabbitMQ 3.x behavior
- **Documentation**: All DLX-related headers and behaviors documented as extensions

## High-Level Design

### 1. Queue Configuration

Dead letter exchange is configured **per queue** using queue arguments during `queue.declare`:

```code
Arguments:
  x-dead-letter-exchange: <exchange-name>       (string, optional)
  x-dead-letter-routing-key: <routing-key>      (string, optional)
```

**Behavior**:

- `x-dead-letter-exchange`: The exchange to route dead-lettered messages to
  - If empty string `""`, uses the default exchange
  - If not specified, messages are discarded (current behavior)
- `x-dead-letter-routing-key`: Override routing key for dead-lettered messages
  - If not specified, uses the message's original routing key
  - Useful for routing all dead letters to a specific queue

### 2. Death Reasons

Messages can be dead-lettered for three reasons:

| Reason | Trigger | Description |
|--------|---------|-------------|
| `rejected` | `basic.reject(requeue=false)` or `basic.nack(requeue=false)` | Consumer explicitly rejected message |
| `expired` | Message TTL or queue TTL expires | Message lived too long (future feature) |
| `maxlen` | Queue length limit exceeded | Queue is full (future feature) |

### 3. Death Headers

When a message is dead-lettered, OtterMQ adds `x-death` headers to track the journey:

```json
{
  "x-death": [
    {
      "reason": "rejected",                    // rejected | expired | maxlen
      "queue": "original-queue-name",          // Queue where death occurred
      "time": "2025-11-11T10:30:00Z",         // ISO 8601 timestamp
      "exchange": "original-exchange",         // Original exchange (if known)
      "routing-keys": ["original.routing.key"], // Original routing key(s)
      "count": 1                               // Number of times dead-lettered from this queue
    }
  ]
}
```

**Notes**:

- `x-death` is an **array** - messages can be dead-lettered multiple times
- Each entry represents a death event
- `count` increments if the same message dies in the same queue again
- Newest death events are prepended to the array

### 4. Message Flow

```code
┌─────────────┐
│  Producer   │
└──────┬──────┘
       │ publish
       ▼
┌─────────────────┐
│   Exchange A    │
└────────┬────────┘
         │ route
         ▼
┌────────────────────────────────────┐
│  Queue (with DLX configured)       │
│  x-dead-letter-exchange: "dlx"     │
│  x-dead-letter-routing-key: "dead" │
└────────┬───────────────────────────┘
         │
    ┌────┴────┐
    │ Consumer│
    └────┬────┘
         │
         │ basic.reject(requeue=false)
         ▼
┌─────────────────┐
│   DLX Exchange  │  (message gets x-death headers added)
└────────┬────────┘
         │ route with override key "dead"
         ▼
┌─────────────────┐
│  Dead Letter Q  │
└─────────────────┘
```

## Implementation Plan

### Phase 1: Core DLX Functionality (Priority)

#### 1.1 Queue Arguments Support

**File**: `internal/core/broker/vhost/queue.go`

```go
type QueueProperties struct {
    // ... existing fields
    DeadLetterExchange   string // x-dead-letter-exchange
    DeadLetterRoutingKey string // x-dead-letter-routing-key
}
```

**Changes**:

- Parse `x-dead-letter-exchange` and `x-dead-letter-routing-key` from queue.declare arguments
- Validate that DLX exchange exists (or will be created)
- Store in queue properties
- Persist durable queue DLX configuration

#### 1.2 Message Rejection with DLX

**File**: `internal/core/broker/vhost/ack.go` (HandleBasicReject, HandleBasicNack)

**Changes**:

```go
func (vh *VHost) HandleBasicReject(conn net.Conn, channel uint16, deliveryTag uint64, requeue bool) error {
    // ... existing code to find message ...
    
    if !requeue {
        // Check if queue has DLX configured
        queue := vh.Queues[record.QueueName]
        if queue.Props.DeadLetterExchange != "" {
            // Dead letter the message instead of discarding
            vh.deadLetterMessage(record.Message, queue, "rejected")
            return nil
        }
    }
    
    // ... existing requeue or discard logic ...
}
```

#### 1.3 Dead Letter Message Function

**File**: `internal/core/broker/vhost/dead_letter.go` (new file)

```go
package vhost

import (
    "time"
    "github.com/andrelcunha/ottermq/internal/core/amqp"
)

type DeathRecord struct {
    Reason      string    // "rejected" | "expired" | "maxlen"
    Queue       string    // Queue where death occurred
    Time        time.Time // When it died
    Exchange    string    // Original exchange
    RoutingKeys []string  // Original routing keys
    Count       int       // Number of times dead-lettered from this queue
}

// deadLetterMessage routes a message to the configured dead letter exchange
func (vh *VHost) deadLetterMessage(msg amqp.Message, queue *Queue, reason string) error {
    // 1. Add x-death header
    msg.Properties.Headers = vh.addDeathHeader(msg.Properties.Headers, queue, reason)
    
    // 2. Determine routing key
    routingKey := msg.RoutingKey
    if queue.Props.DeadLetterRoutingKey != "" {
        routingKey = queue.Props.DeadLetterRoutingKey
    }
    
    // 3. Publish to DLX
    dlxExchange := queue.Props.DeadLetterExchange
    _, err := vh.Publish(dlxExchange, routingKey, msg)
    
    return err
}

func (vh *VHost) addDeathHeader(headers map[string]any, queue *Queue, reason string) map[string]any {
    if headers == nil {
        headers = make(map[string]any)
    }
    
    // Create death record
    death := map[string]any{
        "reason":       reason,
        "queue":        queue.Name,
        "time":         time.Now().UTC().Format(time.RFC3339),
        "exchange":     queue.BoundToExchange, // Track if available
        "routing-keys": []string{queue.LastRoutingKey}, // Track if available
        "count":        1,
    }
    
    // Get existing x-death array or create new
    xDeath, exists := headers["x-death"]
    if !exists {
        headers["x-death"] = []any{death}
    } else {
        // Prepend to array (newest first)
        deathArray := xDeath.([]any)
        
        // Check if already died in this queue - increment count
        for i, d := range deathArray {
            existingDeath := d.(map[string]any)
            if existingDeath["queue"] == queue.Name && existingDeath["reason"] == reason {
                existingDeath["count"] = existingDeath["count"].(int) + 1
                deathArray[i] = existingDeath
                headers["x-death"] = deathArray
                return headers
            }
        }
        
        // New death event - prepend
        headers["x-death"] = append([]any{death}, deathArray...)
    }
    
    return headers
}
```

### Phase 2: TTL Integration (Future)

When message/queue TTL is implemented:

```go
func (vh *VHost) expireMessage(msg amqp.Message, queue *Queue) {
    if queue.Props.DeadLetterExchange != "" {
        vh.deadLetterMessage(msg, queue, "expired")
    }
    // else discard
}
```

### Phase 3: Queue Length Limit (Future)

When queue maxlen is implemented:

```go
func (q *Queue) Push(msg amqp.Message) error {
    if q.Props.MaxLength > 0 && q.Len() >= q.Props.MaxLength {
        // Remove oldest message and dead letter it
        oldest := q.PopOldest()
        if q.Props.DeadLetterExchange != "" {
            vh.deadLetterMessage(oldest, q, "maxlen")
        }
    }
    // ... push new message
}
```

## Configuration

### Feature Flag

**Environment Variable**: `OTTERMQ_ENABLE_DLX`

- **Default**: `true` (enabled for RabbitMQ compatibility)
- **Description**: Enable dead letter exchange extension feature

**Config File** (`config/config.go`):

```go
type Config struct {
    // ... existing fields
    EnableDLX bool `env:"OTTERMQ_ENABLE_DLX" envDefault:"true"`
}
```

### Validation

When DLX is disabled:

- `x-dead-letter-exchange` argument is ignored (logged as warning)
- Messages are discarded normally
- No x-death headers added

## Testing Strategy

### 1. Unit Tests

**File**: `internal/core/broker/vhost/dead_letter_test.go`

Test cases:

- ✅ Add death header to message without existing x-death
- ✅ Prepend death header to message with existing x-death
- ✅ Increment count for repeated deaths in same queue
- ✅ Preserve other message headers
- ✅ Handle nil headers map

### 2. Integration Tests

**File**: `internal/core/broker/dead_letter_test.go`

Test cases:

- ✅ Queue declares with x-dead-letter-exchange argument
- ✅ Message rejected (requeue=false) routes to DLX
- ✅ Override routing key works correctly
- ✅ Multiple death cycles (DLX queue itself has DLX)
- ✅ DLX exchange doesn't exist (error handling)

### 3. E2E Tests

**File**: `tests/e2e/dead_letter_test.go`

Test cases:

- ✅ End-to-end rejection flow with RabbitMQ client
- ✅ x-death headers are correctly populated
- ✅ Multiple consumers and death cycles
- ✅ Feature flag disable/enable behavior

## RabbitMQ Compatibility Matrix

| Feature | RabbitMQ 3.x | OtterMQ Phase 1 | OtterMQ Future |
|---------|--------------|-----------------|----------------|
| x-dead-letter-exchange | ✅ | ✅ | ✅ |
| x-dead-letter-routing-key | ✅ | ✅ | ✅ |
| Death reason: rejected | ✅ | ✅ | ✅ |
| Death reason: expired | ✅ | ❌ | 🔄 Phase 2 |
| Death reason: maxlen | ✅ | ❌ | 🔄 Phase 3 |
| x-death headers | ✅ | ✅ | ✅ |
| Death count tracking | ✅ | ✅ | ✅ |
| Cycle detection | ✅ | ⚠️ Manual | 🔄 Future |

## Edge Cases & Considerations

### 1. DLX Exchange Doesn't Exist

**Behavior**: Log error, discard message (don't crash)
**RabbitMQ**: Discards message and logs warning

### 2. Infinite Cycles

**Scenario**: DLX queue has DLX pointing back to original queue
**Mitigation**:

- Phase 1: No automatic detection (user responsibility)
- Future: Detect cycles via x-death count threshold

### 3. DLX Routing Failure

**Scenario**: No queue bound to DLX for the routing key
**Behavior**: Message is lost (same as normal unroutable message)
**Future**: Could implement DLX for DLX (dead-letter-dead-letters)

### 4. Transaction Rollback

**Scenario**: Message rejected in transaction, then rolled back
**Behavior**: Rejection is rolled back, message returns to queue, no DLX
**RabbitMQ**: Same behavior

### 5. Persistent Messages

**Scenario**: Durable queue with DLX, persistent messages
**Behavior**: Dead-lettered messages retain persistence flag
**Implementation**: Keep `deliveryMode` property unchanged

## Documentation Requirements

### 1. User Documentation

**File**: `docs/features/dead-letter-exchange.md`

- What is DLX and why use it
- How to configure DLX on queues
- Examples with different languages (Go, Python, etc.)
- Common patterns (retry queues, error logging)

### 2. API Documentation

Update Swagger docs:

- Document `x-dead-letter-exchange` queue argument
- Document `x-dead-letter-routing-key` queue argument
- Note these are RabbitMQ extensions

### 3. GitHub Pages

Update `docs/amqp-status.md`:

- Mark DLX as implemented (with extension note)
- Link to detailed feature documentation

### 4. Code Comments

All DLX-related code should include:

```go
// DLX (Dead Letter Exchange) - RabbitMQ Extension
// This feature is NOT part of AMQP 0.9.1 specification.
// See: docs/features/dead-letter-exchange.md
```

## Migration Path

### For New Users

- DLX enabled by default
- Works out of the box like RabbitMQ

### For Existing Users

- No breaking changes
- Existing queues without DLX config work unchanged
- Can add DLX to existing queues via re-declare (if properties match)

## Success Criteria

Phase 1 is complete when:

- ✅ Queues can be declared with `x-dead-letter-exchange` argument
- ✅ `basic.reject(requeue=false)` routes to DLX
- ✅ `basic.nack(requeue=false)` routes to DLX
- ✅ `x-death` headers are correctly added
- ✅ Death count increments correctly
- ✅ Override routing key works
- ✅ Feature flag controls DLX behavior
- ✅ RabbitMQ Go client compatibility verified
- ✅ All tests pass
- ✅ Documentation complete

## References

- [RabbitMQ Dead Letter Exchanges](https://www.rabbitmq.com/dlx.html)
- [RabbitMQ Protocol Extensions](https://www.rabbitmq.com/extensions.html)
- [AMQP 0.9.1 Specification](https://www.rabbitmq.com/resources/specs/amqp0-9-1.pdf)

---

**Status**: 📋 Proposed  
**Last Updated**: November 11, 2025  
**Authors**: OtterMQ Development Team  
**Next Steps**: Review and approve design, begin Phase 1 implementation
