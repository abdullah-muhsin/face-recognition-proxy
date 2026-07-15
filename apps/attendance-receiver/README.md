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
