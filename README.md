# PushSDK Gateway

This repository is a single production application built with Go, PostgreSQL,
Vue 3/Vite (JavaScript), Tailwind CSS, PrimeVue, WebSockets, and Prometheus.

## Service boundary

| Path | Consumer | Purpose |
| --- | --- | --- |
| `/iot/{pushSdkSerial}/global/0-global/model/service/operate/PUSH/...` | Hikvision terminal | Strict PushSDK protocol endpoint |
| `/app/` | Administrators | Vue administration console |
| `/api/v1/...` | Vue console | Same-origin authenticated administration API |
| `/ws/v1/monitor` | Vue administration console | Authenticated replay and live stream of retained operational metadata |
| `/metrics` | Prometheus only | Private metric endpoint |
| `/healthz`, `/readyz` | Infrastructure | Liveness and database readiness |

The HTTPS and secure WebSocket settings on a terminal may both use public port
443. They are separate protocols on the same TLS listener; Nginx routes HTTP
requests by path and upgrades the browser monitoring WebSocket only at `/ws/`.

## Data and security model

PostgreSQL stores registered terminals, raw device-event source values, a
strict one-to-one access-event projection, gateway activity, authenticated
PushSDK protocol state, and hashed administrator sessions. Every valid PushSDK
`eventList` item is deduplicated with the
terminal serial plus its vendor UUID, so device retries are safe and do not
create duplicate events.

The gateway does not contain an HTTP forwarder, queue, SQLite fallback, or an
external data store. A successfully acknowledged device event has already been
committed to PostgreSQL. If PostgreSQL is unavailable, the event is rejected so
the terminal can retry; it is never silently dropped.

Gateway activity retains only status and protocol metadata; it deliberately
excludes encrypted protocol bodies and terminal credentials. Each activity
record is committed to PostgreSQL before its live monitor broadcast, so monitor
history survives a gateway restart. Raw device-event data is retained only in
PostgreSQL and is available only to a signed-in administrator through Event Archive;
it is never broadcast through the monitor or emitted in logs. An event payload
can contain sensitive vendor data, including binary media, so administrator
credentials control access to it.

The PushSDK session checkpoint contains only the negotiated payload mode,
server-issued salt and challenges, iteration count, timestamps, and a hash of
the exact terminal protocol configuration. It never contains a terminal
password. The gateway commits the next challenge before sending each successful
response. On startup it restores only a configuration-matched checkpoint that
is still inside the vendor's three-command-interval challenge window. This
allows a normal short gateway restart to continue the existing session without
turning PushSDK off and on at the terminal. An expired or changed session is
deleted and receives the documented invalid-session response; the gateway never
guesses a challenge or changes terminal configuration.

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
- Every accepted authenticated action durably checkpoints its replacement
  challenge before exposing it in a response. A process restart therefore
  resumes the same strict sequence; it does not introduce a permissive
  previous-challenge fallback.
- `Login` and `Logout` use the standard response envelope. `CommandRequest`
  and `CommandResult` use their documented top-level command fields, while an
  `Event` response is the documented top-level per-event result array; they are
  not wrapped in a generic `data` object.
- Events use an exact JSON envelope, base64 data, unique vendor UUIDs, and one
  of `jsonData`, `xmlData`, `boundaryData`, or empty `noData`.
- The source `eventList.data` base64 string is persisted verbatim for every
  valid item. An independent, versioned read model recognizes only documented
  JSON `AccessControllerEvent` payloads with explicit numeric category and
  subtype codes. It never alters the source payload, infers a missing field,
  or labels an inconsistent subtype; all other JSON, XML, multipart, and binary
  bodies remain explicitly unclassified in Event Archive.
- Event Archive decodes that retained source only in the signed-in administrator
  browser: valid UTF-8 is rendered verbatim, while non-text bytes are shown as
  `\xHH`. The UI never displays the vendor's base64 transport value, omits no
  bytes from its readable representation, and offers an exact-byte download.

## Local run

1. Copy `config/terminals.json.example` to ignored `config/terminals.json` and
   set the terminal mapping. It contains an environment-variable name, never a
   password. Set `loginPasswordDigest` to the one documented algorithm used by
   that exact device firmware.
2. Copy `deploy/production/gateway.env.example` to ignored `.env`, set
   `POSTGRES_PASSWORD`, use a matching `DATABASE_URL`, and set unique administrator
   credentials. Copy `deploy/production/terminal.env.example` to an ignored
   file such as `.local-secrets/terminal.env` and set the terminal password. For
   local HTTP testing explicitly set `COOKIE_SECURE=false`.
3. Start `GATEWAY_ENV_FILE=$PWD/.env TERMINAL_SECRETS_FILE=$PWD/.local-secrets/terminal.env TERMINALS_FILE=$PWD/config/terminals.json docker compose up --build`.
4. Open `http://localhost:18080/app/`.

The Compose port is loopback-only. It does not expose PostgreSQL or Prometheus
to the network. The metrics allow list includes the private Docker bridge range
because a host-loopback request reaches the container through that bridge.

## Frontend development

The administration console uses Tailwind CSS for layout and PrimeVue 4 for
accessible controls, data tables, panels, feedback, and responsive navigation.
Its source is deliberately split into Gateway Board, Event Archive, Terminal
Registry, and Gateway Activity views; it does not render every concern on one
screen.

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
