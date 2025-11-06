# End-to-End Tests

This directory contains end-to-end tests that verify OtterMQ's AMQP protocol implementation using a real RabbitMQ-compatible client (`amqp091-go`).

## Broker Startup

You usually do NOT need to start a broker manually. The suite's `TestMain` (`tests/e2e/setup_test.go`) boots an ephemeral OtterMQ instance with a temporary data directory and shuts it down after tests finish.

Optional manual mode (if you want to point tests at an already running broker):

```bash
make build && make run  # or just: make run
```

## Running Tests

Automatic broker startup:

```bash
go test -v ./tests/e2e/...
```

Single test by name:

```bash
go test -v ./tests/e2e -run TestQoS_PerConsumer_PrefetchLimit
```

Race detector:

```bash
go test -v -race ./tests/e2e/...
```

## Representative QoS Scenarios

1. Per-consumer prefetch limit: publish 10, prefetch=3 => only 3 unacked at a time.
2. Global (channel) prefetch: prefetch=5 shared across multiple consumers on one channel.
3. Multiple ack: `Ack(multiple=true)` releases several slots in one call.
4. Nack with requeue: message requeued with proper `redelivered` flag while honoring QoS.
5. Zero prefetch: unlimited delivery (prefetch=0).
6. Mid-stream QoS change: increase prefetch and observe immediate effect.

## queue.purge Scenario

`TestQueuePurge_RemovesAllMessagesAndReturnsCount` validates purge semantics and return count.
Persistent durable purge behavior is currently validated at unit level; e2e durable variant temporarily skipped pending binding investigation.

## Expectations

- Broker listens on `localhost:5672`
- Default credentials `guest/guest`
- Tests use auto-delete queues unless durability is required
- Each test isolates its resources (unique queue names)

## Troubleshooting

Connection refused: ensure no port conflict and credentials match.
Timeouts: inspect broker logs; increase per-test timeouts if on slow hardware.
Flaky timing: adjust `time.After` windows or add small sleeps where necessary.

## Adding New Tests

1. Use descriptive, unique queue names (e.g., `test.recover.nack.requeue`)
2. Prefer auto-delete for cleanup
3. Keep timeouts modest (≤2s typical)
4. Use `t.Logf` for interim diagnostic output
5. Close channels/connections with `defer`

