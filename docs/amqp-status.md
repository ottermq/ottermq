---
title: AMQP 0.9.1 Support Status
---

## AMQP 0.9.1 Support Status

This page tracks OtterMQ's support for AMQP 0.9.1 classes and methods. It is intended to help users understand current capabilities and to guide contributors.

Status levels:

- **Implemented** ✅: Feature is available and tested
- **Partial** ⚠️: Some behavior is missing or differs from spec
- **Planned** ❌: Not yet implemented but on the roadmap
- **Not Supported** ‼️: Out of scope or no plans yet

## Summary by Class

| Class | Status | Notes |
|------:|:------:|-------|
| connection | 100% | All methods fully implemented |
| channel | 100% | All methods fully implemented including flow control |
| exchange | 100% | All exchange types fully implemented |
| queue | 100% | All methods fully implemented |
| basic | 100% | All methods fully implemented |
| tx | 100% | All transaction methods fully implemented |

## connection

| Method | Status | Notes |
|--------|:------:|------|
| connection.start | ✅ | |
| connection.start-ok | ✅ | |
| connection.tune | ✅ | |
| connection.tune-ok | ✅ | |
| connection.open | ✅ | |
| connection.open-ok | ✅ | |
| connection.close | ✅ | |
| connection.close-ok | ✅ | |

## channel

| Method | Status | Notes |
|--------|:------:|------|
| channel.open | ✅ | |
| channel.open-ok | ✅ | |
| channel.flow | ✅ | Client-initiated flow control; server-initiated flow supported |
| channel.flow-ok | ✅ | |
| channel.close | ✅ | |
| channel.close-ok | ✅ | |

**Flow Control Features:**

- ✅ Client-initiated flow control (client pauses message delivery)
- ✅ Server-initiated flow control (server pauses client publishing)
- ✅ Per-channel flow state (independent control per channel)
- ✅ Integration with QoS throttling
- ✅ `basic.get` unaffected by flow (synchronous pull always works)
- ✅ `basic.consume` respects flow state (async delivery can be paused)
- ✅ Thread-safe flow state management

## exchange

| Method | Status | Notes |
|--------|:------:|------|
| exchange.declare | ✅ | Supports `direct`, `fanout`, and `topic` exchange types |
| exchange.declare-ok | ✅ | |
| exchange.delete | ✅ | |
| exchange.delete-ok | ✅ | |

**Topic Exchange Features:**

- ✅ Wildcard pattern matching with `*` (matches exactly one word)
- ✅ Wildcard pattern matching with `#` (matches zero or more words)
- ✅ Dot-separated routing key matching
- ✅ Multiple binding patterns per queue
- ✅ Complex pattern combinations (e.g., `*.error.#`, `log.*.database`)
- ✅ Malformed input validation (empty words, invalid patterns)

## queue

| Method | Status | Notes |
|--------|:------:|------|
| queue.declare | ✅ | |
| queue.declare-ok | ✅ | |
| queue.bind | ✅ | |
| queue.bind-ok | ✅ | |
| queue.unbind | ✅ | |
| queue.unbind-ok | ✅ | |
| queue.purge | ✅ | |
| queue.purge-ok | ✅ | |
| queue.delete | ✅ | |
| queue.delete-ok | ✅ | |

## basic

| Method | Status | Notes |
|--------|:------:|------|
| basic.qos | ✅ | |
| basic.qos-ok | ✅ | |
| basic.consume | ✅ | ‼️`noLocal` not supported (same as RabbitMQ)  |
| basic.consume-ok | ✅ | |
| basic.cancel | ✅ | |
| basic.cancel-ok | ✅ | |
| basic.publish | ✅ | |
| basic.return | ✅ | ‼️`immediate` flag is deprecated and will not be implemented |
| basic.deliver | ✅ | |
| basic.get | ✅ | |
| basic.get-ok | ✅ | |
| basic.get-empty | ✅ | |
| basic.ack | ✅ | |
| basic.reject | ✅ | Requeue works; dead-lettering implemented |
| basic.recover-async | ✅ | |
| basic.recover | ✅ | |
| basic.recover-ok | ✅ | |
| basic.nack | ✅ | *not part of amqp 0-9-1 specs |

## tx (Transactions)

| Method | Status | Notes |
|--------|:------:|------|
| tx.select | ✅ | Enter transaction mode (idempotent) |
| tx.select-ok | ✅ | |
| tx.commit | ✅ | Atomically commit buffered operations |
| tx.commit-ok | ✅ | |
| tx.rollback | ✅ | Discard buffered operations |
| tx.rollback-ok | ✅ | |

**Transaction Features:**

- ✅ Transactional publishing with buffering
- ✅ Transactional acknowledgments (ACK/NACK/REJECT)
- ✅ Multiple commits per transaction mode
- ✅ Channel close implicit rollback
- ✅ Mandatory message handling in transactions
- ✅ Mixed operations (publish + ack) in single transaction

---

## RabbitMQ Extensions

Beyond core AMQP 0.9.1, OtterMQ implements popular RabbitMQ extensions:

| Extension | Status | Configuration | Documentation |
|-----------|--------|---------------|---------------|
| **Dead Letter Exchange (DLX)** | ✅ Full | `OTTERMQ_ENABLE_DLX=true` | [DLX Guide](./dead-letter-exchange) |
| **Message TTL** | ✅ Full | `OTTERMQ_ENABLE_TTL=true` | [TTL Guide](./message-ttl) |

### Extension Details

#### Dead Letter Exchange (DLX)

- Queue arguments: `x-dead-letter-exchange`, `x-dead-letter-routing-key`
- Death reasons: `rejected`, `expired`, `maxlen` (future)
- Full `x-death` header tracking with count, time, reason, original queue/exchange
- Compatible with all RabbitMQ clients

#### Message TTL and Expiration

- Per-message TTL: `Expiration` property (milliseconds as string)
- Per-queue TTL: `x-message-ttl` queue argument
- Precedence: Per-message TTL takes priority over per-queue TTL
- Lazy expiration: Checked at retrieval time (delivery, basic.get, requeue)
- DLX integration: Expired messages routed to DLX with reason `"expired"`
- Compatible with RabbitMQ TTL semantics

---

**Notes:**

- Keep this table in sync with the implementation in `internal/core/amqp/*` and `internal/core/broker/*`.
- When adding or changing behavior, update the status and add notes on limitations or differences from RabbitMQ behavior.
- "Partial" (⚠️) means one or more optional behaviors/properties are not yet implemented.
