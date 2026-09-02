---
title: Prometheus & Grafana Monitoring
---

# Prometheus & Grafana Monitoring

OtterMQ exposes a Prometheus-compatible `/metrics` endpoint that feeds into an optional monitoring stack. The stack — Prometheus for scraping and storage, Grafana for dashboards — lives in the `prometheus/` directory and runs independently of the broker, so you can bring it up only when needed.

## Architecture

```
OtterMQ broker
  └── :9090/metrics  ←── Prometheus (prometheus/) scrapes every 5s
                               └── Grafana reads ── pre-built dashboard
```

The Prometheus endpoint is served on its own port (default **9090**, separate from the management API on 3000) and must be explicitly enabled.

## Quick Start

**1. Enable the Prometheus endpoint in the broker:**

```bash
OTTERMQ_ENABLE_PROMETHEUS=true   # required — disabled by default
OTTERMQ_PROMETHEUS_PORT=9090     # default
OTTERMQ_PROMETHEUS_PATH=/metrics # default
```

**2. Start the monitoring stack:**

```bash
cd prometheus
docker-compose up -d
```

- Prometheus UI: http://localhost:9091
- Grafana: http://localhost:3001 (admin / admin)

The OtterMQ dashboard loads automatically — no manual import needed.

## Exposed Metrics

### Broker-wide

| Metric | Type | Description |
|--------|------|-------------|
| `ottermq_messages_published_total` | Counter | Total messages published |
| `ottermq_messages_delivered_total` | Counter | Total messages delivered |
| `ottermq_messages_acked_total` | Counter | Total messages acknowledged |
| `ottermq_messages_nacked_total` | Counter | Total messages not acknowledged |
| `ottermq_messages_publish_rate` | Gauge | Publish rate (msg/s) |
| `ottermq_messages_deliver_rate` | Gauge | Deliver rate (msg/s) |
| `ottermq_message_count` | Gauge | Total ready messages in broker |
| `ottermq_queue_count` | Gauge | Number of queues |
| `ottermq_connection_count` | Gauge | Active connections |
| `ottermq_channel_count` | Gauge | Open channels |
| `ottermq_consumer_count` | Gauge | Active consumers |

### Per-queue (labels: `queue`)

| Metric | Type | Description |
|--------|------|-------------|
| `ottermq_queue_depth` | Gauge | Ready messages in queue |
| `ottermq_queue_unacked` | Gauge | Unacknowledged messages |
| `ottermq_queue_consumers` | Gauge | Consumers on queue |
| `ottermq_queue_message_rate` | Gauge | Enqueue rate (msg/s) |
| `ottermq_queue_delivery_rate` | Gauge | Delivery rate (msg/s) |
| `ottermq_queue_ack_rate` | Gauge | Ack rate (msg/s) |

### Per-exchange (labels: `exchange`, `type`)

| Metric | Type | Description |
|--------|------|-------------|
| `ottermq_exchange_publish_rate` | Gauge | Publish rate to exchange (msg/s) |
| `ottermq_exchange_delivery_rate` | Gauge | Routing rate from exchange (msg/s) |
| `ottermq_exchange_published_total` | Counter | Total messages published to exchange |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OTTERMQ_ENABLE_PROMETHEUS` | `false` | Enable the `/metrics` endpoint |
| `OTTERMQ_PROMETHEUS_PORT` | `9090` | Port for the Prometheus scrape endpoint |
| `OTTERMQ_PROMETHEUS_PATH` | `/metrics` | Path for the scrape endpoint |

## Grafana Dashboard

The pre-built dashboard (`prometheus/grafana/provisioning/dashboards/ottermq.json`) covers:

- Message throughput (publish/deliver rates)
- Per-queue depth and consumer counts
- Broker-wide connection and channel counts
- Ack/nack rates

The dashboard is provisioned automatically on first startup. To reset it after changes, remove the `grafana_data` volume:

```bash
cd prometheus
docker-compose down -v
docker-compose up -d
```

## Prometheus Scrape Config

The included `prometheus/prometheus.yml` targets `host.docker.internal:9090`, which works when the broker runs natively (`make run-dev`) and the monitoring stack runs in Docker. If both run in Docker on the same network, replace the target with the OtterMQ service name.

```yaml
scrape_configs:
  - job_name: 'ottermq'
    scrape_interval: 5s
    metrics_path: '/metrics'
    static_configs:
      - targets: ['host.docker.internal:9090']
```
