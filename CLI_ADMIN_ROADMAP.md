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

---

## Phase 1 — Essential Multi-Tenancy

These are blockers for any real multi-tenant deployment. Without them, the CLI cannot manage who has access to what.

### `vhosts` command group

Virtual hosts are already used as path parameters throughout the CLI, but they cannot be managed yet.

| Subcommand | Description |
|-----------|-------------|
| `vhosts list` | List all virtual hosts with metadata |
| `vhosts get <name>` | Get details of a specific vhost |
| `vhosts create <name>` | Create a new vhost |
| `vhosts delete <name>` | Delete a vhost and all its resources |

**Flags for `create`**: `--description`, `--tags`

**API endpoints**:
- `GET /api/vhosts`
- `GET /api/vhosts/{name}`
- `PUT /api/vhosts/{name}`
- `DELETE /api/vhosts/{name}`

---

### `users` command group

User management is entirely absent. Required for multi-tenant operations and security auditing.

| Subcommand | Description |
|-----------|-------------|
| `users list` | List all users and their tags |
| `users get <name>` | Get details of a specific user |
| `users create <name>` | Create a new user |
| `users delete <name>` | Delete a user |
| `users change-password <name>` | Update a user's password |
| `users list-without-permissions` | List users that have no vhost access (orphaned accounts) |

**Flags for `create`**: `--password`, `--tags` (e.g. `administrator`, `management`, `monitoring`)

**API endpoints**:
- `GET /api/users`
- `GET /api/users/{name}`
- `PUT /api/users/{name}`
- `DELETE /api/users/{name}`
- `POST /api/users/bulk-delete`
- `GET /api/users/without-permissions`

---

### `permissions` command group

Without this, user management is incomplete — you can create users but not grant them access to vhosts.

| Subcommand | Description |
|-----------|-------------|
| `permissions list` | List all permissions across all vhosts |
| `permissions get <vhost> <user>` | Get a user's permissions in a specific vhost |
| `permissions set <vhost> <user>` | Grant or update a user's permissions in a vhost |
| `permissions revoke <vhost> <user>` | Remove a user's access to a vhost |

**Flags for `set`**: `--configure <regex>`, `--write <regex>`, `--read <regex>`

**API endpoints**:
- `GET /api/permissions`
- `GET /api/permissions/{vhost}/{user}`
- `PUT /api/permissions/{vhost}/{user}`
- `DELETE /api/permissions/{vhost}/{user}`

---

## Phase 2 — Operational Safety & Observability

Features that make day-to-day operations safer and more automatable.

### `health` command group

Targeted health probes — useful for monitoring scripts, CI/CD pipelines, and readiness checks. Each subcommand exits `0` on healthy, non-zero on failure, making them shell-friendly.

| Subcommand | Description |
|-----------|-------------|
| `health check-alarms` | Fail if any cluster-wide alarms are active |
| `health check-local-alarms` | Fail if the target node has local alarms |
| `health check-port-listener <port>` | Fail if the given port is not listening |
| `health check-virtual-hosts` | Fail if any vhost is in a failed state |
| `health check-certificate-expiry` | Fail if TLS cert expires within a threshold |
| `health check-ready` | Fail if node is not ready to serve clients |

**Flags for `check-certificate-expiry`**: `--within <n>`, `--unit days|weeks|months`

**API endpoints**:
- `GET /api/health/checks/alarms`
- `GET /api/health/checks/local-alarms`
- `GET /api/health/checks/port-listener/{port}`
- `GET /api/health/checks/virtual-hosts`
- `GET /api/health/checks/certificate-expiration/{within}/{unit}`
- `GET /api/health/checks/ready-to-serve-clients`

---

### `definitions` command group

Export and import the full broker configuration as JSON. Critical for backups, environment cloning, and disaster recovery.

| Subcommand | Description |
|-----------|-------------|
| `definitions export` | Export cluster-wide definitions to stdout or a file |
| `definitions export --vhost <v>` | Export definitions scoped to one vhost |
| `definitions import <file>` | Restore definitions from a JSON file |
| `definitions import --vhost <v> <file>` | Restore vhost-scoped definitions |

**Flags for `export`**: `--output <file>` (default: stdout)

The exported JSON contains: vhosts, users, permissions, exchanges, queues, bindings, and policies.

**API endpoints**:
- `GET /api/definitions`
- `POST /api/definitions`
- `GET /api/definitions/{vhost}`
- `POST /api/definitions/{vhost}`

---

### `nodes` command group

Node-level inspection. Useful for diagnosing memory pressure and verifying cluster membership.

| Subcommand | Description |
|-----------|-------------|
| `nodes list` | List all cluster nodes with status |
| `nodes get <name>` | Get details of a specific node |
| `nodes memory <name>` | Show memory usage breakdown by component |

**API endpoints**:
- `GET /api/nodes`
- `GET /api/nodes/{name}`
- `GET /api/nodes/{name}/memory`

---

## Phase 3 — Policy Engine

Policies apply configuration to queues and exchanges dynamically (TTL, overflow behavior, dead-lettering, etc.) without modifying resource definitions directly.

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
