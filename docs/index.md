---
title: OtterMQ Project Overview
---

## 🦦 OtterMQ

OtterMQ is a high-performance message broker written in Go, aiming for compatibility with the AMQP 0.9.1 protocol and interoperability with RabbitMQ clients. It includes a broker backend and a modern management UI built with Vue 3 + Quasar.

This site tracks project status and documentation for users and contributors. For full source code and issue tracking, visit the GitHub repository:

- Repository: [https://github.com/ottermq/ottermq](https://github.com/ottermq/ottermq)
- Docker images and compose files are available in the repo

## Quick links

- AMQP 0.9.1 Support Status: [Status Matrix](./amqp-status)
- REST API Swagger: served by the broker at `/docs` when running
- Project README: [https://github.com/ottermq/ottermq#readme](https://github.com/ottermq/ottermq#readme)
- Roadmap: [https://github.com/ottermq/ottermq/blob/main/ROADMAP.md](https://github.com/ottermq/ottermq/blob/main/ROADMAP.md)

## Highlights

- **Complete AMQP 0.9.1 Basic Class** - All message operations implemented
- AMQP-style exchanges and queues (direct, fanout, topic)
- RabbitMQ client compatibility tested with multiple clients
- Quality of Service (QoS) with prefetch limits
- Message acknowledgments, rejections, and recovery
- Pluggable persistence layer (JSON today; Memento WAL planned)
- Built-in management UI (Vue + Quasar)
- Supports **multiple virtual hosts** for tenant isolation
- Low memory footprint with smart buffering and backpressure

### RabbitMQ Extensions

- [**Dead Letter Exchange (DLX)**](./dead-letter-exchange): Route failed/rejected messages for retry or logging
- [**Message TTL and Expiration**](./message-ttl): Time-To-Live for messages with per-message and per-queue TTL support

## Metrics & Observability

OtterMQ includes built-in metrics collection for monitoring broker performance in real-time.

### Available Metrics

**Broker-Level**
- Total publish rate, delivery rate, ack/nack rates (messages/second)
- Active connections, channels, queues, exchanges (counts)
- Total message count, consumer count

**Queue Metrics** (per queue)
- Message rate (enqueue/sec), delivery rate, ack rate
- Current depth (ready + unacked messages)
- Consumer count

**Channel Metrics** (per channel)
- Publish rate, deliver rate, ack rate, unroutable rate
- Channel state (running, idle, flow, closing)
- Unacked message count, prefetch settings

**Exchange Metrics** (per exchange)
- Publish rate, delivery/routing rate
- Message counts

### Access via Management API

All metrics are available via REST endpoints:

```bash
# Get all channels with live metrics
curl http://localhost:3000/api/channels | jq

# Response includes rates:
{
  "channels": [{
    "number": 1,
    "connection_name": "127.0.0.1:54321",
    "state": "running",
    "publish_rate": 125.4,
    "deliver_rate": 120.8,
    "ack_rate": 118.2
  }]
}
```

**Time-series data**: Metrics include 5-minute history by default (configurable), enabling trend analysis and charting in the management UI.

### Configuration

Metrics are **enabled by default** with minimal performance overhead. Configure via environment variables:

```bash
OTTERMQ_ENABLE_METRICS=true              # Enable/disable
OTTERMQ_METRICS_WINDOW_SIZE=5m           # Retention window
OTTERMQ_METRICS_MAX_SAMPLES=60           # History samples
OTTERMQ_METRICS_SAMPLES_INTERVAL=5       # Sample frequency (seconds)
```

See [Configuration Reference](./configuration) for all available settings.

**Future**: Prometheus exporter and Grafana dashboards are planned for advanced monitoring workflows.

## Getting started

- Build and run using `make build-all` then `make run`
- Development: `make run-dev` for the broker and `quasar dev` for the UI in `ottermq_ui/`

See the repository README for detailed steps and configuration options.

## Contributing documentation

Status pages live in this `/docs` folder and are deployed from the `pages` branch via GitHub Pages. When implementing or changing AMQP features (classes/methods), please keep the [Status Matrix](./amqp-status) up to date.
