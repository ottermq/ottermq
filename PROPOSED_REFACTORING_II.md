# TODO 2: Queue Exclusivity Check (403 ACCESS_REFUSED)

## 📌 Summary

OtterMQ currently allows any connection to interact with any queue. However, AMQP 0-9-1 specifies that **exclusive queues must only be accessible by the connection that declared them**. This proposal introduces a mechanism to enforce exclusivity and raise a `403 ACCESS_REFUSED` error when violated.

---

## 🔐 Problem

- Exclusive queues are not protected from access by other connections.
- This violates AMQP semantics and can lead to race conditions or unexpected behavior.

---

## 🧱 Proposed Changes

### 1. Track Queue Ownership

Add an `OwnerConn net.Conn` field to the `Queue` struct:

```go
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

### 2. Set Owner on Declaration

When creating an exclusive queue, set the owner:

```go
if props.Exclusive {
    queue.OwnerConn = conn  // Pass conn from handler
}
```

This requires `CreateQueue` to accept a `conn net.Conn` parameter.

### 3. Pass Connection to Bind/Unbind

Update method signatures:

```go
func (vh *VHost) BindQueue(exchangeName, queueName, routingKey string, args map[string]interface{}, conn net.Conn) error
func (vh *VHost) UnbindQueue(exchangeName, queueName, routingKey string, args map[string]interface{}, conn net.Conn) error
```

### 4. Validate Exclusivity

Before binding or unbinding, check ownership:

```go
if queue.Props.Exclusive && queue.OwnerConn != nil && queue.OwnerConn != conn {
    return errors.NewChannelError(
        fmt.Sprintf("queue '%s' is exclusive to another connection", queueName),
        uint16(amqp.ACCESS_REFUSED),  // 403
        uint16(amqp.QUEUE),
        uint16(amqp.QUEUE_UNBIND),
    )
}
```

### 5. Clean Up on Connection Close

When a connection closes, release or delete exclusive queues:

```go
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

## ✅ Benefits

- Enforces AMQP exclusivity rules
- Prevents unauthorized access to exclusive queues
- Supports auto-delete semantics on connection close
- Aligns OtterMQ with RabbitMQ behavior and client expectations

---

### 📌 Status

This proposal is under implementation. Current behavior does not enforce exclusivity. This change will ensure protocol compliance and improve queue isolation.
