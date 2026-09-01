# Push SDK Attendance Demo

This Laravel demo receives canonical attendance records from a dedicated Hikvision Push SDK gateway. It deliberately does not implement the terminal protocol or store terminal credentials.

```text
Hikvision terminal -> outbound HTTPS -> Push SDK gateway -> private Laravel API -> SQLite + private picture storage
```

The terminal must connect outward to the gateway. Do not expose the terminal's ISAPI management ports to make this demo work.

The attendance dashboard is intentionally public and has no accounts or login flow. Keep this demo on a trusted network, or put access controls in front of it when hosting it elsewhere.

## Gateway contract

Only the gateway may call these private routes with `Authorization: Bearer $ATTENDANCE_PUSH_GATEWAY_TOKEN`:

- `POST /api/internal/push-sdk/attendance-records`
- `PUT /api/internal/push-sdk/attendance-records/{id}/picture`

The gateway submits the versioned `attendance.push-sdk.gateway.v1` payload, an immutable source event ID, and an RFC 3339 UTC timestamp. Laravel accepts records only from an explicitly registered active terminal and deduplicates by terminal plus vendor event ID.

The registration/login, PBKDF2, event acknowledgement, and command-channel logic belongs exclusively in the gateway. Use the local Push SDK specification as a reference; do not copy the Windows/WPF demo into production.

## Local development

```bash
composer run setup
php artisan test
```

Register a terminal before the gateway submits events:

```bash
php artisan attendance:terminal:register TERMINAL_SERIAL \
  --name="Front entrance" \
  --model="DS-K1T342MFWX-E1" \
  --time-zone="Asia/Baghdad"
```

## Docker and deployment

Copy `docker/production.env.example` to `.env.production`, set `APP_KEY`, `APP_URL`, and a 32+-character `ATTENDANCE_PUSH_GATEWAY_TOKEN`, then run:

```bash
docker compose up --build -d
```

The Compose project creates the internal `attendance_pushsdk_internal` network and the `attendance_receiver_pushsdk` DNS alias. The gateway joins that network and calls Laravel directly; public Nginx traffic is denied from `/api/internal/push-sdk/`.

Deploy the terminal-facing gateway separately, using its [gateway guide](/home/magnet/services/face-recognition-proxy/apps/pushsdk-gateway/README.md). It owns the Push SDK session and only sends canonical records to this application.

Use [the Push SDK Nginx template](/home/magnet/services/face-recognition-proxy/deploy/nginx/attendance-receiver-pushsdk.conf) for the demo dashboard. The production release command uses its own runtime directory and container:

```bash
./scripts/release-production.sh \
  --receiver pushsdk \
  --project-dir /home/abdullah/face-recognition-proxy \
  --dry-run
```

The default production port is `127.0.0.1:8002`.
