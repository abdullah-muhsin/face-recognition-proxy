# Face Recognition Workspace

This repository is organized as a multi-part workspace for the face recognition terminal integration.

## Structure

- `apps/attendance-receiver` - Laravel application served at `/attendance-receiver`.
- `embedded/esp32-wroom-32` - ESP32-WROOM-32 development firmware and helper scripts.
- `docs/devices/hikvision-ds-k1a340fwx` - Hikvision DS-K1A340FWX terminal documentation and API notes.

## ESP32 Development

```bash
cd embedded/esp32-wroom-32
./scripts/esp32-build.sh
ESPPORT=/dev/ttyUSB0 ./scripts/esp32-flash.sh
```

## Laravel Application

The Laravel application is scaffolded under `apps/attendance-receiver` and is configured for PHP 8.3 with a MariaDB/MySQL connection.

```bash
cd apps/attendance-receiver
composer install
```

Nginx serves the app at `http://localhost/attendance-receiver`.

## Local Web Server

Nginx is installed from the official nginx.org stable repository and serves the Laravel app through PHP-FPM.

- Canonical URL: `http://localhost/attendance-receiver`
- App public path: `apps/attendance-receiver/public`
- Nginx site config: `/etc/nginx/conf.d/attendance-receiver.conf`
- Public symlink: `/usr/share/nginx/html/attendance-receiver`

Useful commands:

```bash
sudo systemctl status nginx php-fpm
sudo nginx -t
sudo systemctl reload nginx
```

Local environment files and device credentials are ignored by git.
