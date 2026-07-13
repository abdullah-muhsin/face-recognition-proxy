# ESP32-WROOM-32 Development Setup

Status: build works on Rocky Linux 10.2. Flash, reset, and monitor use native Linux serial devices.

## Installed

- ESP-IDF: `v6.0.2`
- IDF path: `/home/magnet/esp/esp-idf-v6.0.2`
- Stable symlink: `/home/magnet/esp/esp-idf`
- ESP tools cache: `/home/magnet/.espressif`
- Python env: `/home/magnet/.espressif/python_env/idf6.0_py3.12_env`

Installed ESP32 tools:

- `xtensa-esp-elf`
- `xtensa-esp-elf-gdb`
- `esp32ulp-elf`
- `openocd-esp32`
- `esp-rom-elfs`
- `esptool v5.3.1`
- Rocky packages: `cmake`, `ninja-build`, `ccache`

System packages:

```bash
sudo dnf install -y dnf-plugins-core epel-release
sudo dnf config-manager --set-enabled crb
sudo dnf install -y \
  git wget flex bison gperf python3 python3-pip python3-devel \
  gcc make cmake ninja-build ccache libusb1
```

ESP-IDF is installed from Espressif's pinned source release and uses Espressif's own tool installer for the Xtensa compiler/debugger, OpenOCD, esptool, and Python package environment:

```bash
mkdir -p "$HOME/esp"
git clone --depth 1 --branch v6.0.2 --recursive --shallow-submodules \
  https://github.com/espressif/esp-idf.git "$HOME/esp/esp-idf-v6.0.2"
ln -sfn "$HOME/esp/esp-idf-v6.0.2" "$HOME/esp/esp-idf"
"$HOME/esp/esp-idf/install.sh" esp32
```

## Workspace Commands

Default project: `firmware/attendance-bridge` from this directory.

```bash
cd embedded/esp32-wroom-32
./scripts/esp32-build.sh
ESPPORT=/dev/ttyUSB0 ./scripts/esp32-flash.sh
./scripts/esp32-monitor.sh firmware/attendance-bridge /dev/ttyUSB0
./scripts/esp32-reset.sh /dev/ttyUSB0
./scripts/esp32-erase-flash.sh /dev/ttyUSB0
```

From the repository root, call the same scripts through `embedded/esp32-wroom-32/scripts/`.

The port helper prefers stable `/dev/serial/by-id/*` names. If no stable name exists and exactly one `/dev/ttyUSB*` or `/dev/ttyACM*` exists, the scripts auto-detect it. Otherwise set `ESPPORT`.

## Verified

```bash
cd embedded/esp32-wroom-32
./scripts/esp32-build.sh
```

This successfully built:

- `firmware/attendance-bridge/build/bootloader/bootloader.bin`
- `firmware/attendance-bridge/build/partition_table/partition-table.bin`
- `firmware/attendance-bridge/build/attendance_bridge.bin`

## Attendance Bridge Firmware

The bridge firmware is a native ESP-IDF app for the ESP32-WROOM-32 board.

- Starts an open setup AP by default, named `AttendanceBridge-xxxxxx`.
- Runs WiFi in `APSTA` mode, so setup AP and station connection can be active at the same time.
- Stores configuration and the delivery cursor in NVS.
- Polls the Hikvision terminal at `http://192.168.1.3` using Digest authentication with the configured username and password.
- Uses `/ISAPI/AccessControl/AcsEvent?format=json` with a serial cursor and sends one event per Laravel POST.
- Advances `last_serial` only after Laravel returns a 2xx response.
- Serves the setup UI at `http://192.168.4.1/` while connected to the setup AP.
- Serves machine status at `/api/status`.

The default Hikvision settings match the current lab device:

- Base URL: `http://192.168.1.3`
- Username: `admin`
- Password: configured in firmware defaults and editable from the setup UI

Set the receiver URL to the Laravel API endpoint that is reachable from the ESP32's station network, for example:

```text
http://192.168.1.2/attendance-receiver/api/attendance-records
```

## Native Linux Serial

Plug the ESP32 USB bridge directly into the Rocky host and check for a serial device:

```bash
ls -l /dev/ttyUSB* /dev/ttyACM*
```

For CP210x boards, the kernel usually creates `/dev/ttyUSB0`. Prefer the stable path under `/dev/serial/by-id/` when it exists.

## Serial Permissions

Add the current user to `dialout` if the serial device is not writable:

```bash
sudo usermod -aG dialout "$USER"
```

Log out and back in after this setup so the new group membership is active.
