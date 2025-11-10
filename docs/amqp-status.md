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
| channel | 67% | Basic open/close implemented; flow control not yet implemented |
| exchange | 80% | direct/fanout implemented; topic pattern matching TODO |
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
| basic.reject | ⚠️ | Requeue works; dead-lettering TODO |
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

**Notes:**

- Keep this table in sync with the implementation in `internal/core/amqp/*` and `internal/core/broker/*`.
- When adding or changing behavior, update the status and add notes on limitations or differences from RabbitMQ behavior.
- "Partial" (⚠️) means one or more optional behaviors/properties are not yet implemented.
