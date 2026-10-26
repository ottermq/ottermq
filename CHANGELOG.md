# Changelog

All notable changes to OtterMQ will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**Note:** Prior to establishing this CHANGELOG.md file, release notes were maintained directly in [GitHub Releases](https://github.com/ottermq/ottermq/releases). Going forward, all changes will be tracked here and synchronized with releases.

## [Unreleased]

### Upgrade notes

- **The JSON persistence layout changed, and a data dir with durable entities from v0.21 or
  earlier is refused.** Queue, exchange and vhost names are now encoded into file names
  instead of being used as paths (#234). On startup the broker checks for
  `<OTTERMQ_DATA_DIR>/persistence.json`. If it is missing and `vhosts/` isn't empty, the data
  dir is from the old layout and the broker exits with an error rather than misreading it. That
  includes vhosts with no durable entities, whose old directory names can decode as other vhosts.
  To upgrade, back up and remove `<OTTERMQ_DATA_DIR>/vhosts/`. Vhosts, durable queues, exchanges,
  bindings and persistent messages in it are lost; users and permissions (`ottermq.db`) are kept.
  Alternatively, keep running v0.21.x until you can recreate them. A data dir with no `vhosts/`
  directory, or an empty one, upgrades without any action.
- **Web API tokens issued before the upgrade stop working.** `OTTERMQ_JWT_SECRET` no longer
  defaults to `secret`: if it is unset, a random key is generated on first start and kept in
  `<OTTERMQ_DATA_DIR>/jwt.key`, so log in again after upgrading. Back that file up with the data
  dir if tokens should survive a restore (#203).
- **A weak `OTTERMQ_JWT_SECRET` stops the broker at startup.** `secret` (which the old
  `docker.env` and `example.env` set explicitly) or anything shorter than 32 bytes is refused.
  Remove the setting to use a generated key, or set at least 32 random bytes (e.g.
  `openssl rand -hex 32`). `OTTERMQ_INSECURE_DEV=true` accepts a weak key with a warning, for local
  development only (#203).
- **`guest` can only connect from localhost**, over AMQP (`connection.close` 403 ACCESS_REFUSED)
  and on `/api/login` (401), as in RabbitMQ. Clients on other hosts, including the host side of a
  Docker container, need their own user: set `OTTERMQ_USERNAME`/`OTTERMQ_PASSWORD` and restart. The
  broker creates that user as an administrator if it doesn't exist yet, on a new or an existing
  install. `OTTERMQ_LOOPBACK_USERS` lists the restricted users
  (default `guest`); set it empty to restore the old behaviour. `docker-compose.yml` now
  publishes its ports on `127.0.0.1` only (#203).

### Security

- With `LOG_LEVEL=debug` or `trace`, every AMQP login wrote the client's username and password to
  the log in plain text. Neither the `connection.start-ok` response nor its raw payload is logged
  any more.
- Upgraded the web stack to versions without the known vulnerabilities our code reaches: Fiber
  2.52.15 (GO-2026-4543, GO-2025-3845, GO-2025-3706), golang-jwt 5.3.1 (GO-2025-3553), plus
  fasthttp 1.76.0 and golang.org/x/crypto 0.58.0. `govulncheck ./...` now reports none (#207).
- A default install could be taken over by anyone who reached its ports: the bootstrap
  administrator `guest`/`guest` was accepted from any address, and web API tokens were signed with
  the key `secret`, so an admin token could be forged without logging in. The signing key is now
  generated per install, weak configured keys are refused, and `guest` is loopback-only (#203).

### Added

- Consumers are told when their queue is deleted: the broker sends them `basic.cancel`, as
  RabbitMQ does, if their client advertises `consumer_cancel_notify` (amqp091-go does, and turns it
  into `Channel.NotifyCancel`). The broker advertised the capability but never sent it, so these
  consumers waited for messages that would never come (#320).

### Changed

- Building OtterMQ now needs Go 1.26 or later (required by the upgraded dependencies). `go.mod`
  pins the toolchain to Go 1.27.3, which CI uses, and the Docker image builds with
  `golang:1.27-alpine` instead of an unpinned `golang:alpine` (#207).
- Debug and trace logs no longer dump raw AMQP frames, method payloads or message bodies. They
  log the decoded fields instead (channel, queue, exchange, routing key, reply code and text,
  class and method, frame and body sizes) (#328).
- CI now fails when a Go file isn't `gofmt`-formatted, and the whole tree has been formatted once
  (#329).

### Removed

- `Broker.ListVhostDetails`, which nothing called and which would have deadlocked the broker on its
  first call, since it took the broker lock twice (#314).

### Fixed

- `queue.declare-ok` always reported 0 consumers, for a new queue and for a redeclare (passive
  or not) of a queue with consumers. It now reports the queue's active consumer count, as
  RabbitMQ does, so a passive declare shows whether anything consumes from the queue (#335).
- Reject AMQP short strings longer than 255 bytes and propagate encoding errors instead of
  wrapping the length byte and emitting a corrupted frame. Informational channel/connection
  close texts are truncated at a UTF-8 boundary, so long valid queue names still receive a
  channel error without losing the connection (#181).
- `basic.cancel` requeued the consumer's unacked messages, so they were delivered again, and
  acking them afterwards closed the channel with 406 PRECONDITION_FAILED (unknown delivery tag).
  The cancel now only stops new deliveries, as in AMQP and RabbitMQ: messages already delivered
  stay unacked on the channel, can be acked, nacked or rejected, and are requeued on channel
  close, connection close or `basic.recover`. A new consumer reusing the tag doesn't inherit them
  in its prefetch (#299).
- `basic.recover` and `basic.recover-async` from a client failed to parse (the broker expected 9
  bytes of arguments instead of 1) and dropped the connection with 501 FRAME_ERROR.
- Closing a channel, from the client or by the broker after a channel exception, left its
  consumers running and its unacked messages assigned to it. The broker kept delivering to the
  closed channel, and amqp091-go clients dropped the whole connection with 504 CHANNEL_ERROR.
  Unacked `basic.get` deliveries were never requeued, even when the connection closed, so
  non-persistent ones were lost. Closing a channel or a connection now cancels its consumers,
  requeues all its unacked messages in delivery order, marked redelivered, and drops its prefetch,
  flow and transaction state, so a reopened channel number starts clean (#219).
- Unbinding a durable queue from a durable exchange was never saved, so the binding came back after
  a restart and the queue again received messages for the key it was unbound from. Bindings were
  also saved and recovered with their exchange's arguments instead of their own. They are now saved
  with their own arguments, an unbind removes them from disk, and no arguments and an empty
  arguments table name the same binding (#298).
- Other connections could consume from, `basic.get` from and redeclare (passively or not) an
  exclusive queue, which belongs to the connection that declared it. They are now refused with a
  405 RESOURCE_LOCKED channel exception, as in RabbitMQ. `queue.delete`, `queue.purge`,
  `queue.bind` and `queue.unbind` already refused them, but with 403 ACCESS_REFUSED; they answer
  405 too now (#324).
- A refused `basic.consume` (no such queue, a duplicate consumer tag, an exclusive consumer in
  the way) closed the whole connection with 541 INTERNAL_ERROR right after the channel.close, so
  every other channel on it was lost. It now closes only the channel.
- AMQP login rejected passwords and usernames containing spaces, and clients that send a PLAIN
  authorization identity equal to the username: the PLAIN response was split on spaces instead of
  on its NUL separators. It is now parsed as RFC 4616 `authzid NUL authcid NUL passwd`; an
  authorization identity other than the username is still refused (#326).
- Cleaning up a channel's vhost state while that channel ran `tx.commit` deadlocked the vhost, as
  the two took the vhost lock and the transaction lock in opposite orders. No released code path
  cleans up a single channel yet, so this is a latent fix ahead of the channel-close cleanup in #219
  (#311).
- One client that had stopped reading could stall broker shutdown, and with it new connections and
  the management API: the shutdown `connection.close` was written to every client while holding the
  broker lock, with no time limit. The frames are now sent after the lock is released, with a 1s
  write deadline per client, and a client that doesn't take its frame in time is disconnected (#313).
- Deleting a queue that still had consumers left them registered under the queue's name, so a queue
  declared again with that name counted them and delivered its messages to them instead of to its
  own consumers. The consumers of a deleted queue are now removed with it; their unacked messages can
  still be acked, but are no longer requeued into a queue that takes the name. `basic.cancel` for a
  consumer tag the broker doesn't know (such as one of those) gets `cancel-ok`, as in RabbitMQ,
  instead of closing the channel with 404 (#312).
- A consumer tagged `__basic_get__` shared its channel's `basic.get` deliveries: they counted
  against its prefetch, cancelling it requeued them into its own queue, and `basic.recover` requeued
  its messages instead of redelivering them to it. `basic.get` deliveries are now tracked apart
  from consumers, so that tag behaves like any other (#321).
- A crash during the first start, or two brokers starting at once on a fresh data dir, could
  leave an empty `jwt.key`, and the broker then refused to start until the file was removed. The
  generated key is now written to a temp file and linked into place, so `jwt.key` is either
  missing or complete (#203).
- Getting messages through the management API now sizes its result by the number of messages in
  the queue, not by the requested count, so a very large `message_count` no longer fails the
  request.
- A panic in a web API handler now fails only that request with a 500 and is logged with its
  stack trace. Before, it ended the whole process, taking the AMQP broker down with the web API.
  The Prometheus metrics server recovers the same way (#207).
- Publishing while the management API lists channels or connections no longer risks aborting the
  broker with `fatal error: concurrent map read and map write`, which can't be recovered. Every
  broker map is now read and written under its lock, and a connection that disappears while one of
  its requests is in flight is treated as gone instead of panicking (#216).
- Queue, exchange and vhost names can no longer reach outside the data dir. Names such as `..`,
  `../x` or `a/b` used to become file paths: an authenticated client could create or delete
  `.json` files elsewhere, and deleting a vhost named `..` removed the whole data dir, users
  included. Names containing `/` now also persist and recover correctly (#234).

## [v0.21.1]

> **Not production-ready.** The notice for v0.21.0 still applies: stop-ship issues remain open (see
> the [Phase 0 milestone](https://github.com/ottermq/ottermq/milestone/8)). This release fixes two
> memory-exhaustion issues reachable before authentication; upgrade if your AMQP port is reachable
> by clients you don't trust. It also fixes three delivery-loop deadlocks (#214, #215, #218).

### Security

- The AMQP field-table decoder allocated a field's declared length before checking that the frame
  held that many bytes, so a client could make the broker allocate gigabytes without
  authenticating ([GHSA-8gjf-3rj4-m572](https://github.com/ottermq/ottermq/security/advisories/GHSA-8gjf-3rj4-m572)).
- Nested field tables and arrays were copied at every nesting level, so one frame could make the
  broker hold memory growing with the square of its depth, also without authenticating
  ([GHSA-fr4j-xwwq-g84r](https://github.com/ottermq/ottermq/security/advisories/GHSA-fr4j-xwwq-g84r)).

### Fixed

- Field values whose declared length exceeds the frame are now rejected before any memory is
  allocated for them.
- Nested field tables and arrays are decoded in place instead of being copied at every level, so
  decoding memory grows with the size of the frame, not with the square of its nesting depth.
- Deleting a queue no longer hangs the whole vhost. Deletion waited for the queue's delivery
  loop while holding the vhost lock, so it deadlocked when the loop needed that lock, when the
  loop was blocked writing to a consumer that had stopped reading, or after a failed delivery
  had removed the queue's last consumer (the loop then waited for itself, and the queue stopped
  delivering for good). Deletion now stops the loop without waiting for it (#214).
- A queue no longer stops delivering when a consumer subscribes while the queue's last consumer
  is being cancelled, and cancelling a consumer no longer waits for a delivery in flight. Each
  queue now keeps one delivery goroutine for its whole life, which waits for consumers instead
  of exiting when the last one leaves (#218).
- Cancelling a consumer, closing its channel or disconnecting no longer deadlocks the vhost when
  the consumer's unacked messages go back to a queue with `x-max-length` and a dead-letter
  exchange. The messages were requeued while the vhost lock was held, and the requeue
  dead-lettered an evicted message, which takes that lock again. Requeues and publishes to
  queues now happen after the lock is released (#215).

## [v0.21.0]

> **Not production-ready.** OtterMQ is pre-1.0. This release fixes several crashes that a client
> could trigger before authentication, but stop-ship issues remain open (default admin
> credentials, missing web API authorization, path traversal in persistence, deadlocks); see the
> [Phase 0 milestone](https://github.com/ottermq/ottermq/milestone/8) and the
> [production readiness roadmap](https://github.com/ottermq/ottermq/issues/271). Don't expose the
> broker or its web API to untrusted networks.

### Upgrade notes

- `OTTERMQ_FRAME_MAX=0` now means 131072 (it used to mean "no limit"), and values below 4096
  are raised to 4096. Frames larger than the negotiated frame-max close the connection with 501.
- `OTTERMQ_HEARTBEAT_INTERVAL=0` now means "no cap": the client's heartbeat is used. A client
  that answers heartbeat 0 disables heartbeats for its connection.
- Clients that exceed the negotiated limits are now disconnected: `connection.tune-ok` with a
  frame-max out of range, or `channel.open` on channel 0 or above channel-max, gets
  `connection.close` 530.
- Acking an unknown delivery tag now closes the channel with 406, as RabbitMQ does.
- Handshakes must finish within `OTTERMQ_HANDSHAKE_TIMEOUT` (default `10s`).
- Every JSON persistence write is now fsynced, so persistent publishing to durable queues is
  slower on disks with expensive fsync.

### Added

- `OTTERMQ_HANDSHAKE_TIMEOUT` (default `10s`) and the `ottermq_connection_panics_total` metric
  (#194, #195).
- An `edge` Docker image (`ghcr.io/ottermq/ottermq:edge`) is published from the `dev` branch,
  where pull requests are now merged. `latest` is published from `main`, which now changes only
  when a release is made, so `latest` is always the latest release.

### Fixed

- Deleting a queue now removes its bindings from every exchange, not just direct and fanout
  ones. Before, topic bindings (including `amq.topic`) stayed behind and pointed at the deleted
  queue: matching publishes were silently dropped instead of being unroutable, and a persistent
  publish matching a deleted durable queue's pattern closed the publisher's connection with 541.
  Every binding to the queue is removed, its persisted binding state is deleted, and auto-delete
  exchanges left without bindings are deleted (#230).
- A message being delivered when its consumer was cancelled is no longer delivered twice. The
  delivery loop used to push the message back onto the queue when it stopped, even though the
  cancel had already requeued it as unacked.
- A broker restarted on a data dir that holds persistent messages no longer panics while
  recovering the vhost; the messages are back in their durable queues and can be consumed (#233).
- Concurrent persistence writes to one queue or exchange (for example a publish and an ack on
  the same durable queue, or two binds to one exchange) no longer interleave and corrupt its JSON
  file or lose updates. The JSON store now locks each file across every load-modify-save, and the
  `Persistence` interface documents that implementations must be safe for concurrent use (#236).
- JSON persistence writes are now crash-safe. Each write goes to `<name>.json.tmp`, is fsynced,
  then renamed over the live file, and the directory is fsynced, so a crash or power loss leaves
  either the old or the new file, never a truncated one. Stray `.tmp` files are removed at startup.
  Every write is fsynced, so publishing persistent messages to a durable queue is slower on disks
  with expensive fsync (#238).
- AMQP field tables, arrays and strings that declare more bytes than the frame carries are now
  rejected instead of being accepted with the missing bytes zero-padded. An empty string as the
  last field of a table is no longer rejected (#197, #273 — thanks @tnalbant).
- A panic while serving one connection (handshake, frame loop or heartbeat) no longer kills the
  whole broker. The broker logs it with a stack trace, sends `connection.close` 541
  `INTERNAL_ERROR`, closes only that connection and counts it in the new
  `ottermq_connection_panics_total` metric. If cleanup after the panic is blocked by a lock the
  panicking handler left held, the broker exits after 10 s instead of hanging (#194).
- A client that connects and then sends nothing (or stalls mid-handshake) no longer blocks every
  other client from connecting. Handshakes run concurrently and must finish within the new
  `OTTERMQ_HANDSHAKE_TIMEOUT` (default `10s`); the broker closes the socket on timeout, after a
  bad protocol header and on any other handshake failure. Each client is now offered the
  configured heartbeat, frame-max and channel-max instead of the values the previous client sent
  back (#195).
- The broker now closes the socket when it drops a connection: after a heartbeat timeout, a
  frame read or parse error, or any other cleanup. Before, it only forgot the connection, so the
  socket and its reader goroutine stayed open and the client kept publishing into the void. A
  client that sends nothing during its first second after `connection.open-ok` is no longer
  treated as a missed heartbeat (part of #199).
- `basic.ack`, `basic.nack` and `basic.reject` with a delivery tag the channel doesn't have
  outstanding (never delivered, already acked, or a double ack) now close the channel with 406
  `PRECONDITION_FAILED`, as RabbitMQ does. Before, the ack was silently ignored, or, on a channel
  with no deliveries yet, the whole connection was closed with 541. `multiple=true` with
  delivery tag 0 now acknowledges all outstanding deliveries (#226, #281 — thanks
  @kragent66-glitch).
- Frame-max and channel-max negotiation no longer caps a value above 32767 (or 2147483647 for
  32-bit fields) when the other side has no preference. No current code path hit this (#184,
  #280 — thanks @kragent66-glitch).
- A field table with a truncated 64-bit signed integer (`L`) field is now rejected with an error.
  Before, `DecodeTable` returned an empty table and no error (#182, #287 — thanks
  @slopggobblercobbler81).
- Fixed data races between the delivery loop and the connection and channel handlers: a
  connection's last-heartbeat time, a consumer's active flag, and a channel's `basic.qos` and
  flow settings were read without the lock their writers held. The race detector reported
  them, and a delivery could see a half-updated prefetch limit. Recording a heartbeat no longer
  takes the broker-wide lock on every inbound frame (#217).
- A client that answers `connection.tune` with heartbeat 0 ("no heartbeats") or 1 is no
  longer disconnected. The broker's heartbeat goroutine panicked on both values; heartbeat 0 now
  disables heartbeats for that connection, and the broker sends heartbeats every
  `max(1 s, heartbeat/2)` (#177).
- `connection.tune-ok` is now negotiated instead of accepted verbatim. channel-max 0 or above the
  configured `OTTERMQ_CHANNEL_MAX` means the configured value; frame-max 0 means
  `OTTERMQ_FRAME_MAX`, and a frame-max below 4096 or above it closes the connection with 530
  `NOT_ALLOWED`; the heartbeat is capped at `OTTERMQ_HEARTBEAT_INTERVAL` when that is non-zero.
  `channel.open` on channel 0 or above the negotiated channel-max closes the connection with 530.
  Message bodies sent to clients (deliveries, `basic.get-ok`, `basic.return`) are split into body
  frames that fit the negotiated frame-max; before, a body of any size went out as one frame.
  `OTTERMQ_FRAME_MAX=0` now means 131072, and values below 4096 are raised to 4096 (#198).
- A frame whose header declares more than the negotiated frame-max (the configured
  `OTTERMQ_FRAME_MAX` during the handshake) now closes the connection with 501 `FRAME_ERROR`
  before the payload is read. Before, the broker allocated whatever size the header declared, so
  a single 7-byte header could make it allocate up to 4 GiB, even before authentication (#179).

## [v0.20.0]

> **Not production-ready.** OtterMQ is pre-1.0. Known stop-ship issues (pre-authentication
> crashes, default admin credentials, path traversal in persistence, deadlocks) are tracked in the
> [production readiness roadmap](https://github.com/ottermq/ottermq/issues/271). Don't expose
> the broker or its web API to untrusted networks.

### Added

- **New management UI** built with SvelteKit (`web/ui`), replacing the Vue/Quasar UI. It covers
  overview charts, connections, channels, queues, exchanges and administration, including
  creating and deleting queues and exchanges, getting messages, managing bindings, publishing,
  managing users and vhost permissions, a vhost selector, notifications and confirmations
  (#274, #275).
- **Docker images** are published to `ghcr.io/ottermq/ottermq`: `latest` from `main`, and
  `X.Y.Z` / `X.Y` for each release tag. The image now includes the management UI.
- **FreeBSD support:** platform-specific system helpers (file descriptors, memory) and an
  `rc.d` service script in `packaging/freebsd/rc.d`.
- `make install` / `make install-admin` (honouring `PREFIX`/`BINDIR`).
- `OTTERMQ_UI_PATH` sets where the broker serves the UI from (default
  `/usr/local/share/ottermq/ui`).

### Changed

- The UI is built as a static single-page app with `make ui-build` (output copied to `./ui`).
  The web server serves `index.html` for client-side routes, so reloading a page such as
  `/queues` works. The Vue/Quasar UI (`ottermq_ui/`) has been removed.
- The Docker build pins the `swag` CLI to the version in `go.mod`, and the build context excludes
  local data, `.env`, binaries and `node_modules`.

### Fixed

- `queue.unbind` and `DELETE /api/bindings` on a topic exchange returned success without
  removing the binding.
- A `Connection.Close` frame with an oversized reply-text length no longer panics the parser
  (#180, #185 — thanks @SAY-5).
- After an unroutable `mandatory` publish, the next publish on the same channel reused the
  previous message's header and body (#187).
- Declaring an anonymous queue (empty name) hung the client (#188).
- `GET /api/exchanges` reported every exchange as non-durable, non-auto-delete and non-internal.
- Admin permission routes stored URL-encoded vhost names (`%2F` instead of `/`).
- Total RAM reported in node statistics was calculated incorrectly.
- The Docker Image CI workflow failed on every run because it couldn't find the Dockerfile
  (#256, #276); `docker-compose.yml` also pointed at the wrong Dockerfile path.

## [v0.19.3-2]

### Fixed

- **Metrics: negative queue message rate** — `PublishRate` and `DeliveryRate` per-queue rate trackers were being sampled with the current queue depth, which decreases as messages are consumed and produces negative rates. Added `PublishCount` and `DeliveryCount` monotonically increasing counters to `QueueMetrics`; rate trackers now sample these cumulative values (same pattern as `AckCount`).
- **Prometheus: stale queue metrics after deletion** — per-queue `GaugeVec` label sets (`ottermq_queue_depth`, `ottermq_queue_message_rate`, `ottermq_queue_delivery_rate`, `ottermq_queue_ack_rate`, `ottermq_queue_consumers`, `ottermq_queue_unacked`) were never removed when a queue was deleted, causing Grafana to display the last-known value (e.g. a non-zero depth) indefinitely. The exporter now tracks the active queue set each cycle and calls `DeleteLabelValues` for any queue that no longer exists.

## [v0.19.3-1]

### Fixed

- **Critical: queue name corruption via Fiber unsafe string aliasing** — Fiber's default mode uses `utils.UnsafeString` to extract path parameters, returning a zero-copy string backed directly by fasthttp's reused request buffer. Queue names stored from `c.Params("queue")` were silently overwritten when fasthttp recycled that buffer on the next request, producing corrupted names such as `"2F/(AMQ"` (bytes from a subsequent exchange URL). Fixed by enabling `Immutable: true` in the Fiber config, which forces proper copies of all request-derived strings.
- **UI: "Get message" block persists after navigation or queue deletion** — `store.lastMessage` was never cleared when switching to a different queue, navigating away from the Queues page, or deleting the selected queue. The queue store's `select` action now resets `lastMessage` on queue change; `deleteQueue` clears both `selected` and `lastMessage` when the deleted queue was selected; and `QueuesPage` clears `lastMessage` on unmount.
- **UI: no feedback on message publish** — Quasar's `Notify` plugin was not registered in `quasar.config.js` (`plugins: []`), causing all `Notify.create()` calls (success and error toasts) to silently no-op across the entire application. Added `'Notify'` to the plugins list.

## [v0.19.3] - 2026-05-15

### Fixed

- **`queue.delete` AMQP compliance** — `if-unused` and `if-empty` flags are now enforced with `PRECONDITION_FAILED` channel errors; exclusive queue ownership is validated and returns `ACCESS_REFUSED` when a foreign connection attempts deletion
- **Publish deadlock under QLL** — `publishUnlocked` now manually releases `vh.mu` before calling `queue.Push()` so that QLL enforcement (which may dead-letter back through `publishUnlocked`) cannot deadlock; a deferred panic recovery re-acquires the lock before re-panicking to keep the caller's `defer mu.Unlock()` consistent
- **Default exchange alias resolution** — `resolveExchangeAlias` centralises normalisation of `""`, `"amq.default"`, and `"(AMQP default)"` to a single canonical key, fixing routing and binding operations that previously diverged depending on how the default exchange was named
- **Prometheus: stale delta entries on exchange deletion** — `updateExchanges` now prunes `lastExchangePublished` entries for exchanges that no longer exist, preventing unbounded map growth and incorrect counter increments if an exchange is recreated with the same name
- **Management queue statistics** — `fetchQueueStatistics` now prefers the metrics collector snapshot (depth, unacked, consumer count) when available, falling back to direct vhost queries only when metrics are disabled

## [v0.19.2] - 2026-05-11

### Fixed

- **Docker Image CI workflow** — updated to publish container image to GitHub Container Registry (`ghcr.io/ottermq/ottermq`) on merge to `main`; images are tagged with the git version, minor alias, and `latest`

## [v0.19.1] - 2026-04-14

### Added

- **Vhost Pinia store** (`ottermq_ui/src/stores/vhosts.js`) — groundwork for the vhost switcher; fetches available vhosts, preserves selection across refreshes, falls back to `/`

### Fixed

- **Broker panic on dead-letter to deleted queue** — added a `stopped` flag on `Queue` to guard `push()`/`pushPriority()` against sends on a closed channel; added a deferred recovery in `publishUnlocked` to re-acquire the mutex before re-panicking, preventing a secondary "unlock of unlocked mutex" fatal error
- **UnackedCount underflow on nack** — `popUnackedRecords` was calling `RecordQueueAck` for all pops (including nacks), causing a double-decrement; recording is now the caller's responsibility. `RecordQueueNack` gains a `discard bool` parameter to correctly track message removal vs requeue in broker-level counters
- **Exchange page: wrong API endpoints** — `fetchBindings` was calling `/bindings/:vhost/:exchange` instead of `/exchanges/:vhost/:exchange/bindings/source`; `publish` was calling `/messages/:vhost/:exchange` instead of `/exchanges/:vhost/:exchange/publish`
- **Exchange page: binding operations for `(AMQP default)`** — `addBinding`/`deleteBinding` were sending the display alias as the exchange name; `BindQueue`/`UnbindQueue` do a direct map lookup so the alias was never resolved. Now maps `"(AMQP default)"` → `""` client-side
- **Exchange page: silent API failures** — `addBinding`, `deleteBinding`, `del`, `unbind`, `publish`, and `fetchBindings` now surface API errors via a `Notify` toast instead of swallowing them silently
- **Exchange page: binding UI shown for default exchange** — the binding form and list are now hidden when `(AMQP default)` is selected, since all queues are already bound to it automatically

### Refactored

- Replaced remaining `log.Printf("[DEBUG] ...")` calls in `internal/core/amqp/` with structured zerolog calls

---

## [v0.19.0] - 2026-04-14

### Added

- **`nodes` CLI command group** (`nodes list`, `nodes get`, `nodes memory`)
- **Nodes client methods** in `pkg/adminapi/client/nodes.go`
- **Nodes API handlers** (`web/handlers/api/nodes.go`) with `ListNodes`, `GetNode`, `GetNodeMemory`
- **`NodeMemoryDTO`** model with full Go runtime memory breakdown (heap, stack, GC metadata)
- **`definitions` CLI command group** (`definitions export`, `definitions import`)
- **Definitions client methods** in `pkg/adminapi/client/definitions.go`
- **Definitions export/import handlers** with full broker configuration backup/restore
- **`health` CLI command group**: `check-alarms`, `check-local-alarms`, `check-port-listener`, `check-virtual-hosts`, `check-certificate-expiry`, `check-ready`
- **Health check client methods** in `pkg/adminapi/client/health.go`
- **Health check API handlers**
- **`vhosts` CLI command group** (`vhosts list`, `vhosts get`, `vhosts create`, `vhosts delete`)
- **VHost HTTP handlers and routes** — vhost CRUD exposed through management service
- **User management HTTP API**: create, list, get, delete, change-password endpoints
- **Permission management HTTP API**: grant/revoke per-user per-vhost access
- **VHost membership enforcement** in AMQP handshake
- **`user_vhosts` table** and CRUD in `internal/persistdb`
- **Default user granted vhost access** on first broker run
- **`ottermqadmin` CLI tool** (`cmd/ottermqadmin`):
  - Cobra-based binary with shared typed management API client (`pkg/adminapi/client`)
  - Commands: `overview`, `queues`, `exchanges`, `bindings`, `publish`, `connections`, `channels`, `consumers`
  - JSON and human-readable output modes
  - Focused CLI and client unit tests

### Changed

- **Make targets**: added `make build-admin`, `make install-admin`, `make test-cli`; `make build-all` now builds broker, UI, and CLI

### Fixed

- **Bootstrap**: default admin user now seeded when SQLite DB exists but has no users
- **CLI output wiring**: commands now write to process stdout instead of being discarded
- **Default exchange publishing**: `ottermqadmin publish / "" ...` now targets AMQP default exchange correctly
- **Management API**: fixed exchange creation route, vhost-filtered bindings route, message retrieval DTOs
- **JWT secret**: now wired from config into token signing
- **Connection map**: guarded accesses with mutex
- **Priority queue requeue**: messages requeued to priority queue on shutdown
- **Persistent DB**: single connection reused instead of per-request open/close

---

## [v0.18.0] - 2025-12-22

### Added

- **Prometheus metrics exporter**: exposes broker metrics at a configurable `/metrics` endpoint
  - Queue depth, message rates (publish/deliver/ack/nack) per queue
  - Exchange publish and delivery rates per exchange
  - Channel state and activity metrics
  - Broker-level connection, channel, queue, and exchange counts
  - Configurable via `OTTERMQ_ENABLE_PROMETHEUS`, `OTTERMQ_PROMETHEUS_PORT`
- **`prometheus/` package**: dedicated Prometheus integration with `promauto`-based registration

### Changed

- **Broker setup**: metrics collector initialization extracted to `SetupMetricsCollector` method
- **Main function**: improved data directory and user setup logic

### Removed

- **Grafana integration**: removed bundled Grafana configuration and `docker-compose` Grafana service (Prometheus endpoint is the integration point; users bring their own Grafana)

---

## [v0.17.0] - 2025-12-15

### Added

- **Metrics Collection System**: Comprehensive observability infrastructure
  - **Core Package** (`pkg/metrics`): Lock-free, thread-safe metrics collection
    - `Collector`: Central aggregation with `sync.Map` for concurrent access
    - `RateTracker`: Ring buffer-based time-series for rate calculation
    - `MetricsCollector` interface for testability
    - Configurable retention (default: 5min window, 60 samples, 5s intervals)
    - Periodic sampling via background goroutine (`StartPeriodicSampling`)
  
  - **Exchange Metrics**: Per-exchange publish and delivery rate tracking
    - `RecordExchangePublish`, `RecordExchangeDelivery`
    - `GetExchangeSnapshot`, `GetAllExchangeMetrics`
    
  - **Queue Metrics**: Comprehensive per-queue observability ✅ **Fully Integrated**
    - Publish rate, delivery rate, ack rate, nack rate
    - Current depth (ready messages), unacked count, consumer count
    - Requeue tracking for message lifecycle visibility
    - `RecordQueuePublish`, `RecordQueueDelivery`, `RecordQueueAck`, `RecordQueueNack`, `RecordQueueRequeue`
    - `GetQueueSnapshot`, `GetAllQueueMetrics`
    - **Integrated throughout VHost**: delivery loop, ack handling, nack handling, requeue, recovery
    
  - **Broker-Level Metrics**: Global broker statistics
    - Total publish rate, delivery rate (auto-ack + manual-ack), ack rate, nack rate
    - Connection count, channel count, queue count, exchange count
    - Total message count, consumer count, ready depth, unacked depth
    - `RecordConnection`, `RecordConnectionClose`
    - `GetBrokerSnapshot`, `GetBrokerMetrics`
    
  - **Channel Metrics**: Per-channel activity tracking 🆕
    - Publish rate, deliver rate, ack rate, unroutable rate per channel
    - Channel state tracking (running, idle, flow, closing)
    - Unacked count and prefetch count per channel
    - User and VHost association for multi-tenant visibility
    - `RecordChannelPublish`, `RecordChannelDeliver`, `RecordChannelAck`, `RecordChannelUnroutable`
    - `RecordChannelFlow`, `RecordChannelOpen`, `RecordChannelClose`
    - `GetChannelSnapshot`, `GetChannelMetrics`, `GetAllChannelMetrics`
    - **Integrated in broker**: basic.publish, message delivery, ack handling
    
  - **Configuration**:
    - `OTTERMQ_ENABLE_METRICS` - Enable/disable metrics collection (default: true)
    - `OTTERMQ_METRICS_WINDOW_SIZE` - Time window for rate calculation (default: 5m)
    - `OTTERMQ_METRICS_MAX_SAMPLES` - Ring buffer size (default: 60)
    - `OTTERMQ_METRICS_SAMPLES_INTERVAL` - Sampling frequency (default: 5s)
    - Mock collector for testing without metrics overhead
    
  - **Documentation**:
    - [`pkg/metrics/README.md`](pkg/metrics/README.md) - Complete package documentation
    - [`docs/observability-roadmap.md`](docs/observability-roadmap.md) - Multi-phase implementation plan
    - 30+ unit tests with 80%+ coverage

### Changed

- **Management API - Channel Endpoints**: Now return live metrics
  - `GET /api/channels` - Lists channels with publish/deliver/ack rates
  - `GET /api/channels/{vhost}` - Lists vhost channels with metrics
  - `GET /api/connections/{name}/channels` - Lists connection channels with metrics
  - `GET /api/connections/{name}/channels/{id}` - Returns channel details with rates
  - `ChannelDetailDTO` enhanced with: `publish_rate`, `deliver_rate`, `ack_rate`, `unroutable_rate`, `confirm_rate`

- **VHost**: Added `SetMetricsCollector()` for dependency injection
- **Broker**: Initializes metrics collector on startup, starts periodic sampling

### Fixed

- **Channel Metrics Sampling**: Fixed rates showing as "0" by implementing proper periodic sampling
  - Channel counters now sampled every 5 seconds (matching queue/exchange pattern)
  - RateTrackers calculate rates from cumulative atomic counters
  - Removed incorrect direct `Record(1)` calls in channel methods

### Performance

- **Lock-free metric recording**: All `Record*` methods use atomic operations
- **O(1) lookups**: `sync.Map` with composite keys for channel metrics
- **Efficient sampling**: Single background goroutine, 5s interval for all metrics
- **Zero overhead when disabled**: Mock collector replaces real collector if `OTTERMQ_ENABLE_METRICS=false`
- **Fixed memory footprint**: Ring buffers prevent unbounded growth

## [v0.16.0] - 2025-12-04

### Added

- **Priority Queues**: Full AMQP 0.9.1 priority queue implementation
  - Queue-level priority support via `x-max-priority` argument (1-255 range, default limit: 10)
  - Message priority field (0-255) with higher priority delivered first
  - FIFO ordering maintained within same priority level
  - Map-of-channels architecture with lazy allocation
  - Configurable via `OTTERMQ_MAX_PRIORITY` environment variable
  - Zero overhead for non-priority queues (backward compatible)
  - Integration with DLX (priority preserved in dead letters)
  - Integration with TTL (expiration works with priority)
  - Integration with QLL (max-length works with priority)

### Changed

### Fixed

- **E2E Tests**: Fixed `TestPriorityQueue_WithDLX` test logic - consume without auto-ack before manual rejection
- **Delivery Loop**: Fixed priority queue delivery to process all pending messages, not just one per signal

### Performance

- Priority queue delivery: O(P) priority scan where P = max_priority (negligible for P ≤ 10)
- Lazy channel allocation: Memory usage scales with actual priority levels used, not max configured

## [v0.15.0] - 2025-12-01

### Added

- **Management API Refactoring (Phases 1-6 Complete)**: Professional service layer architecture
  - Management service layer (`internal/core/broker/management/`) separating business logic from HTTP handlers
  - `BrokerProvider` interface pattern to avoid circular dependencies
  - **Queue Management**: Complete DTOs with all properties (TTL, DLX, QLL, consumers, unacked count)
    - Operations: List, Get, Create, Delete, Purge with full property support
  - **Exchange Management**: Complete DTOs with properties and metadata
    - Operations: List, Get, Create, Delete with property configuration
  - **Binding Management**: Structured binding operations with DTOs
    - Operations: List, Create, Delete with source/destination/routing key
  - **Consumer Visibility**: API endpoints for active consumer monitoring
    - Operations: List all consumers, List by queue, with detailed consumer info
  - **Channel Monitoring**: Channel information exposure via API
    - Operations: List all channels, List by connection, Get channel details
  - **Message Operations**: Publish and get messages with full AMQP properties
    - Operations: Publish with properties (TTL, priority, headers), Get messages with ack modes
  - **Connection Management**: Connection information and control
    - Operations: List connections, Get connection details, Close connection
  - **VHost Operations**: Virtual host information
    - Operations: List vhosts, Get vhost details with statistics
  - **Statistics & Overview**: Comprehensive broker monitoring
    - Overview endpoint with broker/node/object totals
    - Message statistics aggregation across all queues
    - Connection statistics by state and protocol
    - Broker configuration details
  - Enhanced request models with validation tags (`CreateQueueRequest`, `CreateExchangeRequest`, `PublishMessageRequest`)
  - VHost helper methods for thread-safe statistics (`GetAllQueues()`, `GetConsumerCountsAllQueues()`)
  - **Comprehensive test coverage**: 52 tests covering all management operations (66% coverage)
  - Added GitHub commit activity badge to README
  - **Management API Examples** in README with curl commands for all operations

### Changed

- **API Handlers Refactored**: Complete removal of AMQP client dependency from web layer
  - All handlers now use `management.Service` instead of `amqp091.Channel`
  - Proper separation of concerns: HTTP layer → Service layer → Broker core
  - Thread-safe operations with proper lock management
  - Removed legacy `internal/core/broker/public.go` interface
  - Removed `ManagerApi` interface and AMQP client from `web/server.go`
  - Bindings API returns structured `BindingDTO` instead of raw maps

### Fixed

- **Import Cycle Resolution**: Management service uses interface pattern for broker access
- **Lock Management**: VHost operations handle locking internally, preventing deadlocks
- **Encapsulation**: Removed direct `broker.mu` access from management code
- **Consumer Cleanup**: Fixed bug where `vh.CleanupConnection()` was never called on connection close
- **Channel Reopening**: Fixed channel state not being removed after close, preventing reopening
- **Overview Deadlock**: Fixed `GetObjectTotalsOverview()` deadlock by holding lock throughout operation
- **Nil Pointer Protection**: Added defensive checks in `createChannelInfo()` for nil channel states

### Performance

- Direct broker access eliminates AMQP protocol overhead for management operations
- O(1) queue/exchange lookups via map access
- Efficient statistics gathering with dedicated VHost methods
- No protocol serialization/deserialization for management operations

## [v0.14.0] - 2025-11-18

### Added

- **Management API Refactoring (Phase 1-2)**: Professional service layer architecture
  - Management service layer (`internal/core/broker/management/`) separating business logic from HTTP handlers
  - `BrokerProvider` interface pattern to avoid circular dependencies
  - Complete Queue DTOs with all properties (TTL, DLX, QLL, consumers, unacked count)
  - Complete Exchange DTOs with properties and metadata
  - Enhanced request models with validation tags (`CreateQueueRequest`, `CreateExchangeRequest`)
  - VHost helper methods for thread-safe statistics (`GetAllQueues()`, `GetConsumerCountsAllQueues()`)
  - Queue operations: List, Get, Create, Delete, Purge with full property support
  - Exchange operations: List, Get, Create, Delete with property configuration
- **Per-consumer unacked message tracking**: dual-index data structure (UnackedByConsumer + UnackedByTag) for O(1) consumer operations
- **Consumer cancel E2E tests**: 6 comprehensive tests for consumer cancel behavior and edge cases
- Automatic requeue of unacked messages when consumer is canceled (AMQP 0-9-1 spec compliance)

### Changed

- **API Handlers Refactored**: Removed AMQP client dependency from management endpoints
  - Queue handlers now use `management.Service` instead of `amqp091.Channel`
  - Exchange handlers refactored for direct broker access
  - Proper separation of concerns: HTTP layer → Service layer → Broker core
  - Thread-safe operations with proper lock management
- Queue delivery loop now uses WaitGroup for proper synchronization during shutdown
- Lock ordering improved: release `vh.mu` before queue.Push operations to prevent deadlock
- QoS counting optimized from O(N) to O(1) for per-consumer prefetch enforcement

### Fixed

- **Import Cycle Resolution**: Management service uses interface pattern for broker access
- **Lock Management**: VHost operations handle locking internally, preventing deadlocks
- **Encapsulation**: Removed direct `broker.mu` access from management code
- **AMQP Spec Compliance**: Consumer cancel now properly requeues all unacked messages with redelivered flag set
- **Race Conditions**: Fixed multiple race conditions in queue delivery loop and consumer cancellation
- **Deadlocks**: Resolved deadlock in message publishing path when enforcing queue length limits

### Performance

- Direct broker access eliminates AMQP protocol overhead for management operations
- Consumer cancel with 500 unacked messages: < 1ms (100-1000x improvement)
- QoS per-consumer counting: O(1) lookup instead of O(N) scan

## [v0.13.0] - 2025-11-16

### Added

- **Message TTL and Expiration**: Full Time-To-Live support for automatic message expiration
  - Per-message TTL via `Expiration` property
  - Per-queue TTL via `x-message-ttl` argument
  - Lazy expiration strategy (checked at retrieval time)
  - DLX integration for expired messages (death reason: `"expired"`)
- Extension registry framework with `EnableTTL` configuration flag
- Three-layer message separation (protocol, domain, persistence)
- TTLManager interface with pluggable implementations

### Fixed

- **Critical**: Deadlock in message retrieval during TTL expiration with DLX
- Timezone handling in message timestamp conversion
- Dead-letter status reporting in `HandleBasicNack`
- Type support in `parseTTLArgument` (int64, int32, int, float64)

### Performance

- O(1) expiration check per message at retrieval
- Zero CPU overhead for idle queues with expired messages
- Optimized lock management to prevent contention

## [v0.12.0] - 2025-11-15

### Added

- **Dead Letter Exchange (DLX)**: Full support for automatic message routing on rejection
  - Queue-level DLX configuration via `x-dead-letter-exchange` and `x-dead-letter-routing-key`
  - Rejection-based dead lettering for `basic.reject` and `basic.nack` with requeue=false
  - Comprehensive `x-death` header tracking (reason, time, queue, exchange, count)
  - Multiple death support with history preservation
  - CC/BCC header support and routing key override

### Fixed

- **Critical**: AMQP array encoding bug causing double-wrapping
- Race condition: "send on closed channel" panic during queue auto-delete
- Integer type encoding in AMQP field tables
- Safe queue deletion with closed flag

### Changed

- Introduced `DeadLetterer` interface for pluggable implementations
- Added `NoOpDeadLetterer` for testing and feature flags

## [v0.10.0] - 2025-10-28

### Added

- GitHub Pages documentation site
- `Basic.Nack` support for negative acknowledgments
- `Basic.QoS` handling for prefetch limits (per-consumer and global)
- Message recovery handling improvements

## [v0.9.0] - 2025-10-18

### Added

- **Message Persistence**: Messages now survive broker restarts with durable storage
- Producer/Consumer Support: Full implementation of `basic.deliver`, `basic.ack`, `basic.reject`, `basic.cancel`
- Queue Deletion API with auto-delete exchange support
- Structured Logging with Zerolog integration
- Environment Configuration via `.env` files
- AMQP reply code constants for better protocol compliance

### Fixed

- Queue capacity limit of 1000 messages
- Fatal error during connection cleanup
- Exchange listing and sorting logic

## [v0.8.0] - 2025-10-02

### Added

- Default exchange support
- Vue scaffold and initial Web UI layout
- JWT authentication flow
- API refactoring with improved structure

### Fixed

- Heartbeat hanging issue
- Disconnected clients not being cleaned up properly
- Panic during request processing

### Changed

- Refactored broker loop for better performance
- Moved frame methods to AMQP module for separation of concerns

## [0.7.1] - 2025-09-26

### Added

- **AMQP 0.9.1 Compliance**: Full protocol handshake implementation
  - `connection.start`, `connection.start-ok`, `connection.tune` sequences
  - `connection.close` and graceful shutdown
- Binary-based framing with dedicated framer interface
- Heartbeat negotiation and timeout handling
- `basic.publish`, `basic.get`, `basic.get-empty` support
- `queue.declare`, `queue.bind`, `queue.list` functionality

### Changed

- Major architecture restructuring for protocol compliance
- Consolidated broker and vhost packages
- Moved AMQP protocol logic to top-level `amqp` package
- Centralized domain models under `internal/model`

### Fixed

- Multiple broker bugs and connection issues
- Resource locking during client disconnection
- Heartbeat handling

## Earlier Releases

For releases prior to v0.7.1, please refer to [GitHub Releases](https://github.com/ottermq/ottermq/releases).

---

[Unreleased]: https://github.com/ottermq/ottermq/compare/v0.21.1...HEAD
[v0.21.1]: https://github.com/ottermq/ottermq/releases/tag/v0.21.1
[v0.21.0]: https://github.com/ottermq/ottermq/releases/tag/v0.21.0
[v0.20.0]: https://github.com/ottermq/ottermq/releases/tag/v0.20.0
[v0.19.3]: https://github.com/ottermq/ottermq/releases/tag/v0.19.3
[v0.19.2]: https://github.com/ottermq/ottermq/releases/tag/v0.19.2
[v0.19.1]: https://github.com/ottermq/ottermq/releases/tag/v0.19.1
[v0.19.0]: https://github.com/ottermq/ottermq/releases/tag/v0.19.0
[v0.18.0]: https://github.com/ottermq/ottermq/releases/tag/v0.18.0
[v0.17.0]: https://github.com/ottermq/ottermq/releases/tag/v0.17.0
[v0.16.0]: https://github.com/ottermq/ottermq/releases/tag/v0.16.0
[v0.15.0]: https://github.com/ottermq/ottermq/releases/tag/v0.15.0
[v0.14.0]: https://github.com/ottermq/ottermq/releases/tag/v0.14.0
[v0.13.0]: https://github.com/ottermq/ottermq/releases/tag/v0.13.0
[v0.12.0]: https://github.com/ottermq/ottermq/releases/tag/v0.12.0
[v0.10.0]: https://github.com/ottermq/ottermq/releases/tag/v0.10.0
[v0.9.0]: https://github.com/ottermq/ottermq/releases/tag/v0.9.0
[v0.8.0]: https://github.com/ottermq/ottermq/releases/tag/v0.8.0
[v0.7.1]: https://github.com/ottermq/ottermq/releases/tag/v0.7.1
