# Attendance Receiver

Laravel application for receiving attendance events and viewing protected records and images.

## Integration Boundaries

There are two deliberately separate ingestion paths:

- **Legacy ESP32/ISAPI** — the existing public `/api/attendance-records` API. The ESP32 polls a local terminal, submits sanitized metadata, and streams the exact JPEG only when requested.
- **Direct Push SDK** — a dedicated, stateful Push SDK gateway receives the terminal's MQTT/WebSocket traffic and sends a strict, private HTTP contract to Laravel. A Hikvision terminal never connects to Laravel directly.

```text
Hikvision terminal -> Push SDK gateway -> private Laravel API -> database + image storage
```

The gateway is responsible for terminal authentication, vendor-protocol acknowledgement, durable retry, media/event correlation, and retaining any vendor payload needed for short-term diagnostics. Laravel accepts only the canonical attendance contract below; it does not accept raw Push SDK messages, device credentials, LAN URLs, or inline image data.

### Push SDK gateway contract

The gateway calls `POST /api/internal/push-sdk/attendance-records` over the private Docker network with `Authorization: Bearer $ATTENDANCE_PUSH_GATEWAY_TOKEN`.

```json
{
  "schema": "attendance.push-sdk.gateway.v1",
  "terminal_serial_number": "K1T342MFWXE1...",
  "source_event_id": "immutable-vendor-event-id",
  "occurred_at": "2026-08-31T09:15:30Z",
  "event": {
    "employee_number": "1001",
    "employee_name": "Example Person",
    "verification_method": "face",
    "attendance_status": "check_in",
    "status_value": 1,
    "picture_expected": true
  }
}
```

Rules are intentional and strict:

- `terminal_serial_number` must be pre-registered and active; Laravel never self-registers a terminal from network input.
- `source_event_id` must be the immutable vendor event identity supplied by the gateway. Laravel deduplicates on `(terminal, source_event_id)` at the database level.
- `occurred_at` is UTC RFC 3339 with second precision. The gateway converts the terminal-local time before submission; Laravel also records its own `received_at` time.
- A retry with the same identity must have the same canonical payload hash. Different data for the same event is rejected with `409 Conflict` rather than silently overwritten.
- When `picture_expected` is true, Laravel returns a **relative** private upload path. The gateway streams JPEG bytes to that path with the same bearer token. The existing 2 MiB default is configurable with `ATTENDANCE_PICTURE_MAX_BYTES`.

The direct Push SDK listener is intentionally not included yet. Its MQTT topics, registration/acknowledgement semantics, and WebSocket media correlation must be implemented from the exact model/firmware Push SDK guide and captured device traffic—never guessed from a generic MQTT or WebSocket library.

## Operator access

The web interface, images, and data wipe operation require an operator session. Create the first operator after deployment:

```bash
docker compose exec app php artisan attendance:operator:create operator@example.com --name="Attendance Administrator"
```

The command prompts for a password and requires at least 16 characters. Wiping all records also requires the currently signed-in operator's password.

Before a direct terminal can submit events, register it explicitly:

```bash
docker compose exec app php artisan attendance:terminal:register TERMINAL_SERIAL \
  --name="Front entrance" \
  --model="DS-K1T342MFWX-E1" \
  --time-zone="Asia/Baghdad"
```

## Local Development

```bash
composer run setup
php artisan test
```

The legacy ESP32 API endpoint is `POST /api/attendance-records` when using `php artisan serve`.
Its picture endpoint is `PUT /api/attendance-records/{attendanceRecord}/picture`.

Set `ATTENDANCE_BRIDGE_TOKEN` in production and configure the same token on the ESP32 bridge.

## Production Deployment

Docker Compose is the supported deployment path. It runs Apache/PHP and uses
SQLite with named volumes for the database and application storage; the host
does not need PHP, PHP-FPM, MariaDB, or a Rocky-specific helper.

This is a single-instance service. Do not run multiple application containers
against these SQLite volumes.

From this directory:

```bash
cp docker/production.env.example .env.production
# Set APP_URL, ATTENDANCE_BRIDGE_TOKEN, and ATTENDANCE_PUSH_GATEWAY_TOKEN.
docker compose run --rm --no-deps --entrypoint php app artisan key:generate --show
# Put the generated value in APP_KEY in .env.production.
docker compose up --build -d
```

### Release To The Production VPS

Push the release branch to `origin` first. The current Aruvo deployment runs
rootless Docker as `abdullah`, with its existing SQLite database and uploaded
pictures bind-mounted from `/home/abdullah/attendance-receiver-runtime`. The
release script validates those exact mounts and the `127.0.0.1:8001` binding,
builds the new image before cutover, and retains the stopped prior container
for rollback. It does not run Docker Compose or modify any other container.

For an initial VPS checkout, run this bootstrap preflight from the repository
root on a workstation with SSH access:

```bash
# Validate the live container and confirm the one-time checkout that would be
# created. This does not modify the VPS.
./scripts/release-production.sh \
  --host vps-aruvo \
  --project-dir /home/abdullah/face-recognition-proxy \
  --bootstrap \
  --dry-run

# After the intended commits are pushed to origin/main, clone, build, and
# replace the attendance container while preserving its existing data.
./scripts/release-production.sh \
  --host vps-aruvo \
  --project-dir /home/abdullah/face-recognition-proxy \
  --bootstrap

# Later releases only need the normal preflight and release commands.
./scripts/release-production.sh \
  --host vps-aruvo \
  --project-dir /home/abdullah/face-recognition-proxy \
  --dry-run
```

`vps-aruvo` is the configured SSH alias in this workspace. `--bootstrap` is
deliberately required only for the initial clone. Use `--branch` to release a
branch other than `main`. Later releases stop if tracked or untracked files are
present remotely, so they cannot silently overwrite an ad-hoc production edit
or build files outside the Git revision.

The service is bound to `127.0.0.1:8001` by default. To expose a different host
port or address, set these shell variables before `docker compose up`:

```bash
ATTENDANCE_RECEIVER_BIND_ADDRESS=127.0.0.1
ATTENDANCE_RECEIVER_HOST_PORT=8001
```

For a public deployment, use your existing reverse proxy when one is needed. A
generic nginx template is available at
[`../../deploy/nginx/attendance-receiver.conf`](../../deploy/nginx/attendance-receiver.conf);
replace its example hostname before enabling it.

The public legacy ESP32 endpoints are:

- `POST /api/attendance-records`
- `PUT /api/attendance-records/{attendanceRecord}/picture`

Use the complete public HTTPS API URL, for example
`https://attendance.example.com/api/attendance-records`, in each ESP32 bridge’s
setup UI. Configure `ATTENDANCE_BRIDGE_TOKEN` on every bridge; do not deploy a
public receiver without it.

`/api/internal/push-sdk/*` is not public. The nginx template denies it, and a
future Push SDK gateway must call the receiver over the private Docker network.
The gateway token is only for that internal service-to-service hop; it is not a
terminal password, must be at least 32 characters, and must differ from the
ESP32 token.

### Intentional Schema Reset

The attendance schema is a clean baseline: one creator migration for terminals
and one for records. This release deliberately requires a fresh database; it
does not contain an upgrade compatibility migration.

Use the release script's explicit `--fresh-database` mode for this one release:

```bash
./scripts/release-production.sh \
  --host vps-aruvo \
  --project-dir /home/abdullah/face-recognition-proxy \
  --fresh-database \
  --dry-run

./scripts/release-production.sh \
  --host vps-aruvo \
  --project-dir /home/abdullah/face-recognition-proxy \
  --fresh-database
```

It creates a timestamped backup of this app's SQLite database and attendance
pictures under `/home/abdullah/attendance-receiver-runtime/backups`, verifies
the archive checksums, then stops only `attendance_receiver`, runs
`migrate:fresh`, and removes the now-orphaned attendance pictures. If the
fresh migration or new-container health check fails, the script restores that
backup before starting the prior attendance container. The reset drops all
application tables, including attendance records, operator accounts, registered
terminals, cache, and jobs.

Then recreate the operator and each direct Push SDK terminal before resuming
event delivery. Do not use `--fresh-database` during a normal future release.

### Data Operations

Open **Data operations** in the web interface, type `WIPE`, and provide your
current operator password to delete every attendance record and all stored
pictures. Back up both persistent paths before wiping, upgrading, or changing
hosts:

```bash
mkdir -p backups
docker compose cp app:/var/lib/attendance-receiver/database.sqlite backups/database.sqlite
docker compose cp app:/var/www/html/storage backups/storage
```

To restore, stop the service, copy both paths back to the `app` container, then
start it again. Never run `docker compose down -v` unless you intentionally want
to delete all attendance records and pictures.
