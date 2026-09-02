---
title: Configuration Reference
---

# Configuration Reference

OtterMQ is configured via environment variables. All settings have sensible defaults and can be overridden as needed.

## Core Broker Settings

### Network & Ports

| Variable | Default | Description |
|----------|---------|-------------|
| `OTTERMQ_BROKER_PORT` | `5672` | AMQP protocol port |
| `OTTERMQ_BROKER_HOST` | `""` | Bind address for AMQP listener (empty = all interfaces) |
| `OTTERMQ_WEB_PORT` | `3000` | Management UI and REST API port |

### AMQP Protocol

| Variable | Default | Description |
|----------|---------|-------------|
| `OTTERMQ_HEARTBEAT_INTERVAL` | `60` | Server-side heartbeat interval in seconds (0 = disable) |
| `OTTERMQ_CHANNEL_MAX` | `2048` | Maximum number of channels per connection |
| `OTTERMQ_FRAME_MAX` | `131072` | Maximum frame size in bytes (128 KB) |
| `OTTERMQ_SSL` | `false` | Enable SSL/TLS for AMQP connections |

### Message Processing

| Variable | Default | Description |
|----------|---------|-------------|
| `OTTERMQ_QUEUE_BUFFER_SIZE` | `100000` | Default buffer size for new queues |
| `OTTERMQ_MAX_PRIORITY` | `10` | Maximum priority level for priority queues (1-255) |

### Persistence

| Variable | Default | Description |
|----------|---------|-------------|
| `OTTERMQ_PERSISTENCE_TYPE` | `json` | Persistence backend: `json` (more backends planned) |
| `OTTERMQ_DATA_DIR` | `./data` | Directory for persistent data |

## Web Server & Management UI

| Variable | Default | Description |
|----------|---------|-------------|
| `OTTERMQ_ENABLE_WEB_API` | `true` | Enable REST API for management |
| `OTTERMQ_ENABLE_UI` | `true` | Enable management UI (Vue/Quasar frontend) |
| `OTTERMQ_ENABLE_SWAGGER` | `false` | Enable Swagger API documentation at `/docs` |
| `OTTERMQ_WEB_API_PATH` | `/api` | Base path for REST API endpoints |
| `OTTERMQ_SWAGGER_PATH` | `/docs` | Path for Swagger UI |

## Feature Flags

Enable or disable specific AMQP features:

| Variable | Default | Description |
|----------|---------|-------------|
| `OTTERMQ_ENABLE_DLX` | `true` | Dead Letter Exchange support |
| `OTTERMQ_ENABLE_TTL` | `true` | Message Time-To-Live support |
| `OTTERMQ_ENABLE_QLL` | `true` | Queue Length Limits (max-length) |

## Metrics & Observability

Control the built-in metrics collection system:

| Variable | Default | Description |
|----------|---------|-------------|
| `OTTERMQ_ENABLE_METRICS` | `true` | Enable/disable metrics collection |
| `OTTERMQ_METRICS_WINDOW_SIZE` | `5m` | Time window for rate calculations (duration format: 5m, 10m, 1h) |
| `OTTERMQ_METRICS_MAX_SAMPLES` | `60` | Maximum samples in ring buffer (history depth) |
| `OTTERMQ_METRICS_SAMPLES_INTERVAL` | `5` | Sampling frequency in seconds |

### Metrics Performance Notes

- **Minimal overhead**: Uses atomic operations and lock-free data structures
- **Bounded memory**: Ring buffers prevent unbounded growth
- **Can be disabled**: Set `OTTERMQ_ENABLE_METRICS=false` for extreme performance scenarios
- **Default retention**: 5-minute window with 5-second samples = 60 data points

## Authentication & Security

| Variable | Default | Description |
|----------|---------|-------------|
| `OTTERMQ_USERNAME` | `guest` | Default admin username (first startup only) |
| `OTTERMQ_PASSWORD` | `guest` | Default admin password (first startup only) |
| `OTTERMQ_JWT_SECRET` | Auto-generated | Secret key for JWT token signing (set in production!) |

## Display & Logging

| Variable | Default | Description |
|----------|---------|-------------|
| `OTTERMQ_SHOW_LOGO` | `false` | Display ASCII art logo on startup |

## Database

| Variable | Default | Description |
|----------|---------|-------------|
| `OTTERMQ_DB_PATH` | `./data/ottermq.db` | SQLite database path for users/permissions |

## Environment File

Instead of setting environment variables individually, you can create a `.env` file in the project root:

```bash
# .env example
OTTERMQ_BROKER_PORT=5672
OTTERMQ_WEB_PORT=3000
OTTERMQ_QUEUE_BUFFER_SIZE=100000
OTTERMQ_ENABLE_METRICS=true
OTTERMQ_METRICS_WINDOW_SIZE=10m
```

The broker will automatically load settings from .env on startup.

## Configuration Priority

 1. Built-in defaults
 2. .env file
 3. Environment variables

## Example Configurations

Production (High Throughput)

```bash
OTTERMQ_QUEUE_BUFFER_SIZE=500000
OTTERMQ_ENABLE_METRICS=true
OTTERMQ_METRICS_WINDOW_SIZE=15m
OTTERMQ_MAX_PRIORITY=5
```

Development (Fast Iteration)

```bash
OTTERMQ_QUEUE_BUFFER_SIZE=10000
OTTERMQ_ENABLE_METRICS=true
OTTERMQ_METRICS_SAMPLES_INTERVAL=1
```

Testing (Minimal Overhead)

```bash
OTTERMQ_ENABLE_METRICS=false
OTTERMQ_QUEUE_BUFFER_SIZE=1000
```

## See Also

- **Metrics & Observability** - Understanding collected metrics
- **Dead Letter Exchange** - DLX configuration details
- **Message TTL** - TTL configuration details
