# AGENT.md

## Purpose

This file is a working guide for future coding sessions in OtterMQ. It focuses on how the broker is structured today, where behavior lives, and which repo conventions matter when adding AMQP features.

## Project Snapshot

- Goal: evolve OtterMQ into an AMQP 0.9.1-compliant broker with RabbitMQ compatibility where practical.
- Current shape: AMQP broker in Go, Fiber-based management API, Vue/Quasar UI, JSON-backed persistence, SQLite-backed auth/users.
- Entry point: `cmd/ottermq/main.go`

## High-Level Architecture

- `cmd/ottermq/main.go`
  - Loads config, initializes logging, prepares the auth DB, creates the broker, and optionally starts the web server.
- `config/config.go`
  - Central config loader from env and `.env`.
- `internal/core/amqp`
  - Protocol/framing/parsing/encoding layer.
  - `framer.go` defines the broker-facing interface used everywhere else.
- `internal/core/broker`
  - Top-level broker orchestration and AMQP method handlers.
  - `broker.go` owns listener startup, connection registration, and dispatch by AMQP class.
  - `connection.go` runs the frame read loop and connection/channel close flows.
  - `basic.go`, `channel.go`, `queue.go`, `exchange.go`, `tx.go` handle AMQP classes.
- `internal/core/broker/vhost`
  - Real broker state and behavior live here.
  - Exchanges, queues, bindings, consumers, unacked deliveries, transactions, TTL, DLX, and queue length limiting are implemented here.
  - If a new feature affects routing, storage, delivery, requeueing, dead lettering, or recovery, start here.
- `internal/core/broker/management`
  - Read/write service layer used by the HTTP API.
- `web`
  - Fiber server and REST handlers.
- `pkg/persistence`
  - Persistence abstraction plus implementations.
  - JSON is the active backend today.
- `internal/persistdb`
  - SQLite auth/user/role/permission storage for the management layer.
- `tests/e2e`
  - End-to-end compatibility tests using `amqp091-go`.

## Core Runtime Flow

1. `main` builds a `broker.Broker`.
2. `Broker.Start()` opens the TCP listener and performs AMQP handshakes through the framer.
3. `handleConnection()` reads frames, reconstructs multi-frame operations, and sends them to `processRequest()`.
4. `processRequest()` dispatches by AMQP class.
5. Most stateful behavior eventually lands in `vhost.VHost`.

## Important Hotspots

- Publishing path:
  - `internal/core/broker/basic.go`
  - `internal/core/broker/vhost/message.go`
- Queue declaration/deletion/purge:
  - `internal/core/broker/queue.go`
  - `internal/core/broker/vhost/queue.go`
- Delivery, consumer lifecycle, QoS, ack/nack/recover:
  - `internal/core/broker/vhost/delivery.go`
  - `internal/core/broker/vhost/consumer.go`
  - `internal/core/broker/vhost/ack.go`
  - `internal/core/broker/vhost/recover.go`
- Exchange/binding logic:
  - `internal/core/broker/vhost/exchange.go`
  - `internal/core/broker/vhost/binding.go`
- Extensions:
  - TTL: `internal/core/broker/vhost/ttl.go`
  - DLX: `internal/core/broker/vhost/dead-letter.go`
  - Queue length limit: `internal/core/broker/vhost/qll.go`
- Management read models:
  - `internal/core/broker/management`

## Persistence Notes

- Broker state persistence uses `pkg/persistence`.
- Current active backend is JSON, created in `broker.NewBroker()`.
- Auth/users are separate and stored in SQLite via `internal/persistdb`.
- `vhost.NewVhost()` loads persisted queue/exchange/message state if persistence is enabled.

## Testing Notes

- Unit tests are spread across `internal/core/amqp`, `internal/core/broker`, and `internal/core/broker/vhost`.
- E2E tests in `tests/e2e` start a broker automatically unless one is already listening on `localhost:5672`.
- In this environment, `go test ./...` needed writable temp/cache dirs:
  - `env GOCACHE=/tmp/go-build-cache GOTMPDIR=/tmp/go-tmp go test ./...`

## Repo Conventions To Preserve

- Keep protocol parsing in `internal/core/amqp`; keep behavior in `internal/core/broker` and especially `vhost`.
- Reuse the framer interface instead of writing frames directly in new code.
- When adding AMQP behavior, update docs that track compliance:
  - `README.md`
  - `ROADMAP.md`
  - `docs/amqp-status.md`
- Prefer extending existing e2e coverage when changing wire-visible broker behavior.

## Current Risks / Things To Recheck Before Big Changes

- Documentation drift exists:
  - `README.md` still says TTL and queue length limits are still in progress, while `ROADMAP.md` and `docs/amqp-status.md` mark TTL complete and QLL enabled.
- `broker.NewBroker()` hardcodes JSON persistence type instead of selecting from config.
- `internal/core/broker/state_recovery.go` appears older than the newer `vhost.NewVhost()` recovery path and should be treated carefully before relying on it.
- Some management stats are placeholders, such as publish/deliver rates.
- There are still TODOs around proper AMQP exceptions, vhost validation during handshake, and exchange-to-exchange bindings.

## Practical Starting Point For New Features

- Protocol surface first: inspect `internal/core/amqp` for the method frames and parser support.
- Broker dispatch second: wire method handling in `internal/core/broker`.
- State model third: implement semantics in `internal/core/broker/vhost`.
- Persistence fourth: update JSON persistence if the feature changes durable metadata or stored messages.
- Tests last but mandatory: add unit tests close to the behavior and e2e tests when client-visible semantics change.
