# OtterMQ Observability & Real-Time Metrics Roadmap

## Overview

This document tracks the implementation plan and current status for observability in OtterMQ: internal metrics for UI charts, Prometheus/Grafana integration, and distributed tracing.

**Key Principle:** Build in layers - start with internal metrics for UI charts, then add external monitoring integrations.

## Implementation Status

| Phase | Description | Status |
|---|---|---|
| Phase 1 | Internal metrics collection | ✅ Done |
| Phase 2 | Prometheus/Grafana integration | ✅ Done |
| Phase 3 | OpenTelemetry & distributed tracing | 🔲 Planned |

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    OtterMQ Broker                           │
│  ┌──────────────────────────────────────────────────────┐  │
│  │           Message Flow Instrumentation                │  │
│  │  • Publish events    • Delivery events                │  │
│  │  • ACK/NACK events   • Connection events              │  │
│  └────────────────┬─────────────────────────────────────┘  │
│                   │                                          │
│  ┌────────────────▼─────────────────────────────────────┐  │
│  │         pkg/metrics/Collector                         │  │
│  │  • Rate tracking (messages/sec, connections/sec)      │  │
│  │  • Gauges (queue depths, consumer counts)             │  │
│  │  • Time-series ring buffers (5-60 min history)        │  │
│  └──────────────┬─────────────────┬─────────────────────┘  │
│                 │                 │                          │
└─────────────────┼─────────────────┼──────────────────────────┘
                  │                 │
        ┌─────────▼────────┐  ┌────▼──────────────┐
        │  Management API  │  │ Prometheus        │
        │  /api/*          │  │ :9090/metrics     │
        └─────────┬────────┘  └────┬──────────────┘
                  │                │
        ┌─────────▼────────┐  ┌────▼──────────────┐
        │  Management UI   │  │  Grafana          │
        │  (Vue/Quasar)    │  │  :3001            │
        └──────────────────┘  └───────────────────┘
```

---

## Phase 1: Internal Metrics Collection ✅ Done

**Dependencies:** None

### What was built

- **`pkg/metrics/`** — core metrics package: `Collector`, `RateTracker` (ring buffer), per-queue and per-exchange metric structs
- **`internal/core/broker/`** — broker instrumented at publish, deliver, ack, nack, connection, channel events
- **`internal/core/broker/vhost/`** — vhost-level hooks feeding queue and exchange metrics
- **Management API** — existing `/api/queues`, `/api/exchanges`, `/api/channels`, `/api/overview` endpoints expose live metrics from the collector

### Key design decisions

- Ring buffers with configurable retention (default: 60 samples × 5s = 5 min history)
- Atomic operations for per-event recording; lock-free where possible
- No external dependencies for Phase 1

### Configuration

```bash
OTTERMQ_ENABLE_METRICS=true          # default: true
OTTERMQ_METRICS_WINDOW_SIZE=5m       # retention window
OTTERMQ_METRICS_MAX_SAMPLES=60       # ring buffer depth
OTTERMQ_METRICS_SAMPLES_INTERVAL=5  # sample frequency (seconds)
```

---

## Phase 2: Prometheus Integration ✅ Done

**Dependencies:** Phase 1 complete

### What was built

- **`web/prometheus/exporter.go`** — `Exporter` struct that reads from `pkg/metrics/Collector` and pushes to Prometheus client_golang on a configurable interval. Tracks deltas for counters to avoid double-counting across scrapes.
- **`web/prometheus/config.go`** — `Config` for update interval and endpoint settings
- **`web/server.go`** — exporter wired in; `/metrics` served via `promhttp.Handler()` on a dedicated port
- **`prometheus/`** — standalone monitoring stack (see below)

### Monitoring stack

The Prometheus + Grafana stack lives in `prometheus/` and runs independently from the broker. Start it with:

```bash
cd prometheus && docker-compose up -d
```

- Prometheus UI: http://localhost:9091
- Grafana: http://localhost:3001 (admin / admin)

The OtterMQ dashboard provisions automatically.

#### `prometheus/prometheus.yml`

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'ottermq'
    scrape_interval: 5s
    metrics_path: '/metrics'
    static_configs:
      # When running the broker natively (make run-dev), use host.docker.internal
      # When running the broker in Docker on the same compose network, use the service name
      - targets: ['host.docker.internal:9090']
```

#### `prometheus/docker-compose.yml`

```yaml
services:
  prometheus:
    image: prom/prometheus:latest
    container_name: ottermq_prometheus
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    ports:
      - "9091:9090"
    restart: unless-stopped
    networks:
      - monitoring

  grafana:
    image: grafana/grafana:latest
    container_name: ottermq_grafana
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
      - GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH=/etc/grafana/provisioning/dashboards/ottermq.json
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/provisioning:/etc/grafana/provisioning
    ports:
      - "3001:3000"
    depends_on:
      - prometheus
    restart: unless-stopped
    networks:
      - monitoring

volumes:
  prometheus_data:
  grafana_data:

networks:
  monitoring:
    driver: bridge
```

### Configuration

```bash
OTTERMQ_ENABLE_PROMETHEUS=false   # default: false (opt-in)
OTTERMQ_PROMETHEUS_PORT=9090      # scrape endpoint port
OTTERMQ_PROMETHEUS_PATH=/metrics  # scrape endpoint path
```

---

## Phase 3: OpenTelemetry & Distributed Tracing 🔭

**Timeline:** 2-3 weeks  
**Priority:** LOW - Advanced observability  
**Dependencies:** Phase 2 complete

### Goals

1. Trace message flow end-to-end
2. Identify latency bottlenecks
3. Visualize cross-node routing (for clustering)
4. Debug complex message flows

### Implementation Overview

```go
// pkg/observability/tracing.go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func TracePublish(ctx context.Context, exchange, routingKey string) (context.Context, trace.Span) {
    tracer := otel.Tracer("ottermq")
    ctx, span := tracer.Start(ctx, "amqp.publish",
        trace.WithAttributes(
            attribute.String("exchange", exchange),
            attribute.String("routing_key", routingKey),
        ),
    )
    return ctx, span
}

// Propagate context through message properties
// Extract trace context on delivery
// Export to Jaeger/Tempo
```

### Phase 3 Deliverables

- [ ] OpenTelemetry SDK integration
- [ ] Span creation for publish/route/deliver
- [ ] Context propagation via message headers
- [ ] Jaeger/Tempo export configuration
- [ ] Distributed tracing documentation

---

## Performance Considerations

### Memory Usage

- Rate tracker (60 samples): ~1KB per metric
- Total for all metrics: ~50MB (5 min retention)
- Circular buffers prevent unbounded growth; configurable via `OTTERMQ_METRICS_MAX_SAMPLES`

### CPU Overhead

- Recording: ~50ns per event (atomic increment)
- Sampling: ~1µs per sample (periodic, not per-message)
- Target: < 1% CPU overhead at 100k msg/sec
- Metrics can be disabled entirely with `OTTERMQ_ENABLE_METRICS=false`

### Prometheus Scrape

- Metric payload: ~10KB per scrape
- Scrape interval: 5s (ottermq job), 15s (global)
- Bandwidth: < 1KB/s

---

## Future Enhancements

### Advanced Metrics
- Per-consumer metrics
- Per-connection metrics
- Message flow tracing
- Error rate tracking

### Advanced Visualizations
- Heatmaps for latency distribution
- Topology graphs
- Flow diagrams
- Custom dashboards

### Alerting
- Queue depth thresholds
- Latency thresholds
- Error rate thresholds
- Connection limits

### Multi-Tenancy
- Per-vhost metrics isolation
- Per-user metrics
- Quota tracking

---

## References

- **Prometheus Best Practices**: https://prometheus.io/docs/practices/naming/
- **Grafana Dashboards**: https://grafana.com/docs/grafana/latest/dashboards/
- **OpenTelemetry Go**: https://opentelemetry.io/docs/instrumentation/go/
- **RabbitMQ Metrics**: https://www.rabbitmq.com/monitoring.html
