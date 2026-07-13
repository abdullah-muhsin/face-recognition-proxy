# Face Recognition Workspace

This repository is organized as a multi-part workspace for the face recognition terminal integration.

## Structure

- `apps/attendance-receiver` - Laravel application served at `/attendance-receiver`.
- `embedded/esp32-wroom-32` - ESP32-WROOM-32 development firmware and helper scripts.
- `docs/devices/hikvision-ds-k1a340fwx` - Hikvision DS-K1A340FWX terminal documentation and API notes.

## ESP32 Development

The firmware tools are installed in user space under `/home/magnet/esp/esp-idf-v6.0.2` with Espressif's tool cache in `/home/magnet/.espressif`.

```bash
cd embedded/esp32-wroom-32
./scripts/esp32-build.sh
ESPPORT=/dev/ttyUSB0 ./scripts/esp32-flash.sh
./scripts/esp32-monitor.sh firmware/attendance-bridge /dev/ttyUSB0
```

The default ESP32 firmware is `firmware/attendance-bridge`. It starts an open setup AP by default, joins the configured attendance LAN as a station, polls the Hikvision terminal through ISAPI Digest auth, and posts accepted events to the Laravel receiver.

## Laravel Application

The Laravel application is under `apps/attendance-receiver` and is configured for PHP 8.3 with a MariaDB/MySQL connection.

Rocky Linux 10 packages needed for the Laravel/nginx and ESP32 build sides:

```bash
sudo dnf install -y dnf-plugins-core epel-release
sudo dnf config-manager --set-enabled crb
sudo dnf install -y \
  php php-cli php-fpm php-common php-pdo php-mysqlnd php-xml php-mbstring \
  php-bcmath php-intl php-process php-pecl-zip composer \
  nginx mariadb-server nodejs \
  git curl wget unzip tar rsync gcc make flex bison gperf cmake ninja-build ccache \
  python3 python3-pip python3-devel libusb1 libxcrypt-compat policycoreutils-python-utils
```

Or use the helper, which enables CRB/EPEL, installs the packages, enables services, adds the sudoing user to `dialout`, and removes any old `cmake`/`ninja` bootstrap wheels from the ESP-IDF Python environment:

```bash
./scripts/rocky-install-system-deps.sh
```

```bash
cd apps/attendance-receiver
composer install
npm ci
npm run build
php artisan migrate
php artisan test
```

To deploy the local Rocky nginx/PHP-FPM runtime:

```bash
./scripts/rocky-deploy-attendance-receiver.sh
```

Nginx serves the app at `http://localhost/attendance-receiver`.

ESP32 records are accepted at `POST /attendance-receiver/api/attendance-records` when served by nginx, or `POST /api/attendance-records` when using `php artisan serve`.

## Rocky Local Web Server

Nginx is installed from Rocky AppStream and serves the Laravel app through PHP-FPM.

- Canonical URL: `http://localhost/attendance-receiver`
- Source path: `apps/attendance-receiver`
- Runtime path: `/var/www/attendance-receiver`
- Nginx include: `/etc/nginx/default.d/attendance-receiver.conf`
- PHP-FPM user: `apache`
- SELinux labels: `httpd_sys_content_t` for app files, `httpd_sys_rw_content_t` for `storage` and `bootstrap/cache`

Useful commands:

```bash
sudo systemctl status nginx php-fpm
sudo nginx -t
sudo systemctl reload nginx
```

Local environment files and device credentials are ignored by git.
