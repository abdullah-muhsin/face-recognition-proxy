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
Administrator browser ---TLS/443----------------+-- /app /api /ws
Prometheus -----------loopback-------------------- /metrics
```

Nginx terminates the existing public TLS certificate and connects only to the
gateway's loopback port. The database has no published port. `/metrics` is
denied at Nginx and additionally CIDR-checked by Go.

## Modules

- `internal/pushsdk`: wire protocol, explicit plaintext/encrypted session mode,
  configured PBKDF2 digest, durably checkpointed challenge lifecycle, AES-CBC
  encryption, exact event validation, and action-specific acknowledgement
  generation.
- `internal/accesscontrol`: versioned, declared JSON `AccessControllerEvent`
  projection. It recognizes only the documented event identity and category
  codes; every other raw event remains unclassified.
- `internal/store`: PostgreSQL migrations, terminal state, exact raw device
  event source values, the one-to-one access-event projection, gateway activity
  history, PushSDK session checkpoints, administrator users, and hashed
  administrator sessions.
- `internal/httpapi`: session-authenticated API, static console delivery,
  origin-checked WebSocket, private metrics, and health endpoints.
- `internal/monitor`: in-memory fan-out of activity records that have already
  been committed to PostgreSQL. It never broadcasts raw payloads.
- `web`: Vue 3/Vite JavaScript application compiled into the Go container.

## Explicit non-features

There is no secondary receiver, HTTP forwarding path, event-name remapping,
digest auto-detection, protocol compatibility parser, inferred media type,
database fallback, previous-challenge acceptance, or automatic terminal
reconfiguration. Device events retain the exact source `eventList.data` value;
gateway activity is separate safe operational metadata, not a parsed
compatibility model or a second copy of the device payload.

The access-event projection is not a compatibility parser. It accepts only a
declared `jsonData` `AccessControllerEvent` containing numeric
`majorEventType` and `subEventType` fields. Its raw source continues to be the
forensic record. Missing, malformed, binary, XML, or other vendor event forms
are retained and receive an explicit unclassified projection status. No field
is inferred, no device time is substituted with receipt time, and subtype
labels are returned only when the retained source description is consistent for
that exact category/code.

Adding a new terminal capability is a schema and protocol change: document the
vendor wire form, add a migration and tests, then release it. Do not add a
best-effort parser branch for an observed malformed frame.
