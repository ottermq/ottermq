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
  - [x] Direct, fanout, topic exchange types with full pattern matching
  - [x] Topic exchange wildcards (`*` and `#`)
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

- [x] **Management API Refactoring** - Professional service layer architecture (v0.15.0)
  - [x] Complete separation of HTTP handlers from broker logic
  - [x] Service layer in `internal/core/broker/management/` with 11 service modules
  - [x] Zero AMQP protocol overhead for management operations
  - [x] Full feature exposure: Queues, Exchanges, Bindings, Consumers, Channels, Connections, Messages
  - [x] Complete DTOs with all properties (TTL, DLX, QLL, QoS, consumer counts, unacked messages)
  - [x] Structured binding operations (no raw maps)
  - [x] Consumer visibility API (list all, list by queue, detailed consumer info)
  - [x] Channel monitoring (list all, list by connection, channel details)
  - [x] Message operations with full AMQP properties (publish with TTL/priority/headers, get with ack modes)
  - [x] Connection management (list, get details, close connections)
  - [x] VHost operations with statistics
  - [x] Overview endpoint (broker/node/object totals, message stats, connection stats)
  - [x] BrokerProvider interface pattern to avoid circular dependencies
  - [x] Thread-safe operations with proper lock management
  - [x] Comprehensive test coverage (52 tests, 66% coverage)
  - [x] Updated Swagger documentation
  - [x] API examples in README with curl commands
- [x] **Message TTL and Expiration** - RabbitMQ-compatible time-to-live for messages (v0.13.0)
  - [x] Per-message TTL via `Expiration` property (relative milliseconds)
  - [x] Per-queue TTL via `x-message-ttl` argument
  - [x] TTL precedence rules (per-message takes priority)
  - [x] Lazy expiration at retrieval time (delivery, basic.get, requeue, recovery)
  - [x] Absolute timestamp conversion for per-message TTL
  - [x] Age-based expiration for per-queue TTL
  - [x] DLX integration with `expired` death reason
  - [x] Extension registry framework with `EnableTTL` flag
  - [x] `TTLManager` interface with Default and NoOp implementations
  - [x] Comprehensive test coverage (14 unit tests, 10 e2e scenarios)
- [x] **Dead Letter Exchange (DLX)** - RabbitMQ-compatible error handling (v0.12.0)
  - [x] Queue-level DLX configuration via `x-dead-letter-exchange`
  - [x] Routing key override via `x-dead-letter-routing-key`
  - [x] Automatic routing of rejected messages (`basic.reject`, `basic.nack` with requeue=false)
  - [x] Comprehensive `x-death` header tracking (reason, queue, time, count)
  - [x] Multiple death cycle support with history preservation
  - [x] CC/BCC header support and expiration clearing
  - [x] `DeadLetterer` interface for pluggable implementations
  - [x] Full RabbitMQ client compatibility
  - [x] Comprehensive test coverage (8+ e2e scenarios)
- [x] **Topic Exchange Pattern Matching** - Full AMQP topic exchange implementation
  - [x] `MatchTopic()` recursive pattern matching algorithm
  - [x] `*` wildcard (matches exactly one word)
  - [x] `#` wildcard (matches zero or more words)
  - [x] Complex pattern support (e.g., `#.error.#`, `*.database.*`)
  - [x] Input validation for malformed routing keys
  - [x] Integration with message routing in `Publish()`
  - [x] Comprehensive test coverage (70+ test cases)
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

- [ ] **Queue length limits**
  - [ ] Max-length configuration
  - [ ] Max-length dead lettering (maxlen reason)
- [ ] **Priority queues**

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

### **Phase 6: Topic Exchange Implementation (COMPLETED)**

**Completed implementations**:

- Topic exchange pattern matching with `*` and `#` wildcards
- Recursive matching algorithm for complex patterns
- Integration with existing routing infrastructure
- Comprehensive test suite (70+ test cases)
- Input validation for malformed keys

**Key files**:

- `internal/core/broker/vhost/exchange.go` - MatchTopic implementation
- `internal/core/broker/vhost/message.go` - Topic routing in Publish()
- `internal/core/broker/vhost/topic_test.go` - Pattern matching tests

### **Phase 7: Dead Letter Exchanges (COMPLETED - v0.12.0)**

**Goal**: Implement RabbitMQ-compatible dead letter exchange functionality ✅

**Completed Tasks**:

1. ✅ Added dead letter exchange configuration to queue properties
2. ✅ Implemented automatic routing of rejected messages
3. ✅ Added death reason headers (x-death) with full tracking
4. ✅ Supported dead letter routing key override
5. ✅ Multiple death cycles with count tracking
6. ✅ Tested dead letter behavior with basic.reject and basic.nack
7. ✅ Property preservation (delivery mode, content type)
8. ✅ Fixed critical AMQP array encoding bugs

**Key files**:

- `internal/core/broker/vhost/dead-letter.go` - DLX implementation
- `internal/core/amqp/encode.go` - Fixed array encoding
- `tests/e2e/dead_letter_test.go` - Comprehensive DLX tests
- `docs/dead-letter-exchange.md` - Full DLX documentation

### **Phase 8: Message TTL and Expiration (COMPLETED - v0.13.0)**

**Goal**: Implement message and queue TTL with dead letter integration ✅

**Completed Tasks**:

1. ✅ Per-message TTL via `Expiration` property (relative → absolute conversion)
2. ✅ Per-queue TTL via `x-message-ttl` argument
3. ✅ TTL expiration checking with precedence rules
4. ✅ Lazy expiration at all retrieval points (delivery, basic.get, requeue, recovery)
5. ✅ Dead letter routing for expired messages with `expired` reason
6. ✅ Extension registry framework with feature flags
7. ✅ Three-layer message type separation (amqp/vhost/persistence)
8. ✅ Comprehensive test coverage (14 unit tests, 10 e2e tests)
9. ✅ Documentation (message-ttl.md)

**Key files**:

- `internal/core/broker/vhost/ttl.go` - TTL manager and CheckExpiration logic
- `internal/core/broker/vhost/message.go` - Expiration conversion and checking
- `internal/core/broker/vhost/ttl_test.go` - 14 unit tests
- `tests/e2e/ttl_test.go` - 10 comprehensive e2e tests
- `docs/message-ttl.md` - Full TTL documentation

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

The highest priority is **Phase 9: Queue Length Limits**. Contributors should focus on:

1. Max-length configuration via `x-max-length` argument
2. Message drop strategy (drop-head vs reject-publish)
3. Max-length dead lettering with `maxlen` reason
4. Integration with existing message routing
5. Testing with RabbitMQ client compatibility

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

**Last Updated**: November 16, 2025  
**Current Focus**: Phase 9 - Queue Length Limits  
**Completed**: All CONNECTION, CHANNEL (including flow control), EXCHANGE (including topic pattern matching), QUEUE, BASIC, TX class methods, Dead Letter Exchanges, and Message TTL  
**Latest Release**: v0.13.0 - Message TTL and Expiration Support  
**Next Milestone**: Queue length limits with max-length dead lettering

For detailed implementation tasks, see GitHub Issues tagged with the respective phase labels.
