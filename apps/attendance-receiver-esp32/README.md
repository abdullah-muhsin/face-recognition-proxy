# ESP32 Attendance Demo

This Laravel demo receives attendance records from the local ESP32 bridge. The bridge uses Digest-authenticated ISAPI polling against the Hikvision terminal, then posts sanitized event metadata and optional JPEG data to Laravel.

```text
Hikvision terminal -> ESP32 bridge -> Laravel -> SQLite + private picture storage
```

This project contains no Push SDK routes, terminal-registration table, gateway token, or vendor Push SDK protocol code.

The attendance dashboard is intentionally public and has no accounts or login flow. Keep this demo on a trusted network, or put access controls in front of it when hosting it elsewhere.

## API

The bridge calls:

- `POST /api/attendance-records`
- `PUT /api/attendance-records/{id}/picture`

Set `ATTENDANCE_BRIDGE_TOKEN` to a long random secret in production and configure the same value on each ESP32 bridge. The bridge must never include device passwords, LAN URLs, or `pictureURL` fields in its submitted payload.

## Local development

```bash
composer run setup
php artisan test
```

## Docker and deployment

Copy `docker/production.env.example` to `.env.production`, set `APP_KEY`, `APP_URL`, and `ATTENDANCE_BRIDGE_TOKEN`, then run:

```bash
docker compose up --build -d
```

Use [the ESP32 Nginx template](/home/magnet/services/face-recognition-proxy/deploy/nginx/attendance-receiver-esp32.conf) for a host deployment. The production release command uses its own runtime directory and container:

```bash
./scripts/release-production.sh \
  --receiver esp32 \
  --project-dir /home/abdullah/face-recognition-proxy \
  --dry-run
```

The default production port is `127.0.0.1:8001`.
