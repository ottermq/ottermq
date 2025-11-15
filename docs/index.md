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

**RabbitMQ Extensions**:

- **Dead Letter Exchange (DLX)** - Error handling and message routing ([DLX Guide](./dead-letter-exchange))

## Getting started

- Build and run using `make build-all` then `make run`
- Development: `make run-dev` for the broker and `quasar dev` for the UI in `ottermq_ui/`

See the repository README for detailed steps and configuration options.

## Contributing documentation

Status pages live in this `/docs` folder and are deployed from the `pages` branch via GitHub Pages. When implementing or changing AMQP features (classes/methods), please keep the [Status Matrix](./amqp-status) up to date.
