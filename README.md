# Face Recognition Workspace

This repository is organized as a multi-part workspace for the face recognition terminal integration.

## Structure

- `apps/attendance-receiver` - Docker-deployed Laravel application that receives and displays attendance events.
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

The default ESP32 firmware is `firmware/attendance-bridge`. It starts an open
setup AP, then you configure its attendance LAN, Hikvision terminal, and
receiver URL in the setup UI. It persists that configuration in NVS. A private
`local_defaults.h` can supply build-time defaults; copy the tracked example and
never commit the resulting file.

## Laravel Application

The Laravel application is under `apps/attendance-receiver` and is deployed
with Docker Compose. Its production data is SQLite plus uploaded pictures in
named Docker volumes. No host PHP-FPM, nginx, MariaDB, or Rocky-specific deploy
scripts are required.

```bash
cd apps/attendance-receiver
composer run setup
php artisan test
```

For deployment instructions and the receiver API, see
[`apps/attendance-receiver/README.md`](apps/attendance-receiver/README.md).

Rocky Linux remains a valid ESP32 development host; its ESP-IDF prerequisites
are documented with the firmware.
