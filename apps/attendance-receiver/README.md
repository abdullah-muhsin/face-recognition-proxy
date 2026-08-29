# Attendance Receiver

Laravel application for receiving ESP32/Hikvision attendance events and viewing the ingested records.

## Ingestion Contract

The Laravel receiver is designed to run on a cloud/server network that may not be able to reach the Hikvision terminal directly. The ESP32 bridge is responsible for all terminal communication:

- Poll the Hikvision terminal on the local attendance-device network.
- POST the attendance metadata and sanitized raw Hikvision event data to Laravel.
- When Laravel reports that a picture upload is required, stream the exact JPEG bytes from the terminal to the receiver's picture upload endpoint.
- Advance its serial cursor only after Laravel accepts the metadata and, when needed, the streamed picture upload.

Laravel validates and persists the posted payload. It does not store Hikvision credentials and never attempts to fetch terminal LAN picture URLs.

## Local Development

```bash
composer run setup
php artisan test
```

The API endpoint is `POST /api/attendance-records` when using `php artisan serve`.
The picture upload endpoint is `PUT /api/attendance-records/{attendanceRecord}/picture`.

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
# Set APP_URL and, when desired, ATTENDANCE_BRIDGE_TOKEN.
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

The VPS does not yet have a Git checkout. Run this bootstrap preflight from the
repository root on a workstation with SSH access:

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

The public bridge endpoints are:

- `POST /api/attendance-records`
- `PUT /api/attendance-records/{attendanceRecord}/picture`

Use the complete public API URL, for example
`http://attendance.example.com/api/attendance-records`, in each bridge’s setup
UI. When `ATTENDANCE_BRIDGE_TOKEN` is set, configure the same value on each
bridge; leave it blank to permit unauthenticated bridge requests.

### Data Operations

Open **Data operations** in the web interface and type `WIPE` to delete every
attendance record and all stored pictures. Back up both persistent paths before
wiping, upgrading, or changing hosts:

```bash
mkdir -p backups
docker compose cp app:/var/lib/attendance-receiver/database.sqlite backups/database.sqlite
docker compose cp app:/var/www/html/storage backups/storage
```

To restore, stop the service, copy both paths back to the `app` container, then
start it again. Never run `docker compose down -v` unless you intentionally want
to delete all attendance records and pictures.
