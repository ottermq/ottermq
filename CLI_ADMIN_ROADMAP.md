# ottermqadmin CLI — Feature Roadmap

This document tracks planned features for the `ottermqadmin` CLI tool, derived from a gap analysis against RabbitMQ's `rabbitmqadmin` and Management HTTP API. Features are grouped by priority and implementation phase.

## Current State (Implemented)

The following command groups are already implemented and tested:

| Command | Subcommands |
|---------|-------------|
| `login` | — |
| `overview` | — |
| `queues` | list, get, create, delete, purge, get-messages |
| `exchanges` | list, get, create, delete |
| `bindings` | list, create, delete |
| `publish` | — |
| `connections` | list, get, close |
| `channels` | list, get |
| `consumers` | list |
| `vhosts` | list, get, create, delete |
| `users` | list, get, create, delete, change-password |
| `permissions` | list, get, grant, revoke |
| `health` | check-alarms, check-local-alarms, check-port-listener, check-virtual-hosts, check-certificate-expiry, check-ready |
| `definitions` | export, import |
| `nodes` | list, get, memory |

---

## Phase 1 — Essential Multi-Tenancy ✅ COMPLETED

### ~~`vhosts` command group~~ ✅ COMPLETED

### ~~`users` command group~~ ✅ COMPLETED

Implemented: `list`, `get`, `create` (`--password`, `--role`), `delete`, `change-password` (interactive prompt).

Not implemented (no API endpoint in OtterMQ): `list-without-permissions`, `bulk-delete`.

### ~~`permissions` command group~~ ✅ COMPLETED

Implemented: `list`, `get`, `grant`, `revoke`.

Note: OtterMQ uses a binary grant/revoke model — the RabbitMQ-style `--configure/--write/--read` regex flags do not apply.

---

## Phase 2 — Operational Safety & Observability ✅ COMPLETED

### ~~`health` command group~~ ✅ COMPLETED

### ~~`definitions` command group~~ ✅ COMPLETED

### ~~`nodes` command group~~ ✅ COMPLETED

---

## Phase 3 — Policy Engine ❌ BLOCKED (broker side)

Policies apply configuration to queues and exchanges dynamically (TTL, overflow behavior, dead-lettering, etc.) without modifying resource definitions directly.

OtterMQ does not yet have a policy engine. These CLI commands cannot be implemented until the HTTP API exists in the broker.

### `policies` command group

| Subcommand | Description |
|-----------|-------------|
| `policies list` | List all policies |
| `policies list --vhost <v>` | List policies for a specific vhost |
| `policies get <vhost> <name>` | Get a specific policy definition |
| `policies set <vhost> <name>` | Create or update a policy |
| `policies delete <vhost> <name>` | Remove a policy |

**Flags for `set`**:
- `--pattern <regex>`: Resource name pattern to match
- `--definition <json>`: Policy definition (e.g. `{"max-length": 1000}`)
- `--apply-to queues|exchanges|all`: What the policy applies to
- `--priority <n>`: Policy priority (higher wins on conflict)

**API endpoints**:
- `GET /api/policies`
- `GET /api/policies/{vhost}`
- `GET /api/policies/{vhost}/{name}`
- `PUT /api/policies/{vhost}/{name}`
- `DELETE /api/policies/{vhost}/{name}`

---

### `parameters` command group

Runtime parameters for configuring components (e.g. federation upstreams, shovel configs, or custom plugin parameters).

| Subcommand | Description |
|-----------|-------------|
| `parameters list` | List all runtime parameters |
| `parameters list --vhost <v>` | List parameters for a vhost |
| `parameters list --component <c>` | List parameters for a component |
| `parameters set <component> <vhost> <name>` | Set a parameter value |
| `parameters delete <component> <vhost> <name>` | Remove a parameter |

**Flags for `set`**: `--value <json>`

**API endpoints**:
- `GET /api/parameters`
- `GET /api/parameters/{component}`
- `GET /api/parameters/{component}/{vhost}`
- `PUT /api/parameters/{component}/{vhost}/{name}`
- `DELETE /api/parameters/{component}/{vhost}/{name}`

---

## Low Priority Additions

Small additions to existing command groups that add operational value.

### Extended connection management

| Subcommand | Description |
|-----------|-------------|
| `connections close-all --vhost <v>` | Close all connections for a vhost |
| `connections close-all --username <u>` | Close all connections for a user |

**API endpoints**:
- `DELETE /api/connections/username/{username}`
- `GET /api/vhosts/{vhost}/connections`

### Extended binding inspection

| Subcommand | Description |
|-----------|-------------|
| `exchanges bindings-source <vhost> <exchange>` | List bindings where this exchange is the source |
| `exchanges bindings-destination <vhost> <exchange>` | List bindings where this exchange is the destination |
| `queues bindings <vhost> <queue>` | List all bindings for a queue |

**API endpoints**:
- `GET /api/exchanges/{vhost}/{name}/bindings/source`
- `GET /api/exchanges/{vhost}/{name}/bindings/destination`
- `GET /api/queues/{vhost}/{name}/bindings`

---

## Deferred / Out of Scope

Features tied to Erlang's runtime, not applicable to ottermq's architecture, or depending on unimplemented plugins.

| Feature | Reason deferred |
|---------|----------------|
| Cluster join/leave/forget | Requires direct node connection, not HTTP API |
| Shovel management | Plugin-specific, no ottermq equivalent yet |
| Federation management | Plugin-specific, no ottermq equivalent yet |
| Feature flags | Erlang runtime feature, not applicable |
| MQTT/STOMP connection inspection | Protocol plugins not yet implemented |
| Stream protocol connections | Stream queues not yet implemented |
| Operator policies | Can be added alongside Phase 3 policies if needed |
