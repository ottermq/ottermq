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
| connection | 100% | Handshake and basic lifecycle supported |
| channel | 67% | Basic open/close implemented; flow control not yet implemented |
| exchange | 80% | direct/fanout declare implemented; missing topic pattern matching |
| queue | 85% | declare/bind/unbind/delete with argument validation; purge planned |
| basic | 100% | All methods fully implemented and tested |
| tx | 0% | Transaction support planned |

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
| channel.flow | ❌ | Flow control not yet implemented |
| channel.flow-ok | ❌ | |
| channel.close | ✅ | |
| channel.close-ok | ✅ | |

## exchange

| Method | Status | Notes |
|--------|:------:|------|
| exchange.declare | ⚠️ | Supports `direct`/`fanout`; missing `topic` |
| exchange.declare-ok | ✅ | |
| exchange.delete | ✅ | |
| exchange.delete-ok | ✅ | |

## queue

| Method | Status | Notes |
|--------|:------:|------|
| queue.declare | ✅ | |
| queue.declare-ok | ✅ | |
| queue.bind | ✅ | |
| queue.bind-ok | ✅ | |
| queue.unbind | ✅ | Raises channel exception on errors; see notes below |
| queue.unbind-ok | ✅ | |
| queue.purge | ❌ | |
| queue.purge-ok | ❌ | |
| queue.delete | ⚠️ | Basic deletion works; `if-unused`/`if-empty` flags TODO |
| queue.delete-ok | ✅ | |

**queue.unbind Implementation Notes:**

- Returns 404 NOT_FOUND if exchange, queue, or binding doesn't exist
- ✅ **Argument matching implemented** - Bindings are uniquely identified by queue+exchange+routingKey+arguments
- Returns 404 NOT_FOUND if binding with matching arguments doesn't exist during unbind
- Supports DIRECT and FANOUT exchange types
- Triggers exchange auto-delete if `auto-delete=true` and no bindings remain
- Treats `nil` and empty `{}` arguments as equivalent (AMQP standard behavior)
- **TODO**: Queue exclusivity validation (will raise 403 ACCESS_REFUSED)

**queue.bind Implementation Notes:**

- ✅ **Duplicate binding prevention** - Returns 406 PRECONDITION_FAILED when attempting to create an identical binding (same queue+exchange+routingKey+arguments)
- Supports binding arguments for future headers exchange support

## basic

| Method | Status | Notes |
|--------|:------:|------|
| basic.qos | ✅ | Not yet implemented |
| basic.qos-ok | ✅ | |
| basic.consume | ✅ | ‼️`noLocal` not supported (same as RabbitMQ)  |
| basic.consume-ok | ✅ | |
| basic.cancel | ✅ | |
| basic.cancel-ok | ✅ | |
| basic.publish | ✅ | |
| basic.return | ✅ | ‼️`immediate` flag is deprecated and will not be implemented |
| basic.deliver | ✅ | |
| basic.get | ✅ | Pull-based message retrieval |
| basic.get-ok | ✅ | |
| basic.get-empty | ✅ | |
| basic.ack | ✅ | Supports multiple flag |
| basic.reject | ⚠️ | Requeue works; dead-lettering TODO |
| basic.recover-async | ✅ | |
| basic.recover | ✅ | |
| basic.recover-ok | ✅ | |
| basic.nack | ✅ | *not part of amqp 0-9-1 specs |

## tx (Transactions)

| Method | Status | Notes |
|--------|:------:|------|
| tx.select | ❌ | Transaction support not yet implemented |
| tx.select-ok | ❌ | |
| tx.commit | ❌ | |
| tx.commit-ok | ❌ | |
| tx.rollback | ❌ | |
| tx.rollback-ok | ❌ | |

---

**Notes:**

- Keep this table in sync with the implementation in `internal/core/amqp/*` and `internal/core/broker/*`.
- When adding or changing behavior, update the status and add notes on limitations or differences from RabbitMQ behavior.
- "Partial" (⚠️) means one or more optional behaviors/properties are not yet implemented.
