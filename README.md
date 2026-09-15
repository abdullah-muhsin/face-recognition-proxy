# PushSDK Gateway

This repository is a single production application built with Go, PostgreSQL,
Vue 3/Vite (JavaScript), Tailwind CSS, PrimeVue, WebSockets, and Prometheus.

## Service boundary

| Path | Consumer | Purpose |
| --- | --- | --- |
| `/iot/{pushSdkSerial}/global/0-global/model/service/operate/PUSH/...` | Hikvision terminal | Strict PushSDK protocol endpoint |
| `/app/` | Administrators | Vue administration console |
| `/api/v1/...` | Vue console | Same-origin authenticated administration API |
| `/api/v1/admin/terminals/{serial}/isapi-commands` | Administrators and approved gateway clients | Queue an exact ISAPI command for one online PushSDK terminal |
| `/api/v1/admin/terminals/{serial}/retained-event-syncs` | Administrators | Reconcile one online terminal's retained access-event archive through PushSDK |
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

## Database bootstrap and terminal seeding

`db/schema.sql` is the canonical, create-only PostgreSQL schema. An empty
database installs it as one baseline and then receives only later forward
migrations. The historical migration chain remains immutable under
`db/migrations/legacy/` so an existing deployment can verify its checksums and
complete an interrupted pre-baseline upgrade safely.

After schema setup, the gateway runs `SeedConfiguredTerminals` from the
protected `terminals.json` inventory. It upserts each configured terminal's
canonical serial, PushSDK ID, protocol settings, and credential fingerprint;
it never stores the terminal password. The seed intentionally starts each
configured terminal offline until that terminal proves an authenticated
PushSDK session. The installed inventory is documented as
[GN0953967](docs/devices/hikvision-ds-k1t342mfwx-e1.md) and
[GN0954003](docs/devices/hikvision-ds-k1t342mfwx-e1-gn0954003.md), each with
its exact non-secret identifiers and seed template.

ISAPI command request bytes and command-result source values can be sensitive
as well. They are retained only in PostgreSQL's command audit and returned only
by the signed-in administration API; gateway activity and its live monitor
contain command metadata only, never command payloads or results.

Retained ISAPI access-event records are a separate archive source. A device's
`AcsEvent.InfoList` records do not carry the PushSDK event UUID used by live
delivery, so the gateway never presents them as live retries. It retains each
exact record JSON segment with a SHA-256 source fingerprint scoped to the
terminal. A subsequent identical search result is counted as already retained;
a changed source record is a distinct retained record. The full source page
remains in the related ISAPI command audit as well.

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
  not wrapped in a generic `data` object. `CommandRequest` delivers at most 20
  durable, UUID-correlated ISAPI commands in vendor format. `CommandResult`
  accepts the exact Base64 result value for one of those sent commands. This
  terminal family may omit result `dataFormat`; the gateway records that
  explicit absence and the exact Base64 value without inferring, substituting,
  or normalizing a format. A workflow that has a documented response contract,
  such as retained-event reconciliation, validates the raw response bytes
  directly against that contract.
- The signed-in administration console provides an ISAPI Console and the same
  capability is available through `POST
  /api/v1/admin/terminals/{serial}/isapi-commands`. It requires `method`,
  `url`, `dataFormat`, and `expiresInSeconds`; JSON and XML use `textData`,
  multipart uses `dataBase64`, and `noData` carries neither. The URL is an
  exact absolute `/ISAPI/` path. The gateway does not open direct connections
  to terminals, infer an omitted data format, rewrite paths or payloads, or
  retain an unbounded command. The selected expiry is a command deadline: the
  terminal must collect the command and return its result before it; otherwise
  the command expires durably.
- Terminal Registry can queue one retained-event reconciliation for an online
  terminal. The workflow first sends `GET /ISAPI/System/time` and accepts only
  the documented `Time.localTime` XML response. It then uses that terminal time
  to create the exact all-category `POST /ISAPI/AccessControl/AcsEvent?format=json`
  request: `major: 0`, `minor: 0`, 30 records per vendor-supported page, from
  `2000-01-01` through the device's reported current time. Every next page is
  queued only after the preceding command result, response status, position,
  count, search ID, and total are validated. `MORE`, `OK`, and `NO MATCH` are
  distinct documented states; an inconsistent or unsupported response ends the
  run with a durable failure instead of filling gaps or retrying under altered
  conditions.
- Events use an exact JSON envelope, base64 data, unique vendor UUIDs, and one
  of `jsonData`, `xmlData`, `boundaryData`, or empty `noData`.
- The source `eventList.data` base64 string is persisted verbatim for every
  valid item. An independent, versioned read model recognizes only documented
  JSON `AccessControllerEvent` payloads with explicit numeric category and
  subtype codes, including the documented multipart form part of a
  `boundaryData` envelope. It projects only declared event context, including
  state, device metadata, event sequence values, verification policy, picture
  count, and face-rectangle coordinates. It never alters the source payload,
  infers a missing field, or labels an inconsistent subtype: only a
  model-verified vendor subtype catalog supplies labels. All other JSON, XML,
  multipart, and binary bodies remain explicitly unclassified in Event Archive.
- Event Archive decodes that retained source only in the signed-in administrator
  browser: valid UTF-8 is rendered verbatim, while non-text bytes are shown as
  `\xHH`. The UI never displays the vendor's base64 transport value, omits no
  bytes from its readable representation, and offers an exact-byte download.
  For a documented multipart `Picture` part explicitly declared as JPEG, it
  also displays those exact image bytes without resizing, transcoding, or
  reconstructing the source.

## ISAPI command API

The ISAPI Console and automation use the same signed-in administration API.
Commands are delivered only through a configured terminal's authenticated
PushSDK `CommandRequest` poll; this is not a gateway HTTP proxy to the terminal.

`POST /api/v1/admin/terminals/{serial}/isapi-commands` accepts exactly one JSON
request of this form:

```json
{
  "method": "GET",
  "url": "/ISAPI/System/deviceInfo",
  "dataFormat": "noData",
  "expiresInSeconds": 60
}
```

`method` is exactly `GET`, `POST`, `PUT`, or `DELETE`. `url` is an exact,
visible-ASCII absolute `/ISAPI/` path with an optional one query string. A
`jsonData` or `xmlData` command instead includes non-empty `textData`; a
`boundaryData` command includes non-empty canonical-standard `dataBase64` for
the complete multipart bytes. `noData` carries neither field. Every request
must explicitly choose a command deadline from 1 to 3600 seconds. The terminal
must collect the command and return its result before that deadline. The
terminal must be online when the command is queued; otherwise the endpoint
returns `409` and does not retain the request.

The gateway deliberately has no ISAPI endpoint catalog and does not impose a
method-to-payload pairing: every combination of those vendor-supported methods
and request formats is transported exactly as supplied. The selected terminal
model's ISAPI documentation remains the authority for a route's semantics and
required payload format.

`GET /api/v1/admin/isapi-commands` lists the durable command audit history and
`GET /api/v1/admin/isapi-commands/{uuid}` returns its exact request bytes and
the terminal's exact response Base64 value together with whether it declared a
response format. All three endpoints use the existing signed-in administrator
session, so no terminal credential or separate device-facing authorization is
exposed to the caller.

## Retained-event reconciliation API

`POST /api/v1/admin/terminals/{serial}/retained-event-syncs` has no request
body or query parameters. It returns `201` with the durable run state and
queues the initial terminal-time command only when the terminal is currently
online. There can be one active reconciliation per terminal; a second request
returns `409` rather than competing for the device archive.

`GET /api/v1/admin/retained-event-syncs/{uuid}` returns that run's exact
lifecycle state, device-time bounds, page count, total matches, imported count,
duplicate count, and any terminal-response failure. Event Archive presents live
PushSDK deliveries and retained ISAPI records together, with an explicit source
selector and source-specific payload endpoint. The payload route is
`GET /api/v1/admin/events/{source}/{id}/payload`, where `source` is exactly
`pushsdk` or `isapi`; retained payloads are the exact `InfoList` record bytes.

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

Session recovery tests cover expiry, a fresh login after rejection, and delayed
requests from an old session. To also run the PostgreSQL transaction test, set
`GATEWAY_TEST_DATABASE_URL` when running `go test ./internal/store`; it creates
and removes a dedicated test schema in that database.

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
