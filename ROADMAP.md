# OtterMQ Development Roadmap

## Overview

OtterMQ aims to be a fully AMQP 0.9.1 compliant message broker with RabbitMQ compatibility. This roadmap tracks our progress toward complete protocol implementation and production readiness.

## AMQP 0.9.1 Implementation Status

### ✅ **Implemented Features**

- [x] **Connection Management**
  - [x] Protocol handshake and negotiation
  - [x] SASL PLAIN authentication
  - [x] Connection heartbeat handling
  - [x] Graceful connection close
- [x] **Channel Operations**
  - [x] Channel open/close lifecycle
  - [x] Multi-channel support per connection
- [x] **Exchange Management**
  - [x] Exchange declare/delete
  - [x] Direct, fanout, topic exchange types
  - [x] Mandatory exchanges (default, amq.*)
- [x] **Queue Management**
  - [x] Queue declare/delete with properties
  - [x] Queue binding to exchanges
  - [x] Message buffering and storage
- [x] **Basic Publishing & Consumption**
  - [x] `BASIC_PUBLISH` with routing
  - [x] `BASIC_CONSUME` - Push-based consumption
  - [x] `BASIC_DELIVER` - Server-initiated message delivery
  - [x] `BASIC_CANCEL` - Cancel consumer subscription
  - [x] `BASIC_GET` - Pull-based consumption
  - [x] Multi-frame message assembly (method + header + body)
  - [x] Message properties and headers
  - [x] Message count reporting
- [x] **Message Acknowledgments & Recovery**
  - [x] `BASIC_ACK` - Acknowledge messages (single and multiple)
  - [x] `BASIC_NACK` - Negative acknowledgment with requeue
  - [x] `BASIC_REJECT` - Reject single message with requeue
  - [x] `BASIC_RECOVER` - Redeliver unacknowledged messages
  - [x] `BASIC_RECOVER_ASYNC` - Async message recovery
  - [x] Delivery tag tracking and management
- [x] **Quality of Service**
  - [x] `BASIC_QOS` - Prefetch count limits
  - [x] Per-consumer prefetch control (global=false)
  - [x] Channel-wide prefetch control (global=true)
  - [x] Message throttling and flow control

### � **Recently Completed**

- [x] Consumer management system refactoring
- [x] **`BASIC_CONSUME`** - Start consuming messages from queue
- [x] **`BASIC_DELIVER`** - Server-initiated message delivery
- [x] **`BASIC_CANCEL`** - Cancel consumer subscription
- [x] **`QUEUE_UNBIND`** - Remove queue bindings (Phase 2 completion)
  - [x] Support for DIRECT and FANOUT exchanges
  - [x] Proper error handling with channel exceptions
  - [x] Auto-delete exchange when last binding is removed
  - [x] Comprehensive unit test coverage

### ❌ **Missing Features**

#### **Phase 1: Transaction Support (High Priority)**

- [ ] **`TX_SELECT`** - Enter transaction mode
- [ ] **`TX_COMMIT`** - Commit transaction
- [ ] **`TX_ROLLBACK`** - Rollback transaction
- [ ] **Transactional publishing/consuming**

#### **Phase 2: Advanced Queue Operations (Medium Priority)**

- [ ] **`QUEUE_UNBIND` enhancements** - Argument validation and exclusivity checks
  - [ ] Binding argument matching (406 PRECONDITION_FAILED on mismatch)
  - [ ] Queue exclusivity validation (403 ACCESS_REFUSED for wrong connection)
- [ ] **`QUEUE_PURGE`** - Clear queue contents
- [ ] **`QUEUE_DELETE` improvements** - Support if-unused and if-empty flags

#### **Phase 3: Flow Control (Medium Priority)**

- [ ] **`CHANNEL_FLOW`** - Channel-level flow control
- [ ] **`CHANNEL_FLOW_OK`** - Flow control acknowledgment
- [ ] Backpressure handling integration

#### **Phase 4: Advanced Features (Lower Priority)**

- [ ] **Message TTL and expiration**
- [ ] **Dead letter exchanges**
- [ ] **Priority queues**
- [ ] **Topic exchange pattern matching**
- [ ] **Cluster support**

## Architecture Improvements

### **Persistence Layer**

- [ ] **Swappable Persistence Architecture** - Move to pluggable persistence backends
  - [ ] Refactor current JSON implementation to `pkg/persistence/implementations/json/`
  - [ ] Create abstract persistence interface for multiple backends
  - [ ] Configuration-based persistence selection
- [ ] **Memento WAL Engine** - Custom append-only transaction log (Long-term)
  - [ ] WAL-based persistence inspired by RabbitMQ's Mnesia approach
  - [ ] Message event streaming (publish/ack/reject)
  - [ ] Crash recovery via log replay
  - [ ] Periodic state snapshots for performance
  - [ ] Foundation for future clustering capabilities
- [ ] **Recovery system** - Restore state after restart
  - [ ] Durable queues and exchanges
  - [ ] Persistent message recovery

### **Performance & Scalability**

- [ ] **Connection pooling** optimizations
- [ ] **Memory management** for high-throughput scenarios
- [ ] **Metrics and monitoring** integration
- [ ] **Load testing** and benchmarking

### **Management & Observability**

- [ ] **Enhanced Web UI** features
  - [ ] Real-time connection monitoring
  - [ ] Message flow visualization
  - [ ] Performance metrics dashboard
- [ ] **REST API** completeness
  - [ ] Full queue/exchange management
  - [ ] Consumer monitoring endpoints
- [ ] **Logging improvements**
  - [ ] Structured logging with correlation IDs
  - [ ] Debug modes for protocol tracing

## Development Phases

### ✅ **Phase 1-3: Core Messaging (COMPLETED)**

**Completed implementations**:

- Push and pull-based message consumption
- Message acknowledgments and recovery
- Quality of Service (QoS) with prefetch limits
- Consumer lifecycle management
- Delivery tracking and flow control

### **Phase 4: Transactions (Current Focus)**

**Goal**: ACID message operations

**Tasks**:

1. Implement transaction state tracking
2. Add transactional publishing
3. Implement commit/rollback logic
4. Add transaction mode per channel

**Files to modify**:

- `internal/core/amqp/tx.go` (create)
- `internal/core/broker/tx.go` (create)
- `internal/core/broker/vhost/transaction.go` (create)

### **Phase 5: Advanced Queue Operations**

**Goal**: Complete queue management

**Tasks**:

1. Implement `QUEUE_UNBIND`
2. Add `QUEUE_PURGE` functionality
3. Enhance `QUEUE_DELETE` with conditional flags

### **Phase 6: Flow Control & Performance**

**Goal**: Channel flow control and optimization

**Tasks**:

1. Implement `CHANNEL_FLOW` and `CHANNEL_FLOW_OK`
2. Add backpressure handling
3. Performance profiling and optimization

## Testing Strategy

### **Compatibility Testing**

- [ ] Test with official RabbitMQ clients
  - [ ] `rabbitmq/amqp091-go` (already working)
  - [ ] RabbitMQ .NET Client
  - [ ] Python `pika` library
  - [ ] Node.js `amqplib`

### **Integration Testing**

- [ ] End-to-end message flow tests
- [ ] Multi-consumer scenarios
- [ ] High-throughput stress testing
- [ ] Failure recovery testing

### **Performance Benchmarks**

- [ ] Message throughput comparison with RabbitMQ
- [ ] Memory usage under load
- [ ] Connection handling capacity

## Contributing

### **Current Priority**

The highest priority is **Phase 4: Transaction Support**. Contributors should focus on:

1. Transaction state management per channel
2. `TX_SELECT`, `TX_COMMIT`, `TX_ROLLBACK` implementation
3. Transactional message publishing and consumption

### **Getting Started**

1. Review `internal/core/amqp/tx.go` for transaction-related types
2. Check `internal/core/broker/tx.go` for transaction handler implementation patterns
3. See `.github/copilot-instructions.md` for architecture patterns
4. Study existing channel state management in `internal/core/broker/vhost/`

### **Code Guidelines**

- Follow existing AMQP frame processing patterns
- Add comprehensive error handling
- Include unit tests for new parsers
- Test with RabbitMQ clients for compatibility
- Ensure transaction state is properly isolated per channel

---

## Progress Tracking

**Last Updated**: October 28, 2025  
**Current Focus**: Phase 4 - Transaction Support  
**Completed**: All basic class methods (consume, deliver, ack, nack, reject, recover, qos)  
**Next Milestone**: Transaction support (`TX_SELECT`, `TX_COMMIT`, `TX_ROLLBACK`)

For detailed implementation tasks, see GitHub Issues tagged with the respective phase labels.
