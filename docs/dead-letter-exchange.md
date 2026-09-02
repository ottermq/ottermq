# Dead Letter Exchange (DLX)

## Overview

Dead Letter Exchanges (DLX) allow messages that cannot be delivered or processed to be routed to another exchange instead of being silently discarded. This is useful for implementing retry logic, error logging, and debugging pipelines.

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

## Configuration

### Feature Flag

**Environment Variable**: `OTTERMQ_ENABLE_DLX`

- **Default**: `true` (enabled for RabbitMQ compatibility)
- **Description**: Enable dead letter exchange extension feature

When DLX is disabled:

- `x-dead-letter-exchange` argument is ignored (logged as warning)
- Messages are discarded normally
- No x-death headers added

## RabbitMQ Compatibility Matrix

| Feature | RabbitMQ 3.x | OtterMQ |
|---------|--------------|---------|
| x-dead-letter-exchange | ✅ | ✅ |
| x-dead-letter-routing-key | ✅ | ✅ |
| Death reason: rejected | ✅ | ✅ |
| Death reason: expired | ✅ | 🔄 Planned |
| Death reason: maxlen | ✅ | 🔄 Planned |
| x-death headers | ✅ | ✅ |
| Death count tracking | ✅ | ✅ |
| Cycle detection | ✅ | 🔄 Planned |

## Edge Cases & Considerations

### 1. DLX Exchange Doesn't Exist

**Behavior**: Log error, discard message (don't crash)
**RabbitMQ**: Discards message and logs warning

### 2. Infinite Cycles

**Scenario**: DLX queue has DLX pointing back to original queue
**Mitigation**: No automatic cycle detection yet — avoid creating circular DLX chains. Future versions will detect cycles via x-death count threshold.

### 3. DLX Routing Failure

**Scenario**: No queue bound to DLX for the routing key
**Behavior**: Message is lost (same as normal unroutable message)

### 4. Transaction Rollback

**Scenario**: Message rejected in transaction, then rolled back
**Behavior**: Rejection is rolled back, message returns to queue, no DLX routing occurs
**RabbitMQ**: Same behavior

### 5. Persistent Messages

**Scenario**: Durable queue with DLX, persistent messages
**Behavior**: Dead-lettered messages retain their persistence flag

## Migration Path

### For New Users

- DLX enabled by default
- Works out of the box like RabbitMQ

### For Existing Users

- No breaking changes
- Existing queues without DLX config work unchanged
- Can add DLX to existing queues via re-declare (if properties match)

## References

- [RabbitMQ Dead Letter Exchanges](https://www.rabbitmq.com/dlx.html)
- [RabbitMQ Protocol Extensions](https://www.rabbitmq.com/extensions.html)
- [AMQP 0.9.1 Specification](https://www.rabbitmq.com/resources/specs/amqp0-9-1.pdf)

---

**Status**: ✅ Implemented (v0.12.0)  
**Last Updated**: November 15, 2025
