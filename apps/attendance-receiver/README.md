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
composer install
npm ci
npm run build
php artisan migrate
php artisan test
```

The API endpoint is `POST /api/attendance-records` when using `php artisan serve`.
The picture upload endpoint is `PUT /api/attendance-records/{attendanceRecord}/picture`.

Set `ATTENDANCE_BRIDGE_TOKEN` in production and configure the same token on the ESP32 bridge.

## Rocky Runtime

Use the repository-level Rocky helpers from the workspace root:

```bash
./scripts/rocky-install-system-deps.sh
./scripts/rocky-deploy-attendance-receiver.sh
```

The deploy helper builds the frontend assets, creates the MariaDB database/user from `.env`, syncs the app to `/var/www/attendance-receiver`, applies persistent SELinux labels, writes the nginx include under `/etc/nginx/default.d`, runs migrations as the PHP-FPM user, and reloads nginx.

The nginx-served endpoints are:

- `GET /attendance-receiver`
- `GET /attendance-receiver/attendance-records`
- `POST /attendance-receiver/api/attendance-records`
- `PUT /attendance-receiver/api/attendance-records/{attendanceRecord}/picture`

## Container Runtime

For a lightweight demo deployment, use the container helper from the workspace
root:

```bash
ATTENDANCE_RECEIVER_HOST_PORT=8001 \
./scripts/rocky-deploy-attendance-receiver-container.sh
```

The helper defaults `ATTENDANCE_RECEIVER_PUBLIC_URL` to
`http://209.42.26.200:8001`; set that variable only when deploying the demo to a
different host.

The container helper builds the Laravel receiver image, generates and preserves
an application key, stores SQLite and uploaded pictures under
`~/attendance-receiver-runtime`, runs migrations on startup, and binds Apache to
`127.0.0.1:8001` by default. Install `deploy/nginx/attendance-receiver.conf` as
`/etc/nginx/conf.d/attendance-receiver.conf` to expose it through host nginx on
`http://209.42.26.200:8001`.

That hosted demo is now the canonical bridge target. The ESP32 bridge should
post to `http://209.42.26.200:8001/api/attendance-records`.
