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
  - [x] Channel flow control (`CHANNEL_FLOW`, `CHANNEL_FLOW_OK`)
  - [x] Backpressure handling with flow state
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
- [x] **Transaction Support**
  - [x] `TX_SELECT` - Enter transaction mode
  - [x] `TX_COMMIT` - Commit buffered operations
  - [x] `TX_ROLLBACK` - Rollback buffered operations
  - [x] Transactional publishing with message buffering
  - [x] Transactional acknowledgments (ACK/NACK/REJECT)
  - [x] Multiple commits per transaction mode
  - [x] Channel close implicit rollback
  - [x] Mandatory message handling in transactions

### ⚡ **Recently Completed**

- [x] **Channel Flow Control** - Client and server-initiated flow control
  - [x] `CHANNEL_FLOW` - Channel-level flow control with active flag
  - [x] `CHANNEL_FLOW_OK` - Flow control acknowledgment
  - [x] Per-channel flow state tracking
  - [x] Integration with delivery throttling (shouldThrottle)
  - [x] Thread-safe flow state management
  - [x] Flow initiator tracking (client vs server)
- [x] **Transaction Support (TX class)** - Full AMQP transaction implementation
  - [x] Transaction mode selection per channel
  - [x] Operation buffering (publish, ack, nack, reject)
  - [x] Atomic commit with delivery tracking
  - [x] Rollback with proper state cleanup
  - [x] Mixed operations in single transaction
- [x] **`QUEUE_DELETE` enhancements** - if-unused and if-empty flags
- [x] **`QUEUE_PURGE`** - Clear queue contents with persistent message deletion
- [x] **`QUEUE_UNBIND`** - Remove queue bindings with argument matching
- [x] **Binding Structure Refactoring** - Unified binding with argument validation
- [x] **Consumer management** - Push/pull consumption with QoS support
- [x] **Message acknowledgments** - ACK, NACK, REJECT, and RECOVER

### ❌ **Missing Features**

#### **Phase 1: Advanced Features (High Priority)**

- [ ] **Message TTL and expiration**
- [ ] **Dead letter exchanges**
- [ ] **Priority queues**
- [ ] **Topic exchange pattern matching**

#### **Phase 2: Clustering (Lower Priority)**

- [ ] **Cluster support**
- [ ] **Queue mirroring**
- [ ] **Federated exchanges**

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

### ✅ **Phase 1-4: Core Messaging & Transactions (COMPLETED)**

**Completed implementations**:

- Push and pull-based message consumption
- Message acknowledgments and recovery
- Quality of Service (QoS) with prefetch limits
- Consumer lifecycle management
- Delivery tracking and flow control
- Full transaction support (TX class)
- Atomic commit/rollback of operations
- Transaction mode per channel

**Key files**:

- `internal/core/amqp/tx.go` - TX protocol parsing
- `internal/core/broker/tx.go` - TX handler implementation
- `internal/core/broker/basic.go` - Transaction-aware publishing/acking
- `tests/e2e/tx_test.go` - Comprehensive transaction tests

### **Phase 5: Flow Control & Performance (COMPLETED)**

**Completed implementations**:

- Channel flow control (`CHANNEL_FLOW`, `CHANNEL_FLOW_OK`)
- Client-initiated flow (client pauses message delivery)
- Server-initiated flow (server pauses client publishing)
- Per-channel flow state management
- Integration with QoS throttling
- Comprehensive test coverage

**Key files**:

- `internal/core/amqp/channel.go` - Channel flow parsing
- `internal/core/broker/channel.go` - Flow handler implementation
- `internal/core/broker/vhost/delivery.go` - Flow state management
- `tests/internal/core/broker/vhost/channel_flow_test.go` - Flow tests

### **Phase 6: Advanced Features & Performance (Current Focus)**

**Goal**: Message lifecycle features and optimization

**Tasks**:

1. Implement message TTL and expiration
2. Add dead letter exchanges
3. Implement priority queues
4. Complete topic exchange pattern matching
5. Performance profiling and optimization

## Testing Strategy

### **Compatibility Testing**

- [x] Test with official RabbitMQ clients
  - [x] `rabbitmq/amqp091-go` (working with all TX features)
  - [ ] RabbitMQ .NET Client
  - [ ] Python `pika` library
  - [ ] Node.js `amqplib`

### **Integration Testing**

- [x] End-to-end message flow tests
- [x] Multi-consumer scenarios
- [x] Transaction commit/rollback tests
- [ ] High-throughput stress testing
- [ ] Failure recovery testing

### **Performance Benchmarks**

- [ ] Message throughput comparison with RabbitMQ
- [ ] Memory usage under load
- [ ] Connection handling capacity
- [ ] Transaction overhead measurement

## Contributing

### **Current Priority**

The highest priority is **Phase 6: Advanced Features & Performance**. Contributors should focus on:

1. Message TTL and expiration mechanisms
2. Dead letter exchange implementation
3. Priority queue support
4. Topic exchange pattern matching completion
5. Performance profiling and optimization

### **Getting Started**

1. Review `internal/core/amqp/` for protocol-level types
2. Check existing handler patterns in `internal/core/broker/`
3. See `.github/copilot-instructions.md` for architecture patterns
4. Study message lifecycle in `internal/core/broker/vhost/`

### **Code Guidelines**

- Follow existing AMQP frame processing patterns
- Add comprehensive error handling
- Include unit tests for new parsers
- Test with RabbitMQ clients for compatibility
- Ensure proper state isolation per channel

---

## Progress Tracking

**Last Updated**: November 2024  
**Current Focus**: Phase 6 - Advanced Features & Performance  
**Completed**: All CONNECTION, CHANNEL (including flow control), EXCHANGE, QUEUE, BASIC, and TX class methods  
**Next Milestone**: Message TTL, dead letter exchanges, and topic pattern matching

For detailed implementation tasks, see GitHub Issues tagged with the respective phase labels.
