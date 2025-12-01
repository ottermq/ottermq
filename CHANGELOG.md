# Changelog

All notable changes to OtterMQ will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**Note:** Prior to establishing this CHANGELOG.md file, release notes were maintained directly in [GitHub Releases](https://github.com/ottermq/ottermq/releases). Going forward, all changes will be tracked here and synchronized with releases.

## [Unreleased]

### Added

- **Management API Refactoring (Phases 1-6 Complete)**: Professional service layer architecture
  - Management service layer (`internal/core/broker/management/`) separating business logic from HTTP handlers
  - `BrokerProvider` interface pattern to avoid circular dependencies
  - **Queue Management**: Complete DTOs with all properties (TTL, DLX, QLL, consumers, unacked count)
    - Operations: List, Get, Create, Delete, Purge with full property support
  - **Exchange Management**: Complete DTOs with properties and metadata
    - Operations: List, Get, Create, Delete with property configuration
  - **Binding Management**: Structured binding operations with DTOs
    - Operations: List, Create, Delete with source/destination/routing key
  - **Consumer Visibility**: API endpoints for active consumer monitoring
    - Operations: List all consumers, List by queue, with detailed consumer info
  - **Channel Monitoring**: Channel information exposure via API
    - Operations: List all channels, List by connection, Get channel details
  - **Message Operations**: Publish and get messages with full AMQP properties
    - Operations: Publish with properties (TTL, priority, headers), Get messages with ack modes
  - **Connection Management**: Connection information and control
    - Operations: List connections, Get connection details, Close connection
  - **VHost Operations**: Virtual host information
    - Operations: List vhosts, Get vhost details with statistics
  - **Statistics & Overview**: Comprehensive broker monitoring
    - Overview endpoint with broker/node/object totals
    - Message statistics aggregation across all queues
    - Connection statistics by state and protocol
    - Broker configuration details
  - Enhanced request models with validation tags (`CreateQueueRequest`, `CreateExchangeRequest`, `PublishMessageRequest`)
  - VHost helper methods for thread-safe statistics (`GetAllQueues()`, `GetConsumerCountsAllQueues()`)
  - **Comprehensive test coverage**: 52 tests covering all management operations (66% coverage)
  - Added GitHub commit activity badge to README
  - **Management API Examples** in README with curl commands for all operations

### Changed

- **API Handlers Refactored**: Complete removal of AMQP client dependency from web layer
  - All handlers now use `management.Service` instead of `amqp091.Channel`
  - Proper separation of concerns: HTTP layer → Service layer → Broker core
  - Thread-safe operations with proper lock management
  - Removed legacy `internal/core/broker/public.go` interface
  - Removed `ManagerApi` interface and AMQP client from `web/server.go`
  - Bindings API returns structured `BindingDTO` instead of raw maps

### Fixed

- **Import Cycle Resolution**: Management service uses interface pattern for broker access
- **Lock Management**: VHost operations handle locking internally, preventing deadlocks
- **Encapsulation**: Removed direct `broker.mu` access from management code
- **Consumer Cleanup**: Fixed bug where `vh.CleanupConnection()` was never called on connection close
- **Channel Reopening**: Fixed channel state not being removed after close, preventing reopening
- **Overview Deadlock**: Fixed `GetObjectTotalsOverview()` deadlock by holding lock throughout operation
- **Nil Pointer Protection**: Added defensive checks in `createChannelInfo()` for nil channel states

### Performance

- Direct broker access eliminates AMQP protocol overhead for management operations
- O(1) queue/exchange lookups via map access
- Efficient statistics gathering with dedicated VHost methods
- No protocol serialization/deserialization for management operations

## [v0.14.0] - 2025-11-18

### Added

- **Management API Refactoring (Phase 1-2)**: Professional service layer architecture
  - Management service layer (`internal/core/broker/management/`) separating business logic from HTTP handlers
  - `BrokerProvider` interface pattern to avoid circular dependencies
  - Complete Queue DTOs with all properties (TTL, DLX, QLL, consumers, unacked count)
  - Complete Exchange DTOs with properties and metadata
  - Enhanced request models with validation tags (`CreateQueueRequest`, `CreateExchangeRequest`)
  - VHost helper methods for thread-safe statistics (`GetAllQueues()`, `GetConsumerCountsAllQueues()`)
  - Queue operations: List, Get, Create, Delete, Purge with full property support
  - Exchange operations: List, Get, Create, Delete with property configuration

### Changed

- **API Handlers Refactored**: Removed AMQP client dependency from management endpoints
  - Queue handlers now use `management.Service` instead of `amqp091.Channel`
  - Exchange handlers refactored for direct broker access
  - Proper separation of concerns: HTTP layer → Service layer → Broker core
  - Thread-safe operations with proper lock management

### Fixed

- **Import Cycle Resolution**: Management service uses interface pattern for broker access
- **Lock Management**: VHost operations handle locking internally, preventing deadlocks
- **Encapsulation**: Removed direct `broker.mu` access from management code

### Performance

- Direct broker access eliminates AMQP protocol overhead for management operations
- O(1) queue/exchange lookups via map access
- Efficient statistics gathering with dedicated VHost methods

## [v0.14.0] - 2025-11-18

### Added

- **Per-consumer unacked message tracking**: Implemented dual-index data structure (UnackedByConsumer + UnackedByTag) for O(1) consumer operations
- **Consumer cancel E2E tests**: Added 6 comprehensive tests for consumer cancel behavior and edge cases
- Automatic requeue of unacked messages when consumer is canceled (AMQP 0-9-1 spec compliance)
- Performance optimization: Consumer cancel now O(1) instead of O(N) for large prefetch scenarios

### Fixed

- **AMQP Spec Compliance**: Consumer cancel now properly requeues all unacked messages with redelivered flag set
- **Race Conditions**: Fixed multiple race conditions in queue delivery loop and consumer cancellation
- **Deadlocks**: Resolved deadlock in message publishing path when enforcing queue length limits
- Test `TestMaxLen_RequeueRespected` re-enabled and passing after implementing consumer cancel auto-requeue feature
- Test `TestHandleBasicNack_Multiple_Boundary_DiscardPersistent` fixed by correcting queue name mismatch

### Changed

- Queue delivery loop now uses WaitGroup for proper synchronization during shutdown
- Lock ordering improved: release vh.mu before queue.Push operations to prevent deadlock
- QoS counting optimized from O(N) to O(1) for per-consumer prefetch enforcement

### Performance

- Consumer cancel with 500 unacked messages: < 1ms (100-1000x improvement)
- QoS per-consumer counting: O(1) lookup instead of O(N) scan
- Memory overhead: Minimal (~8 bytes per unacked message for second index pointer)

## [0.13.0] - 2025-11-16

### Added

- **Message TTL and Expiration**: Full Time-To-Live support for automatic message expiration
  - Per-message TTL via `Expiration` property
  - Per-queue TTL via `x-message-ttl` argument
  - Lazy expiration strategy (checked at retrieval time)
  - DLX integration for expired messages (death reason: `"expired"`)
- Extension registry framework with `EnableTTL` configuration flag
- Three-layer message separation (protocol, domain, persistence)
- TTLManager interface with pluggable implementations

### Fixed

- **Critical**: Deadlock in message retrieval during TTL expiration with DLX
- Timezone handling in message timestamp conversion
- Dead-letter status reporting in `HandleBasicNack`
- Type support in `parseTTLArgument` (int64, int32, int, float64)

### Performance

- O(1) expiration check per message at retrieval
- Zero CPU overhead for idle queues with expired messages
- Optimized lock management to prevent contention

## [0.12.0] - 2025-11-15

### Added

- **Dead Letter Exchange (DLX)**: Full support for automatic message routing on rejection
  - Queue-level DLX configuration via `x-dead-letter-exchange` and `x-dead-letter-routing-key`
  - Rejection-based dead lettering for `basic.reject` and `basic.nack` with requeue=false
  - Comprehensive `x-death` header tracking (reason, time, queue, exchange, count)
  - Multiple death support with history preservation
  - CC/BCC header support and routing key override

### Fixed

- **Critical**: AMQP array encoding bug causing double-wrapping
- Race condition: "send on closed channel" panic during queue auto-delete
- Integer type encoding in AMQP field tables
- Safe queue deletion with closed flag

### Changed

- Introduced `DeadLetterer` interface for pluggable implementations
- Added `NoOpDeadLetterer` for testing and feature flags

## [0.10.0] - 2025-10-28

### Added

- GitHub Pages documentation site
- `Basic.Nack` support for negative acknowledgments
- `Basic.QoS` handling for prefetch limits (per-consumer and global)
- Message recovery handling improvements

## [0.9.0] - 2025-10-18

### Added

- **Message Persistence**: Messages now survive broker restarts with durable storage
- Producer/Consumer Support: Full implementation of `basic.deliver`, `basic.ack`, `basic.reject`, `basic.cancel`
- Queue Deletion API with auto-delete exchange support
- Structured Logging with Zerolog integration
- Environment Configuration via `.env` files
- AMQP reply code constants for better protocol compliance

### Fixed

- Queue capacity limit of 1000 messages
- Fatal error during connection cleanup
- Exchange listing and sorting logic

## [0.8.0] - 2025-10-02

### Added

- Default exchange support
- Vue scaffold and initial Web UI layout
- JWT authentication flow
- API refactoring with improved structure

### Fixed

- Heartbeat hanging issue
- Disconnected clients not being cleaned up properly
- Panic during request processing

### Changed

- Refactored broker loop for better performance
- Moved frame methods to AMQP module for separation of concerns

## [0.7.1] - 2025-09-26

### Added

- **AMQP 0.9.1 Compliance**: Full protocol handshake implementation
  - `connection.start`, `connection.start-ok`, `connection.tune` sequences
  - `connection.close` and graceful shutdown
- Binary-based framing with dedicated framer interface
- Heartbeat negotiation and timeout handling
- `basic.publish`, `basic.get`, `basic.get-empty` support
- `queue.declare`, `queue.bind`, `queue.list` functionality

### Changed

- Major architecture restructuring for protocol compliance
- Consolidated broker and vhost packages
- Moved AMQP protocol logic to top-level `amqp` package
- Centralized domain models under `internal/model`

### Fixed

- Multiple broker bugs and connection issues
- Resource locking during client disconnection
- Heartbeat handling

## Earlier Releases

For releases prior to v0.7.1, please refer to [GitHub Releases](https://github.com/ottermq/ottermq/releases).

---

[Unreleased]: https://github.com/ottermq/ottermq/compare/v0.13.0...HEAD
[0.13.0]: https://github.com/ottermq/ottermq/releases/tag/v0.13.0
[0.12.0]: https://github.com/ottermq/ottermq/releases/tag/v0.12.0
[0.10.0]: https://github.com/ottermq/ottermq/releases/tag/v0.10.0
[0.9.0]: https://github.com/ottermq/ottermq/releases/tag/v0.9.0
[0.8.0]: https://github.com/ottermq/ottermq/releases/tag/v0.8.0
[0.7.1]: https://github.com/ottermq/ottermq/releases/tag/v0.7.1

