# Attendance Receiver

Laravel application for receiving ESP32/Hikvision attendance events and viewing the ingested records.

## Local Development

```bash
composer install
npm ci
npm run build
php artisan migrate
php artisan test
```

The API endpoint is `POST /api/attendance-records` when using `php artisan serve`.

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
