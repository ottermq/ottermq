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

## Progress Checklist

### Foundation

- [ ] Create CLI binary entrypoint
- [ ] Add Cobra dependency
- [ ] Create root command
- [ ] Add shared config flags
- [ ] Add HTTP client package

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

### Docs

- [ ] Update roadmap
- [ ] Update README after initial commands are ready
- [ ] Add changelog entry on first release

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

