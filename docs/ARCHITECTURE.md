# Architecture

## One deployable application

The gateway is a modular monolith. One Go process owns the terminal protocol,
administration API, browser WebSocket feed, static Vue files, and Prometheus
instrumentation. PostgreSQL is its only durable state. This makes a successful
PushSDK acknowledgement equal to one clear outcome: the record exists in the
database, or the terminal received an error and may retry.

```text
Hikvision terminal --TLS/443--> Nginx --loopback--> Go gateway --SQL--> PostgreSQL
                                               |
Operator browser -----TLS/443------------------+-- /app /api /ws
Prometheus -----------loopback-------------------- /metrics
```

Nginx terminates the existing public TLS certificate and connects only to the
gateway's loopback port. The database has no published port. `/metrics` is
denied at Nginx and additionally CIDR-checked by Go.

## Modules

- `internal/pushsdk`: wire protocol, explicit plaintext/encrypted session mode,
  configured PBKDF2 digest, challenge lifecycle, AES-CBC encryption, exact
  event validation, and action-specific acknowledgement generation.
- `internal/store`: PostgreSQL migrations, terminal state, exact raw device
  event source values, operator users, and hashed sessions.
- `internal/httpapi`: session-authenticated API, static console delivery,
  origin-checked WebSocket, private metrics, and health endpoints.
- `internal/monitor`: in-memory fan-out of safe operational metadata. It has
  no disk retention and never broadcasts raw payloads.
- `web`: Vue 3/Vite JavaScript application compiled into the Go container.

## Explicit non-features

There is no secondary receiver, HTTP forwarding path, separate audit-event
ledger, event-name remapping, digest auto-detection, protocol compatibility
parser, inferred media type, database fallback, or automatic terminal
reconfiguration. The durable device-event record is the exact source
`eventList.data` value, not a parsed compatibility model.

Adding a new terminal capability is a schema and protocol change: document the
vendor wire form, add a migration and tests, then release it. Do not add a
best-effort parser branch for an observed malformed frame.
