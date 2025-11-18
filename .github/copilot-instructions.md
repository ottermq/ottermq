# OtterMQ AI Agent Instructions

## Project Overview
OtterMQ is a **high-performance message broker** implementing the **AMQP 0.9.1 protocol** in Go, designed for RabbitMQ compatibility. The project consists of a broker backend and a Vue/Quasar management UI.

## Architecture Components

### Core Structure
- **`internal/core/broker/`**: Main broker orchestration and connection handling
- **`internal/core/amqp/`**: AMQP 0.9.1 protocol implementation (framing, handshake, message parsing)
- **`internal/core/broker/vhost/`**: Virtual host management with exchanges, queues, and message routing
- **`web/`**: Fiber-based REST API with Swagger docs for management UI
- **`ottermq_ui/`**: Vue 3 + Quasar SPA frontend

### Key Patterns

#### AMQP State Management
The broker maintains **stateful AMQP connections** through `ChannelState` in `internal/core/amqp/`. Multi-frame messages (method + header + body) are assembled across multiple TCP packets before processing.

```go
// Pattern: Check for complete message state before processing
if currentState.MethodFrame.Content != nil && currentState.HeaderFrame != nil && 
   currentState.BodySize > 0 && currentState.Body != nil {
    // Process complete message
}
```

#### Graceful Shutdown Flow
Uses atomic flags and wait groups for coordinated shutdown:
1. Set `ShuttingDown.Store(true)` to reject new connections
2. `BroadcastConnectionClose()` to notify all clients
3. `ActiveConns.Wait()` for graceful cleanup

#### Persistence Interface
Implements swappable persistence via `pkg/persistence/`. Currently uses JSON files in `data/` directory. Future plans include:
- implementing pluggable backends -- currently only JSON
- **Memento WAL Engine**: Custom append-only transaction log inspired by RabbitMQ's Mnesia
- Event-driven persistence optimized for message broker workloads

## Development Workflows

### Build & Run
```bash
make build           # Builds broker with git version tag
make run            # Build and run broker only
make docs           # Generate Swagger API docs
make test           # Run all tests with verbose output
make lint           # Run golangci-lint for code quality
make install        # Install binary to GOPATH/bin
make clean          # Remove all build artifacts

# UI Integration
make ui-deps        # Install UI dependencies (npm install)
make ui-build       # Build UI and copy to ./ui directory
make build-all      # Build both UI and broker (production ready)
make run-dev        # Run broker only (UI runs separately on :9000)
```

### UI Integration Modes
1. **Development**: Use `make run-dev` for broker, run UI separately with `cd ottermq_ui && quasar dev`
2. **Production**: Use `make build-all` to build UI into `./ui` directory for embedded serving
3. **No symlinks required** - UI files are copied directly via makefile targets

### Configuration
Environment variables override `.env` file settings. Key configs:
- `OTTERMQ_BROKER_PORT=5672` (AMQP)
- `OTTERMQ_WEB_PORT=3000` (Management UI)
- `OTTERMQ_QUEUE_BUFFER_SIZE=100000` (Message buffering)

## Project-Specific Conventions

### Database Setup
On first run, creates SQLite database in `data/ottermq.db` with default admin user (guest/guest). Handles migration through `persistdb.InitDB()`.

### AMQP Frame Processing
Each connection runs its own goroutine calling `processRequest()` which routes by `ClassID` (CONNECTION, CHANNEL, EXCHANGE, QUEUE, BASIC). State is connection-specific and channel-scoped.

### Error Patterns
- Network errors during shutdown are expected (`net.ErrClosed`)
- Incomplete AMQP frames return `nil` to continue receiving
- Protocol violations return errors that close connections

### API Authentication
Uses JWT middleware for all `/api/*` routes except `/api/login`. Tokens include user role information for authorization.

## Integration Points

### Broker-Web Communication
Web server connects to broker via standard AMQP client (`rabbitmq/amqp091-go`) on localhost, enabling real-time management operations through the same protocol as external clients.

### Message Routing
VHost contains `MessageController` interface for exchange-to-queue routing. Default implementation supports direct, fanout, topic exchanges with binding key matching.

### Cross-Component Dependencies
- Web handlers inject broker instance for read operations (listings, stats)
- AMQP operations go through connected client for consistency
- Persistence layer abstracts storage from broker logic

### Persistence Architecture
Currently uses JSON file storage with plans for **Memento WAL Engine**:
- **Current**: JSON snapshots in `data/` directory via `DefaultPersistence`
- **Planned**: Event-sourced WAL similar to RabbitMQ's Mnesia approach
- **Architecture**: Swappable backends via persistence interface
- **Events vs State**: Memento will use append-only event log vs JSON's state snapshots

Focus on **protocol compliance**, **connection lifecycle management**, and **stateful message assembly** when working with AMQP components. UI changes require understanding the REST API contract defined in `web/handlers/api/`.

## Documentation & GitHub Pages

We maintain a GitHub Pages site to track protocol/class/method support, roadmap notes, and user-facing documentation.

- Public URL: https://ottermq.github.io/ottermq/
- Source directory: `/docs`
- Deployment branch: `pages` (Pages is configured to use `/docs` as the site root)

Contributor workflow for Pages updates:
1. When adding or changing AMQP support (new classes/methods, flags, error codes), or altering behavior that affects compatibility, update the relevant docs under `/docs` to reflect the new status.
2. Commit documentation changes alongside code when reasonable, or in a follow-up PR before release.
3. Open a PR targeting the `pages` branch (or cherry-pick docs commits to `pages`) so the site is deployed after merge.
4. After merge, verify the site renders correctly and reflects the change at the public URL.

PR checklist for protocol changes:
- [ ] Updated `/docs` status matrices/tables for affected AMQP classes and methods
- [ ] Added/updated notes on limitations, TODOs, or partial compliance
- [ ] Cross-referenced the related code areas (`internal/core/amqp/*`, `internal/core/broker/*`) where applicable
- [ ] If API or UI behavior changed, ensured Swagger docs (`make docs`) and any UI docs are updated

## CHANGELOG Management

Starting from the time of this documentation update, OtterMQ uses `CHANGELOG.md` to track all notable changes following the [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) format.

**Historical Context:**
- Prior to this resolution (through v0.13.0), release notes were maintained exclusively in [GitHub Release descriptions](https://github.com/ottermq/ottermq/releases)
- The `CHANGELOG.md` file now serves as the single source of truth for change documentation
- Historical releases (v0.13.0 and earlier) have been backfilled into `CHANGELOG.md` from GitHub releases

**Workflow for Changes:**
1. **During Development**: Update the `[Unreleased]` section in `CHANGELOG.md` as features/fixes are implemented
2. **Before Release**: Move unreleased changes to a new version section with the release date
3. **On Release**: 
   - Create GitHub Release with the same notes from CHANGELOG.md
   - Tag the release version
   - Update version links at the bottom of CHANGELOG.md

**CHANGELOG Structure:**
```markdown
## [Unreleased]
### Added
- New features

### Changed
- Changes to existing functionality

### Fixed
- Bug fixes

### Performance
- Performance improvements (when significant)

## [X.Y.Z] - YYYY-MM-DD
### Added
...
```

**PR Checklist for Contributors:**
- [ ] Updated `CHANGELOG.md` under the `[Unreleased]` section
- [ ] Categorized changes appropriately (Added/Changed/Fixed/Performance)
- [ ] Used descriptive bullet points with context (not just "fixed bug")
- [ ] Referenced issue numbers where applicable

When working on features or bug fixes, always update the CHANGELOG.md file to maintain project history and ease release preparation.