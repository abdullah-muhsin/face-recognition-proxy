# PushSDK Gateway

The old Laravel receiver and .NET gateway have been retired from the active
source tree. Their complete pre-rebuild source snapshot is retained locally in
the ignored `backups/` directory. This repository is now a single production
application built with Go, PostgreSQL, Vue 3/Vite (JavaScript), WebSockets, and
Prometheus.

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

- The exact `/iot/{pushSdkSerial}/.../PUSH/{action}` route and `POST` with
  `Content-Type: application/json`.
- `AuthInfo` announces the terminal's configured security version (3 or 4).
- `Login` and every authenticated action use negotiated AES-CBC encryption
  with exactly `security`, `iv`, and `random` query parameters.
- Each authenticated request must present the server-issued challenge header.
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
   password.
2. Copy `.env.example` to ignored `.env`, set `POSTGRES_PASSWORD`, use a
   matching `DATABASE_URL`, and set unique operator credentials. Put
   `PUSHSDK_TERMINAL_PASSWORD` in a separate ignored terminal environment file.
   For local HTTP testing use `COOKIE_SECURE=false`.
3. Start `GATEWAY_ENV_FILE=.env TERMINAL_SECRETS_FILE=/path/to/terminal.env TERMINALS_FILE=$PWD/config/terminals.json docker compose up --build`.
4. Open `http://localhost:18080/app/`.

The Compose port is loopback-only. It does not expose PostgreSQL or Prometheus
to the network. The metrics allow list includes the private Docker bridge range
because a host-loopback request reaches the container through that bridge.

## Production release

First create these protected VPS files, owned by the deployment account and
mode `0600`:

```text
/home/abdullah/pushsdk-gateway-runtime/gateway.env
/home/abdullah/pushsdk-gateway-runtime/terminal.env
/home/abdullah/pushsdk-gateway-runtime/terminals.json
```

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
