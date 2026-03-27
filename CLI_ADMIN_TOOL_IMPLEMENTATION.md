# OtterMQ CLI Admin Tool - Implementation Guide

## 📋 Executive Summary

This document outlines the implementation plan for an **OtterMQ CLI Admin Tool**, inspired by `rabbitmqadmin`.

The primary goal is to create a practical command-line tool that can:

- help validate new broker features during development
- expose the management API in a scriptable form
- evolve into a useful operator/admin tool for real deployments

### What Are We Building?

A new CLI binary, tentatively named `ottermqadmin`, that talks to a running OtterMQ broker through the existing HTTP management API.

This is intentionally **not** a thin wrapper over internal Go services. The CLI should validate and consume the same public API surface used by the web UI.

### Goals

1. ✅ **Developer Utility**: make feature validation faster for Codex and humans
2. ✅ **Operational Utility**: provide a real administration tool for live brokers
3. ✅ **API Validation**: exercise the same REST API surface used by the web UI
4. ✅ **Remote-Friendly**: work against local or remote OtterMQ instances
5. ✅ **Scriptable**: support structured output for automation (`--json`)
6. ✅ **Incremental**: start with core commands and expand over time

### Architecture Decision

**Implementation Strategy**: Cobra-based CLI + shared HTTP client package

```go
cmd/ottermqadmin/          // CLI entrypoint
internal/cli/...           // cobra commands, config, auth helpers
pkg/adminapi/client        // typed HTTP client for OtterMQ management API
```

**Benefits**:

- ✅ Reuses the existing management API instead of bypassing it
- ✅ Tests the real public admin surface
- ✅ Supports local and remote brokers equally well
- ✅ Keeps broker internals decoupled from CLI concerns
- ✅ Gives us a reusable client layer for future tooling

---

## 🎯 Goals & Benefits

### Goals

1. **Public API First**: CLI should consume the existing REST API, not internal broker packages
2. **Cobra Command Tree**: establish a clean, extensible command structure
3. **Authentication**: support login and JWT reuse
4. **Human + Machine Output**: support readable output and JSON mode
5. **Safe Expansion**: add commands incrementally, aligned with existing API coverage

### Benefits

- 🚀 Faster validation of new broker features without relying only on the UI
- 🛠️ Useful operational tooling for queue, exchange, binding, and message management
- 🔍 Better visibility into API gaps before adding more management features
- 🤖 Easier scripted validation in local development and CI
- 🔄 Stronger alignment between UI, REST API, and operator workflows

---

## 📂 File Structure

### Files to Create

```code
cmd/
└── ottermqadmin/
    └── main.go

internal/cli/
├── root.go
├── config.go
├── output.go
├── auth.go
├── overview.go
├── queues.go
├── exchanges.go
├── bindings.go
└── messages.go

pkg/adminapi/
└── client/
    ├── client.go
    ├── auth.go
    ├── overview.go
    ├── queues.go
    ├── exchanges.go
    ├── bindings.go
    └── messages.go
```

### Files to Modify

```code
makefile                                # Build/run/install targets for ottermqadmin
README.md                               # Add CLI usage section once minimally usable
ROADMAP.md                              # Track feature status and priority
CHANGELOG.md                            # Document initial CLI release
```

### Files to Potentially Modify

```code
web/handlers/api/*.go                   # If missing API endpoints block planned CLI commands
internal/core/broker/management/*.go    # If service-level gaps are found during CLI implementation
internal/core/models/*.go               # If request/response DTOs need refinement
```

---

## Core Design Decisions

### 1. CLI talks to HTTP API, not broker internals

This is the most important decision.

The CLI must behave like a real external admin client:

- connect to a running broker
- authenticate through the login endpoint
- call the same REST endpoints used by the UI

This ensures the tool remains useful outside the codebase and doubles as API validation.

### 2. Use Cobra for command structure

`cobra` is a strong fit because it gives us:

- nested commands
- flags and argument parsing
- shell completion support later
- consistent help output
- a clear path for growth

### 3. Add a shared typed HTTP client package

Instead of embedding HTTP logic directly into Cobra commands, we should centralize it in a reusable package.

Responsibilities:

- base URL handling
- login and JWT storage/reuse
- request/response typing
- consistent error handling
- future retries/timeouts if needed

### 4. Optimize for both humans and automation

The CLI should support:

- human-readable output by default
- `--json` for scripting and validation

This is especially valuable for future feature checks performed by Codex.

### 5. Start narrow and useful

We do not need full `rabbitmqadmin` parity in the first version.

Start with the commands that provide immediate value for development and administration.

---

## Initial Command Scope

### Phase 1 - Foundation

- `ottermqadmin login`
- `ottermqadmin overview`
- global config flags such as:
  - `--url`
  - `--username`
  - `--password`
  - `--token`
  - `--json`

### Phase 2 - Read-Only Resource Inspection

- `ottermqadmin queues list`
- `ottermqadmin queues get <vhost> <queue>`
- `ottermqadmin exchanges list`
- `ottermqadmin exchanges get <vhost> <exchange>`
- `ottermqadmin bindings list`
- `ottermqadmin connections list`

### Phase 3 - Core Mutations

- `ottermqadmin queues create`
- `ottermqadmin queues delete`
- `ottermqadmin queues purge`
- `ottermqadmin exchanges create`
- `ottermqadmin exchanges delete`
- `ottermqadmin bindings create`
- `ottermqadmin bindings delete`
- `ottermqadmin publish`
- `ottermqadmin queues get-messages`

### Phase 4 - Nice-to-Have Enhancements

- shell completions
- output formatting improvements
- config file/profile support
- richer filtering
- better scripting ergonomics

---

## Example Command Style

```bash
ottermqadmin login --url http://localhost:3000 --username guest --password guest
ottermqadmin overview
ottermqadmin queues list
ottermqadmin queues create / my-queue --durable
ottermqadmin exchanges create / my-exchange --type topic
ottermqadmin bindings create --vhost / --source my-exchange --destination my-queue --routing-key demo.*
ottermqadmin publish --vhost / --exchange my-exchange --routing-key demo.test --body "hello"
ottermqadmin queues get-messages / my-queue --count 10 --ack-mode ack_requeue_false
```

## Output Strategy

### Default Output

- concise human-readable summaries
- table-like formatting where helpful

### Structured Output

- `--json` returns raw structured data suitable for scripting

Example:

```bash
ottermqadmin queues list --json
```

This is important for future automated validation workflows.

---

## Authentication Strategy

### Initial Approach

- Login through the existing `/api/login` endpoint
- Store JWT in memory for the current process
- Allow explicit `--token` override

### Later Improvements

- persist token to a config file
- support named profiles/environments
- token refresh or re-login helpers if needed

For the first version, keeping auth simple is the right tradeoff.

---

## API Coverage Strategy

The CLI should follow the current management API, but it may expose gaps.

That is expected and useful.

If the CLI design reveals missing or awkward endpoints, we should:

1. fix the API shape first
2. then implement the matching CLI command

This prevents CLI-only hacks and keeps the platform coherent.

---

## API Coverage Audit

This section compares the planned CLI surface against the current HTTP management API.

### Summary

The current API is already strong enough to support a meaningful first version of `ottermqadmin`.

The original HTTP blockers identified during the audit have now been fixed:

- exchange creation route now matches the handler contract
- vhost-filtered binding listing is now actually vhost-aware
- queue message retrieval now returns structured message DTOs

The main remaining API gap for first-class CLI coverage is:

- vhost service methods exist internally but are not exposed over HTTP

### Coverage Matrix

| CLI Area | Current HTTP Coverage | Status | Notes |
|---------|------------------------|--------|------|
| login | `POST /api/login` | ✅ Ready | Good starting point for CLI auth |
| overview | `GET /api/overview`, `GET /api/overview/broker`, `GET /api/overview/charts` | ✅ Ready | Enough for `overview` and future metrics commands |
| queues list/get | `GET /api/queues`, `GET /api/queues/:vhost/:queue` | ✅ Ready | Good coverage |
| queues create/delete/purge | `POST /api/queues/:vhost/:queue`, `POST /api/queues/:vhost/`, `DELETE /api/queues/:vhost/:queue`, `DELETE /api/queues/:vhost/:queue/content` | ✅ Ready | Good for first CLI iteration |
| queue bindings | `GET /api/queues/:vhost/:queue/bindings` | ✅ Ready | Useful for inspection commands |
| queue consumers | `GET /api/queues/:vhost/:queueName/consumers` | ✅ Ready | Nice extra capability |
| exchanges list/get/delete | `GET /api/exchanges`, `GET /api/exchanges/:vhost/:exchange`, `DELETE /api/exchanges/:vhost/:exchange` | ✅ Ready | Good coverage |
| exchange create | `POST /api/exchanges/:vhost/:exchange` | ✅ Ready | Fixed during CLI pre-work |
| exchange source bindings | `GET /api/exchanges/:vhost/:exchange/bindings/source` | ✅ Ready | Good coverage |
| bindings list/create/delete | `GET /api/bindings`, `POST /api/bindings`, `DELETE /api/bindings` | ✅ Ready | Good for first CLI iteration |
| bindings list by vhost | `GET /api/bindings/:vhost` | ✅ Ready | Fixed during CLI pre-work |
| publish message | `POST /api/exchanges/:vhost/:exchange/publish` | ✅ Ready | Best route for CLI publish |
| generic publish | `POST /api/messages` | ⚠️ Ambiguous | Handler expects path params and defaults silently |
| get messages from queue | `POST /api/queues/:vhost/:queue/get` | ✅ Ready | Now returns structured `MessageListResponse` |
| consumers list/by-vhost/by-queue | `GET /api/consumers`, `GET /api/consumers/:vhost`, `GET /api/queues/:vhost/:queueName/consumers` | ✅ Ready | Good optional CLI support |
| channels list/by-vhost/by-connection/get | `GET /api/channels`, `GET /api/channels/:vhost`, `GET /api/connections/:name/channels`, `GET /api/connections/:name/channels/:channel` | ✅ Mostly Ready | Detail endpoint needs a multi-vhost sanity check |
| connections list/get/close | `GET /api/connections`, `GET /api/connections/:name`, `DELETE /api/connections/:name` | ✅ Ready | Good operational CLI fit |
| vhosts list/get | Service exists, no HTTP route exposed | ❌ Missing API | Needed if we want first-class CLI vhost commands |
| admin users list/create | `GET /api/admin/users`, `POST /api/admin/users` | ✅ Available | Out of current CLI v1 scope |

### Concrete Findings

#### 1. Exchange creation route mismatch

The server registers:

- `POST /api/exchanges`

But the handler expects:

- `:vhost`
- `:exchange`

Relevant files:

- [web/server.go](/home/andre/src/tests_and_examples/golang/ottermq/web/server.go#L123)
- [web/handlers/api/exchanges.go](/home/andre/src/tests_and_examples/golang/ottermq/web/handlers/api/exchanges.go#L89)

That means the current `CreateExchange` handler cannot receive the required exchange name from the route as registered today.

**Status**:

- fixed during CLI pre-work

#### 2. Vhost-filtered bindings route

The server exposes:

- `GET /api/bindings/:vhost`

But it is wired to `api.ListBindings`, not a vhost-specific handler.

Relevant file:

- [web/server.go](/home/andre/src/tests_and_examples/golang/ottermq/web/server.go#L140)

**Status**:

- fixed during CLI pre-work

#### 3. Queue message retrieval response shape

At the service layer, `GetMessages` already returns structured `[]models.MessageDTO`.

But the HTTP handler:

- fetches up to `message_count`
- discards all but the first message
- returns only the payload as `models.SuccessResponse`

Relevant files:

- [internal/core/broker/management/message.go](/home/andre/src/tests_and_examples/golang/ottermq/internal/core/broker/management/message.go#L41)
- [web/handlers/api/messages.go](/home/andre/src/tests_and_examples/golang/ottermq/web/handlers/api/messages.go#L71)

**Status**:

- fixed during CLI pre-work
- endpoint now returns structured message DTOs suitable for automation and CLI output

#### 4. VHost service support exists, but HTTP routes do not

The management service already supports:

- `ListVHosts()`
- `GetVHost(name)`

Relevant file:

- [internal/core/broker/management/vhost.go](/home/andre/src/tests_and_examples/golang/ottermq/internal/core/broker/management/vhost.go#L5)

But there are no matching web handlers/routes today.

**Impact**:

- prevents a proper `ottermqadmin vhosts list` / `get`
- not required for CLI v1, but clearly a useful next API addition

#### 5. Generic publish route is ambiguous

The API exposes both:

- `POST /api/exchanges/:vhost/:exchange/publish`
- `POST /api/messages`

The handler behind both expects route params for `vhost` and `exchange`.

Relevant files:

- [web/server.go](/home/andre/src/tests_and_examples/golang/ottermq/web/server.go#L111)
- [web/server.go](/home/andre/src/tests_and_examples/golang/ottermq/web/server.go#L132)
- [web/handlers/api/messages.go](/home/andre/src/tests_and_examples/golang/ottermq/web/handlers/api/messages.go#L26)

This means `/api/messages` behaves implicitly rather than explicitly, likely falling back to default vhost and empty exchange semantics.

**Impact**:

- not a blocker if the CLI uses the exchange-scoped publish route
- should be clarified or removed later to reduce ambiguity

#### 6. Channel detail lookup deserves a multi-vhost sanity check

`GetChannel(connectionName, channelNumber)` currently resolves snapshots using the default vhost `/`.

Relevant file:

- [internal/core/broker/management/channel.go](/home/andre/src/tests_and_examples/golang/ottermq/internal/core/broker/management/channel.go#L71)

**Impact**:

- likely fine today if the broker mostly operates on `/`
- may become incorrect as soon as multi-vhost usage becomes real

This is not a CLI blocker, but it is worth fixing before relying heavily on per-channel inspection.

### Recommended API Work Before CLI Implementation

#### Still strongly recommended soon after

- [ ] Add `GET /vhosts`
- [ ] Add `GET /vhosts/:name`
- [ ] Clarify or remove ambiguous `POST /messages`

#### Can wait

- [ ] Improve channel detail lookup for multi-vhost correctness
- [ ] Expand admin user endpoints if CLI admin-user commands become desirable

### Observability Update

Recent merged work added meaningful observability capabilities that improve the CLI outlook:

- broker startup now initializes a metrics collector
- management overview includes broker snapshot metrics and chart data
- Prometheus export support exists as a separate server
- channel and broker-level rate information is richer than before

This does not change the Cobra + HTTP-client architecture decision, but it makes the following future CLI commands more attractive:

- richer `overview`
- `channels list` / `channels get` with live rates
- future optional metrics-oriented commands

Prometheus should remain out of CLI v1 scope; the immediate CLI should still target the management API first.

### Conclusion

The API is now sufficient for a strong first `ottermqadmin` release.

With the pre-work fixes already completed, we have a solid base for:

- login
- overview
- queues list/get/create/delete/purge
- exchanges list/get/create/delete
- bindings list/create/delete
- publish
- structured queue message retrieval

That means the CLI can start soon without large architectural changes.

---

## Testing Strategy

### Unit Tests

Test the following in isolation:

- command argument validation
- output formatting
- client request construction
- error handling

### Integration Tests

Add CLI-focused tests against a running OtterMQ instance where practical:

- login flow
- queue/exchange/binding lifecycle
- publish/get message flows

### Manual Validation

The CLI itself becomes a manual validation tool for future broker features:

- create broker resources
- publish messages with feature-specific properties
- inspect effects through API responses

---

## Implementation Order

1. Create `pkg/adminapi/client`
2. Create `cmd/ottermqadmin/main.go`
3. Add Cobra root command and shared flags
4. Implement auth/login flow
5. Implement `overview`
6. Implement read-only queue/exchange/binding commands
7. Implement core mutation commands
8. Improve output formatting
9. Update README and changelog after initial usability milestone

---

## Implementation Slices Checklist

This section breaks the work into small, reviewable slices so implementation and commits can stay easy to follow.

### Slice 1 - Foundation

- [ ] Add `cmd/ottermqadmin/main.go`
- [ ] Add `cobra` dependency
- [ ] Create root command
- [ ] Add shared root flags:
  - [ ] `--url`
  - [ ] `--username`
  - [ ] `--password`
  - [ ] `--token`
  - [ ] `--json`
- [ ] Create initial internal CLI package structure
- [ ] Add initial CLI unit test layout

**Commit goal**: CLI starts, shows help, and has a stable command tree.

### Slice 2 - HTTP Client

- [ ] Create `pkg/adminapi/client`
- [ ] Add base URL handling
- [ ] Add bearer token injection
- [ ] Add shared request helpers
- [ ] Implement first typed client methods:
  - [ ] `Login`
  - [ ] `GetOverview`
  - [ ] `ListQueues`
  - [ ] `GetQueue`
  - [ ] `ListExchanges`
  - [ ] `GetExchange`
  - [ ] `ListBindings`
- [ ] Add unit tests for client request construction and response parsing

**Commit goal**: reusable management API client exists and can be tested independently from Cobra.

### Slice 3 - Authentication

- [ ] Add `ottermqadmin login`
- [ ] Support `--token`
- [ ] Support `--username` + `--password`
- [ ] Keep auth process-local for v1
- [ ] Add clear auth error messaging
- [ ] Add unit tests for auth flag resolution and login flow behavior

**Commit goal**: CLI can authenticate cleanly against a running broker.

### Slice 4 - Read-Only Commands

- [ ] Add `overview`
- [ ] Add `queues list`
- [ ] Add `queues get`
- [ ] Add `exchanges list`
- [ ] Add `exchanges get`
- [ ] Add `bindings list`
- [ ] Optionally add `connections list` if it stays small and low-risk
- [ ] Add command-level unit tests for argument validation and happy-path rendering

**Commit goal**: first useful inspection-only CLI for development and debugging.

### Slice 5 - Output Layer

- [ ] Centralize output formatting
- [ ] Add default human-readable output
- [ ] Add `--json` output mode
- [ ] Make list output consistent across commands
- [ ] Avoid duplicating formatting logic in command handlers
- [ ] Add unit tests for JSON and human-readable output formatting

**Commit goal**: commands are pleasant for humans and scriptable for automation.

### Slice 6 - Core Mutations

- [ ] Add `queues create`
- [ ] Add `queues delete`
- [ ] Add `queues purge`
- [ ] Add `exchanges create`
- [ ] Add `exchanges delete`
- [ ] Add `bindings create`
- [ ] Add `bindings delete`
- [ ] Add `publish`
- [ ] Add `queues get-messages`
- [ ] Add unit tests for mutation command validation and request mapping

**Commit goal**: first truly useful admin tool for feature validation and routine broker operations.

### Slice 7 - Operational Introspection

- [ ] Add `connections get`
- [ ] Add `connections close`
- [ ] Add `channels list`
- [ ] Add `channels get`
- [ ] Add `consumers list`
- [ ] Reuse richer broker metrics where useful
- [ ] Add unit tests for operational inspection commands

**Commit goal**: stronger observability-oriented CLI for live broker inspection.

### Slice 8 - Polish

- [ ] Improve help text and examples
- [ ] Add client tests
- [ ] Add command validation tests
- [ ] Update `makefile`
- [ ] Add `makefile` targets for:
  - [ ] building `ottermqadmin`
  - [ ] installing `ottermqadmin`
  - [ ] running focused CLI tests
- [ ] Update `README.md`
- [ ] Add changelog entry

**Commit goal**: project-ready first release of `ottermqadmin`.

### Suggested Commit Sequence

- [ ] `feat(cli): scaffold ottermqadmin root command with cobra`
- [ ] `feat(cli): add shared management API client`
- [ ] `feat(cli): implement login and auth plumbing`
- [ ] `feat(cli): add read-only overview queues exchanges bindings commands`
- [ ] `feat(cli): add json and human-readable output formatting`
- [ ] `feat(cli): add queue exchange binding and publish mutation commands`
- [ ] `feat(cli): add connection channel and consumer inspection commands`
- [ ] `test(cli): add unit tests for client commands and formatting`
- [ ] `docs(cli): document ottermqadmin usage and add build targets`

---

## Progress Checklist

### Foundation

- [ ] Create CLI binary entrypoint
- [ ] Add Cobra dependency
- [ ] Create root command
- [ ] Add shared config flags
- [ ] Add HTTP client package
- [ ] Add CLI unit test scaffolding

### Authentication

- [ ] Implement login command
- [ ] Support JWT bearer auth in client
- [ ] Support explicit token flag

### Read Commands

- [ ] Implement overview command
- [ ] Implement queues list/get
- [ ] Implement exchanges list/get
- [ ] Implement bindings list
- [ ] Implement connections list

### Write Commands

- [ ] Implement queue create/delete/purge
- [ ] Implement exchange create/delete
- [ ] Implement binding create/delete
- [ ] Implement publish
- [ ] Implement queue message retrieval

### UX

- [ ] Add human-friendly output formatting
- [ ] Add JSON output mode
- [ ] Improve error messages
- [ ] Add help examples
- [ ] Add CLI output/command unit tests

### Docs

- [ ] Update roadmap
- [ ] Update README after initial commands are ready
- [ ] Add changelog entry on first release
- [ ] Update `makefile` with CLI build/install/test targets

---

## Risks & Open Questions

### Risks

- The CLI may expose management API gaps or awkward route shapes
- Authentication UX may feel rough until token persistence/profile support exists
- Output formatting can become inconsistent if not centralized early

### Open Questions

- Final binary name: `ottermqadmin`, `otterctl`, or something else?
- Should v1 persist tokens on disk, or keep auth process-local only?
- Should some commands default the vhost to `/` to reduce verbosity?
- Which output format should be the default for list commands?

For now, the implementation should assume:

- binary name: `ottermqadmin`
- no persisted auth state in v1
- default vhost may later become `/`, but explicit is safer during early development
