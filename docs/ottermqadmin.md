# ottermqadmin CLI

`ottermqadmin` is the OtterMQ command-line administration tool. It connects to a running broker through the management HTTP API — the same surface used by the web UI — making it suitable for scripting, automation, and day-to-day operator tasks.

## Installation

```bash
# Build and install to GOPATH/bin
make install-admin

# Or build the binary only
make build-admin   # output: bin/ottermqadmin
```

## Global Flags

All commands accept the following flags:

| Flag | Default | Description |
|---|---|---|
| `--url` | `http://localhost:3000` | Management API base URL |
| `--username` | — | Username for login-based auth |
| `--password` | — | Password for login-based auth |
| `--token` | — | JWT token (skips login) |
| `--json` | `false` | Output results as JSON |

## Authentication

```bash
# Login with username and password
ottermqadmin login --username guest --password guest

# Use an explicit token
ottermqadmin overview --token <jwt>

# Connect to a remote broker
ottermqadmin --url http://broker.example.com:3000 --username admin --password secret overview
```

Auth is process-local — the token is not persisted to disk between invocations. Pass `--username` and `--password` (or `--token`) with each command, or use `login` to verify connectivity.

## Commands

### `overview`

```bash
ottermqadmin overview
```

### `queues`

```bash
ottermqadmin queues list
ottermqadmin queues get <vhost> <queue>
ottermqadmin queues create <vhost> [queue] [flags]
ottermqadmin queues delete <vhost> <queue> [flags]
ottermqadmin queues purge <vhost> <queue>
ottermqadmin queues get-messages <vhost> <queue> [flags]
```

**`queues create` flags:**

| Flag | Description |
|---|---|
| `--durable` | Survive broker restarts |
| `--auto-delete` | Delete when no consumers remain |
| `--passive` | Validate existence without creating |
| `--max-length <n>` | Set `x-max-length` |
| `--message-ttl <ms>` | Set `x-message-ttl` in milliseconds |
| `--dead-letter-exchange <name>` | Set `x-dead-letter-exchange` |
| `--dead-letter-routing-key <key>` | Set `x-dead-letter-routing-key` |

**`queues delete` flags:** `--if-unused`, `--if-empty`

**`queues get-messages` flags:** `--count <n>` (default 1), `--ack-mode <mode>` (`ack`, `no_ack`, `reject`, `reject_requeue`)

### `exchanges`

```bash
ottermqadmin exchanges list
ottermqadmin exchanges get <vhost> <exchange>
ottermqadmin exchanges create <vhost> <exchange> [flags]
ottermqadmin exchanges delete <vhost> <exchange>
```

**`exchanges create` flags:** `--type <direct|fanout|topic|headers>`, `--durable`, `--auto-delete`, `--passive`

### `bindings`

```bash
ottermqadmin bindings list [--vhost <vhost>]
ottermqadmin bindings create --vhost <vhost> --source <exchange> --destination <queue> [--routing-key <key>]
ottermqadmin bindings delete --vhost <vhost> --source <exchange> --destination <queue> [--routing-key <key>]
```

### `publish`

```bash
ottermqadmin publish <vhost> <exchange> --routing-key <key> --body <message> [--content-type <type>]
```

### `connections`

```bash
ottermqadmin connections list
ottermqadmin connections get <name>
ottermqadmin connections close <name>
```

### `channels`

```bash
ottermqadmin channels list
ottermqadmin channels get <connection> <channel>
```

### `consumers`

```bash
ottermqadmin consumers list
```

### `vhosts`

```bash
ottermqadmin vhosts list
ottermqadmin vhosts get <vhost>
ottermqadmin vhosts create <vhost>
ottermqadmin vhosts delete <vhost>
```

### `users`

```bash
ottermqadmin users list
ottermqadmin users get <username>
ottermqadmin users create <username> --password <pass> [--role <admin|user|guest>]
ottermqadmin users delete <username>
ottermqadmin users change-password <username>   # prompts interactively
```

### `permissions`

```bash
ottermqadmin permissions list
ottermqadmin permissions get <vhost> <username>
ottermqadmin permissions grant <vhost> <username>
ottermqadmin permissions revoke <vhost> <username>
```

### `health`

```bash
ottermqadmin health check-alarms
ottermqadmin health check-local-alarms
ottermqadmin health check-port-listener <port>
ottermqadmin health check-virtual-hosts
ottermqadmin health check-certificate-expiry
ottermqadmin health check-ready
```

### `definitions`

```bash
ottermqadmin definitions export
ottermqadmin definitions import <file>
```

### `nodes`

```bash
ottermqadmin nodes list
ottermqadmin nodes get <name>
ottermqadmin nodes memory <name>
```

## JSON Output

Any command can return raw JSON by adding `--json`:

```bash
ottermqadmin queues list --json
ottermqadmin queues get-messages / my-queue --count 5 --json
```

## Examples

```bash
# Create a durable queue with DLX
ottermqadmin queues create / orders \
  --durable \
  --dead-letter-exchange dlx \
  --dead-letter-routing-key failed

# Bind it to an exchange
ottermqadmin bindings create \
  --vhost / \
  --source amq.direct \
  --destination orders \
  --routing-key new-order

# Publish a test message
ottermqadmin publish / amq.direct \
  --routing-key new-order \
  --body '{"id": 1}' \
  --content-type application/json

# Inspect dead letters
ottermqadmin queues get-messages / failed --count 10 --ack-mode no_ack

# Export broker definitions
ottermqadmin definitions export > backup.json
```
