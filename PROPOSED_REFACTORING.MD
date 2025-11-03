# Proposal: Refactor Exchange Bindings to Unified `[]Binding` Structure

## 📌 Summary

OtterMQ currently uses a `map[string][]*Queue` structure to represent bindings in direct and topic exchanges, and a separate `map[string]*Queue` for fanout. This proposal suggests refactoring to a unified `[]Binding` structure to simplify routing, persistence, and future extensibility.

---

## 📦 Current Structure

```go
Bindings map[string][]*Queue // for direct/topic
Queues map[string]*Queue     // for fanout
```

## 🧱 Proposed Structure

```go
type Binding struct {
    Queue      *Queue
    RoutingKey string
    Arguments  map[string]interface{}
}

type Exchange struct {
    Name      string
    Type      ExchangeType
    Bindings  []Binding
    Props     *ExchangeProperties
}
```

## ✅ Benefits

- Unified logic across direct, fanout, topic, and future exchange types
- Simplifies routing: one loop, one matcher per exchange type
- Supports headers exchange via Arguments
- Improves persistence: serialize []Binding directly
- Enables future features: TTLs, priorities, plugin metadata

🔁 Routing Behavior by Exchange Type

| Exchange Type | Routing Behavior | Binding Usage |
|---------------|------------------|---------------|
| direct | Match RoutingKey == msg.RoutingKey | Filter bindings by routing key |
|fanout | Broadcast to all queues | Ignore routing key, deliver to all |
|topic | Pattern match routing key | Use matchTopic(binding.RoutingKey) |
|headers | Match message headers | Use matchHeaders(binding.Arguments) |

## 🛠️ Migration Plan

1. Replace `map[string][]*Queue` with `[]Binding`
2. Update `BindQueue` and `UnbindQueue` to match full binding spec
3. Refactor persistence layer to store `[]Binding`
4. Update routing logic to iterate over `[]Binding`
5. Add helper functions for matching by exchange type

## 🧠 Future Extensions

```go
type Binding struct {
    Queue      *Queue
    RoutingKey string
    Arguments  map[string]interface{}
    Timestamp  time.Time
    Priority   int
    PluginData map[string]interface{}
}
```

## 📌 Status

This proposal is under consideration. Current implementation ignores Arguments and uses separate structures for fanout vs direct. Refactor would unify and future-proof the binding model.
