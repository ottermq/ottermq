---
title: Message TTL and Expiration
---

## Message TTL and Expiration

OtterMQ supports Time-To-Live (TTL) for messages, allowing messages to expire automatically after a specified time. This feature is compatible with RabbitMQ's TTL implementation and provides two ways to set expiration:

1. **Per-Message TTL**: Set expiration on individual messages
2. **Per-Queue TTL**: Set a default expiration for all messages in a queue

> **Note**: This is a RabbitMQ extension feature and is not part of the core AMQP 0.9.1 specification.

## Configuration

### Enabling TTL

TTL support must be enabled via configuration:

```sh
export OTTERMQ_ENABLE_TTL=true
```

Or in your `.env` file:

```env
OTTERMQ_ENABLE_TTL=true
```

When disabled, TTL checks are skipped entirely (uses `NoOpTTLManager`).

## Per-Message TTL

Set the `Expiration` property when publishing a message. The value is a string representing milliseconds until expiration:

### Go Example (rabbitmq/amqp091-go)

```go
err := ch.Publish(
    "",           // exchange
    "my-queue",   // routing key
    false,        // mandatory
    false,        // immediate
    amqp.Publishing{
        ContentType: "text/plain",
        Body:        []byte("This message expires in 5 seconds"),
        Expiration:  "5000", // TTL in milliseconds
    },
)
```

### .NET Example (RabbitMQ.Client)

```csharp
var properties = channel.CreateBasicProperties();
properties.Expiration = "5000"; // 5 seconds in milliseconds

channel.BasicPublish(
    exchange: "",
    routingKey: "my-queue",
    basicProperties: properties,
    body: Encoding.UTF8.GetBytes("This message expires in 5 seconds")
);
```

### Python Example (pika)

```python
import pika

connection = pika.BlockingConnection(pika.ConnectionParameters('localhost'))
channel = connection.channel()

properties = pika.BasicProperties(
    expiration='5000'  # 5 seconds in milliseconds
)

channel.basic_publish(
    exchange='',
    routing_key='my-queue',
    body='This message expires in 5 seconds',
    properties=properties
)
```

## Per-Queue TTL

Set the `x-message-ttl` argument when declaring a queue. All messages routed to this queue will have this TTL unless they specify their own:

### Go Example

```go
args := amqp.Table{
    "x-message-ttl": int32(10000), // 10 seconds for all messages
}

queue, err := ch.QueueDeclare(
    "my-queue-with-ttl",
    false,  // durable
    true,   // auto-delete
    false,  // exclusive
    false,  // no-wait
    args,
)
```

### .NET Example

```csharp
var args = new Dictionary<string, object>
{
    { "x-message-ttl", 10000 } // 10 seconds
};

channel.QueueDeclare(
    queue: "my-queue-with-ttl",
    durable: false,
    exclusive: false,
    autoDelete: true,
    arguments: args
);
```

## TTL Precedence

When both per-message and per-queue TTL are set:

- **Per-message TTL takes precedence** over per-queue TTL
- A message with `Expiration: "100"` will expire in 100ms even if the queue has `x-message-ttl: 10000`

## Expiration Behavior

### Lazy Expiration

OtterMQ uses **lazy expiration** - messages are checked for expiration when:

- They are about to be delivered to a consumer (`basic.consume`)
- They are retrieved via `basic.get`
- They are about to be requeued (`basic.nack`/`basic.reject` with requeue=true)
- They are recovered from persistence on broker restart

Messages do **not** expire while sitting idle in a queue. This matches RabbitMQ's behavior for per-message TTL.

### Expired Message Handling

When a message expires:

1. **If a Dead Letter Exchange (DLX) is configured**: The message is routed to the DLX with reason `"expired"`
2. **If no DLX is configured**: The message is silently discarded

### Example: Expiration with Dead Letter Exchange

```go
// Declare dead letter exchange
err := ch.ExchangeDeclare("my-dlx", "direct", false, true, false, false, nil)

// Declare dead letter queue
dlq, err := ch.QueueDeclare("my-dlq", false, true, false, false, nil)

// Bind DLQ to DLX
err = ch.QueueBind(dlq.Name, "expired", "my-dlx", false, nil)

// Declare main queue with DLX and TTL
args := amqp.Table{
    "x-message-ttl":             int32(5000),  // 5 second TTL
    "x-dead-letter-exchange":    "my-dlx",
    "x-dead-letter-routing-key": "expired",
}
mainQueue, err := ch.QueueDeclare("my-queue", false, true, false, false, args)

// Publish a message
err = ch.Publish("", mainQueue.Name, false, false, amqp.Publishing{
    Body: []byte("Will expire and go to DLQ"),
})

// Wait for expiration (> 5 seconds)
time.Sleep(6 * time.Second)

// Consume from DLQ - expired message should be there
msgs, err := ch.Consume(dlq.Name, "", false, false, false, false, nil)
for msg := range msgs {
    // Check x-death headers for expiration information
    xDeath := msg.Headers["x-death"]
    fmt.Printf("Message expired: %v\n", xDeath)
    msg.Ack(false)
    break
}
```

## Dead Letter Headers

Expired messages that are dead-lettered include comprehensive death tracking in the `x-death` header array:

```json
{
  "reason": "expired",
  "queue": "original-queue-name",
  "time": "2025-11-16T15:30:00Z",
  "exchange": "original-exchange",
  "routing-keys": ["original.routing.key"],
  "count": 1,
  "original-expiration": "5000"
}
```

Additional headers:

- `x-first-death-queue`: The first queue where the message was dead-lettered
- `x-first-death-reason`: The first death reason (e.g., "expired")
- `x-first-death-exchange`: The first exchange the message was published to

## Implementation Details

### Internal Flow

1. **Message Reception**: When a message with `Expiration` is received, it's converted to an absolute timestamp (Unix milliseconds)
2. **Storage**: The message is stored with its `EnqueuedAt` timestamp for per-queue TTL calculations
3. **Expiration Check**: At retrieval time:
   - Check per-message TTL: `now >= expirationTimestamp`
   - If no per-message TTL, check per-queue TTL: `age >= x-message-ttl`
4. **Expiration Handling**:
   - If expired and DLX configured: Route to DLX with reason `"expired"`
   - If expired and no DLX: Delete message
5. **Delivery**: Only non-expired messages are delivered to consumers

### Key Components

- **`TTLManager` interface**: Provides `CheckExpiration()` method
- **`DefaultTTLManager`**: Implements TTL checking logic
- **`NoOpTTLManager`**: Used when TTL is disabled (no-op implementation)
- **Extension registry**: TTL is controlled via `EnableTTL` configuration flag

### Performance Considerations

- TTL checking is **O(1)** per message at retrieval time
- No background scanning or active expiration
- Expired messages remain in memory until attempted retrieval
- For queues with many expired messages, consider periodic consumption or `queue.purge`

## Limitations and Notes

1. **Lazy expiration**: Messages don't expire "in place" - they expire when checked
2. **Memory usage**: Expired messages consume memory until retrieved or purged
3. **No queue-level expiration sweeper**: Unlike some brokers, there's no background process actively removing expired messages
4. **Persistent expired messages**: If messages are persistent, they're deleted from storage when expired

## Testing

OtterMQ includes comprehensive TTL test coverage:

- **Unit tests**: 14 tests covering `CheckExpiration()` logic and argument parsing
- **E2E tests**: 10 tests covering all TTL scenarios with RabbitMQ clients

Run tests:

```sh
# Unit tests
go test ./internal/core/broker/vhost/ -run TTL -v

# E2E tests  
go test ./tests/e2e/ -run TestTTL -v
```

## Compatibility

TTL implementation is fully compatible with:

- RabbitMQ clients (Go, .NET, Python, Node.js)
- RabbitMQ TTL semantics (lazy expiration, precedence rules)
- Dead Letter Exchange integration

## Related Documentation

- [Dead Letter Exchange (DLX)](./dead-letter-exchange)
- [AMQP Status Matrix](./amqp-status)
- [Project Overview](./index)

## Configuration Reference

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `OTTERMQ_ENABLE_TTL` | `false` | Enable message TTL and expiration support |
| `OTTERMQ_ENABLE_DLX` | `false` | Enable Dead Letter Exchange (recommended with TTL) |

---

**Last Updated**: November 16, 2025  
**Feature Version**: v0.13.0+
