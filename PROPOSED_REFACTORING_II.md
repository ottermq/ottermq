# TODO 1: Queue Exclusivity Check (403 ACCESS_REFUSED)

Current Issue: Exclusive queues should only be accessible by the connection that declared them. Operations from other connections should raise 403.

## Sumary

1. Add OwnerConn net.Conn to Queue struct
2. Set owner when declaring exclusive queues
3. Pass conn through bind/unbind operations
4. Validate connection matches owner before operations
5. Clean up on connection close

Approach:

### Step 1: Track Queue Owner

Add owner tracking to `Queue` and `QueueProperties`:

```go
// In internal/core/broker/vhost/queue.go
type Queue struct {
    Name           string
    Props          *QueueProperties
    messages       chan amqp.Message
    count          int
    mu             sync.Mutex
    OwnerConn      net.Conn  // NEW: Track owner connection for exclusive queues
    deliveryCtx    context.Context
    deliveryCancel context.CancelFunc
    delivering     bool
}
```

### Step 2: Set Owner on Queue Declaration

```
// In internal/core/broker/vhost/queue.go - CreateQueue function
if props.Exclusive {
    queue.OwnerConn = conn  // Pass conn from handler
}
```

This means CreateQueue needs a `conn net.Conn` parameter when creating exclusive queues.

### Step 3: Add Connection Context to Bind/Unbind

```go
// In internal/core/broker/vhost/binding.go
func (vh *VHost) BindQueue(exchangeName, queueName, routingKey string, args map[string]interface{}, conn net.Conn) error
func (vh *VHost) UnbindQueue(exchangeName, queueName, routingKey string, args map[string]interface{}, conn net.Conn) error
```

### Step 4: Exclusivity Validation

```go
func (vh *VHost) UnbindQueue(exchangeName, queueName, routingKey string, args map[string]interface{}, conn net.Conn) error {
    vh.mu.Lock()
    defer vh.mu.Unlock()

    // ... existing exchange/queue lookup ...

    // Check exclusivity
    if queue.Props.Exclusive && queue.OwnerConn != nil && queue.OwnerConn != conn {
        return errors.NewChannelError(
            fmt.Sprintf("queue '%s' is exclusive to another connection", queueName),
            uint16(amqp.ACCESS_REFUSED),  // 403
            uint16(amqp.QUEUE),
            uint16(amqp.QUEUE_UNBIND),
        )
    }

    // ... rest of unbind logic ...
}
```

### Step 5: Clean Up Owner on Connection Close

When a connection closes, clear the owner from exclusive queues (or delete them based on auto-delete/exclusive semantics):

```go
// In connection cleanup code
for _, queue := range vh.Queues {
    if queue.OwnerConn == conn {
        if queue.Props.AutoDelete {
            vh.DeleteQueue(queue.Name)
        } else {
            queue.OwnerConn = nil  // Release ownership
        }
    }
}
```

---

