# PushSDK Gateway

The old Laravel receiver and .NET gateway have been retired from the active
source tree. Their complete pre-rebuild source snapshot is retained locally in
the ignored `backups/` directory. This repository is now a single production
application built with Go, PostgreSQL, Vue 3/Vite (JavaScript), Tailwind CSS,
PrimeVue, WebSockets, and Prometheus.

## Service boundary

| Path | Consumer | Purpose |
| --- | --- | --- |
| `/iot/{pushSdkSerial}/global/0-global/model/service/operate/PUSH/...` | Hikvision terminal | Strict PushSDK protocol endpoint |
| `/app/` | Operators | Vue console |
| `/api/v1/...` | Vue console | Same-origin authenticated administration API |
| `/ws/v1/monitor` | Vue console | Authenticated live operational metadata |
| `/metrics` | Prometheus only | Private metric endpoint |
| `/healthz`, `/readyz` | Infrastructure | Liveness and database readiness |

The HTTPS and secure WebSocket settings on a terminal may both use public port
443. They are separate protocols on the same TLS listener; Nginx routes HTTP
requests by path and upgrades the browser monitoring WebSocket only at `/ws/`.

## Data and security model

PostgreSQL stores registered terminals, attendance records, and hashed operator
sessions. Attendance is deduplicated with the terminal serial plus the vendor
event UUID, so device retries are safe and do not create duplicate records.

The gateway does not contain an HTTP forwarder, Laravel runtime, queue, SQLite
fallback, or an audit-event ledger. A successfully acknowledged attendance
event is already committed to PostgreSQL. If PostgreSQL is unavailable, the
event is rejected so the terminal can retry; it is never silently dropped.

Live monitor messages and JSON logs contain status and protocol metadata only.
They deliberately exclude encrypted protocol bodies, terminal credentials,
biometric images, and raw face-event payloads. Those data are neither persisted
nor broadcast to browsers.

## Strict PushSDK contract

This gateway intentionally accepts only the documented protocol forms:

- The exact `/iot/{pushSdkSerial}/.../PUSH/{action}` route and `POST` method.
- `AuthInfo` has two mutually exclusive documented modes: an empty body creates
  a JSON/plaintext session, while an `application/json` negotiation body that
  offers the configured security version creates an encrypted session. The
  response advertises only that configured version; it never claims support for
  a version the session will reject.
- A plaintext session permits no query parameters and carries JSON request and
  response bodies. An encrypted session requires exactly `security`, `iv`, and
  `random`; encrypted payloads use `application/octet-stream`.
- `Login` validates the configured username and `loginPasswordDigest` PBKDF2
  algorithm (`sha1` or `sha256`). The selected algorithm is terminal
  configuration, not an auto-detected or retry fallback. Subsequent
  actions validate `My-Custom-Auth`, calculated from the previous
  server-issued `My-Custom-Challenge`.
- `Login` and `Logout` use the standard response envelope. `CommandRequest`
  and `CommandResult` use their documented top-level command fields, while an
  `Event` response is the documented top-level per-event result array; they are
  not wrapped in a generic `data` object.
- Events use an exact JSON envelope, base64 data, unique vendor UUIDs, and one
  of `jsonData`, `xmlData`, `boundaryData`, or empty `noData`.
- `boundaryData` must be the documented direct `multipart/form-data` form with
  explicit content types. Serialized HTTP wrappers, headerless parts, and
  guessed media types are rejected.

An active `AccessControllerEvent` must provide its timestamp, employee number,
and verification method. Missing attendance-status fields remain `NULL`; the
gateway does not invent labels or normalize vendor values.

## Local run

1. Copy `config/terminals.json.example` to ignored `config/terminals.json` and
   set the terminal mapping. It contains an environment-variable name, never a
   password. Set `loginPasswordDigest` to the one documented algorithm used by
   that exact device firmware.
2. Copy `deploy/production/gateway.env.example` to ignored `.env`, set
   `POSTGRES_PASSWORD`, use a matching `DATABASE_URL`, and set unique operator
   credentials. Copy `deploy/production/terminal.env.example` to an ignored
   file such as `.local-secrets/terminal.env` and set the terminal password. For
   local HTTP testing explicitly set `COOKIE_SECURE=false`.
3. Start `GATEWAY_ENV_FILE=$PWD/.env TERMINAL_SECRETS_FILE=$PWD/.local-secrets/terminal.env TERMINALS_FILE=$PWD/config/terminals.json docker compose up --build`.
4. Open `http://localhost:18080/app/`.

The Compose port is loopback-only. It does not expose PostgreSQL or Prometheus
to the network. The metrics allow list includes the private Docker bridge range
because a host-loopback request reaches the container through that bridge.

## Frontend development

The operator console uses Tailwind CSS for layout and PrimeVue 4 for accessible
controls, data tables, panels, feedback, and responsive navigation. Its source
is deliberately split into Overview, Attendance, Terminals, and Live Monitor
workspaces; it does not render every operational concern on a single screen.

Vue Router uses history-mode paths beneath `/app/`; the gateway serves the Vue
entry document for those browser routes while continuing to return `404` for
missing static assets. Pinia owns the authenticated gateway state and monitor
connection. Vite auto-imports Vue, Vue Router, PrimeVue Toast APIs, and used
PrimeVue components on demand; `.eslintrc-auto-import.json` is the matching
tracked ESLint globals contract.

For standalone frontend work, run:

```bash
npm --prefix web ci
npm --prefix web run check
npm --prefix web run dev
```

Run `make test` and `make vet` to verify the Go service with the same Go 1.24
toolchain used by the production image. They explicitly test the gateway's Go
source roots so ignored frontend dependencies cannot affect the result.

The Vite server serves the console at `/app/`. For authenticated behaviour,
run it through the gateway or the local Compose stack so its same-origin API
and WebSocket endpoints are available.

## Production release

First create these VPS files, owned by the deployment account:

```text
/home/abdullah/pushsdk-gateway-runtime/gateway.env
/home/abdullah/pushsdk-gateway-runtime/terminal.env
/home/abdullah/pushsdk-gateway-runtime/terminals.json
```

`gateway.env` and `terminal.env` contain credentials and must be mode `0600`.
`terminals.json` contains only terminal identifiers, the gateway username, and
the name of the password environment variable; it must be mode `0644` because
the non-root gateway process reads its read-only bind mount.

`loginPasswordDigest` is required for every terminal mapping. It is exactly
`sha256` for standard firmware or `sha1` only where the vendor documentation
identifies that exact firmware. The gateway performs one configured PBKDF2
check; it never attempts both values. Earlier mappings used the ambiguous key
`digest`; rename that key to `loginPasswordDigest` before releasing this
revision.

Use `deploy/production/gateway.env.example`,
`deploy/production/terminal.env.example`, and
`config/terminals.json.example` as schemas; never copy an example password into
a runtime file. Install `deploy/nginx/pushsdk-gateway.conf` as the complete
`vps.itplus.club` virtual host, validate Nginx, then reload it. It does not
publish `/metrics`, `/healthz`, or `/readyz`.

After committing a clean tree, run:

```bash
scripts/release-production.sh
```

It creates a versioned source release under
`/home/abdullah/pushsdk-gateway-src/`, starts only the
`pushsdk-gateway` Compose project, and checks its loopback readiness endpoint.
It never edits Nginx, certificates, or unrelated services.

See [architecture](docs/ARCHITECTURE.md) and the
[device migration runbook](docs/DEVICE-MIGRATION.md) before changing the live
terminal configuration.
