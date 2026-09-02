# Running QoS End-to-End Tests

## Quick Start

The e2e suite will start a test broker automatically by default. You can simply run the tests directly.

```bash
# Run all QoS/e2e tests (automatic broker startup)
go test -v ./tests/e2e -run TestQoS

# Run a specific test
go test -v ./tests/e2e -run TestQoS_PerConsumer_PrefetchLimit

# Run with race detector (recommended)
go test -v -race ./tests/e2e -run TestQoS
```

## Test Suite Overview

### Core QoS Tests

1. **TestQoS_PerConsumer_PrefetchLimit** - Basic per-consumer throttling
   - ✓ Verifies prefetch limit is enforced
   - ✓ Verifies ack releases a slot for delivery

2. **TestQoS_Global_ChannelLimit** - Channel-wide throttling
   - ✓ Multiple consumers share one prefetch pool
   - ✓ Channel-wide limit is enforced

3. **TestQoS_MultipleAck** - Batch acknowledgments
   - ✓ Acking with multiple=true releases multiple slots
   - ✓ Multiple messages delivered after batch ack

4. **TestQoS_Nack_WithRequeue** - Negative acknowledgments
   - ✓ Nacked messages are requeued
   - ✓ Redelivered flag is set correctly
   - ✓ QoS limits still apply to redelivered messages

5. **TestQoS_ZeroPrefetch_Unlimited** - Unlimited mode
   - ✓ prefetch=0 disables throttling
   - ✓ All messages delivered immediately

6. **TestQoS_ChangeLimit_MidConsume** - Dynamic QoS changes
   - ✓ Changing prefetch limit takes effect immediately
   - ✓ New limit is enforced for subsequent deliveries

### Helper Test

- **TestQoS_WithHelpers_Example** - Demonstrates test helper usage

## Expected Output

When all tests pass, you should see:
```
=== RUN   TestQoS_PerConsumer_PrefetchLimit
--- PASS: TestQoS_PerConsumer_PrefetchLimit (3.51s)
=== RUN   TestQoS_Global_ChannelLimit
--- PASS: TestQoS_Global_ChannelLimit (2.18s)
=== RUN   TestQoS_MultipleAck
--- PASS: TestQoS_MultipleAck (2.01s)
=== RUN   TestQoS_Nack_WithRequeue
--- PASS: TestQoS_Nack_WithRequeue (1.52s)
=== RUN   TestQoS_ZeroPrefetch_Unlimited
--- PASS: TestQoS_ZeroPrefetch_Unlimited (3.12s)
=== RUN   TestQoS_ChangeLimit_MidConsume
--- PASS: TestQoS_ChangeLimit_MidConsume (2.05s)
=== RUN   TestQoS_WithHelpers_Example
--- PASS: TestQoS_WithHelpers_Example (1.23s)
PASS
ok      github.com/ottermq/ottermq/tests/e2e        15.627s
```

## Troubleshooting

### "Connection refused"
- Make sure broker is running: `make run`
- Check broker is listening on port 5672: `netstat -an | grep 5672`

### "Test timeout"
- Check broker logs for panics or errors
- Increase test timeouts if running on slow hardware

### "Unexpected message count"
- Enable detailed logging: `OTTERMQ_LOG_LEVEL=debug make run`
- Check if messages are being throttled correctly in broker logs

## What the Tests Verify

### Protocol Compliance
- ✅ `basic.qos` method is properly handled
- ✅ `basic.qos-ok` response is sent
- ✅ prefetchCount parameter is respected
- ✅ global flag correctly switches between per-consumer and per-channel modes

### Delivery Behavior
- ✅ Messages stop being delivered when prefetch limit is reached
- ✅ Delivery resumes immediately after ack/nack
- ✅ Multiple ack correctly releases multiple slots
- ✅ Requeued messages maintain redelivered flag

### Edge Cases
- ✅ prefetch=0 means unlimited (no throttling)
- ✅ Changing QoS mid-consumption works correctly
- ✅ All consumers throttled doesn't lose messages
- ✅ Nack with requeue maintains delivery order

## CI Integration

Because the test suite starts a broker automatically, CI can simply run the tests directly without a separate broker process:

```bash
go test -v -race ./tests/e2e -run TestQoS
```

If you prefer to manage the broker process yourself in CI, you can still start the broker in the background and run tests against it (legacy/manual mode), but this is optional.
