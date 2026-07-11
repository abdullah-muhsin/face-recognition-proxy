# ESP32 Development Setup

Status: build, flash, reset, and monitor are working from WSL.

## Installed

- ESP-IDF: `v6.0.2`
- IDF path: `/home/magnet-wsl/esp/esp-idf-v6.0.2`
- Stable symlink: `/home/magnet-wsl/esp/esp-idf`
- ESP tools cache: `/home/magnet-wsl/.espressif`
- Python env: `/home/magnet-wsl/.espressif/python_env/idf6.0_py3.12_env`

Installed ESP32 tools:

- `xtensa-esp-elf`
- `xtensa-esp-elf-gdb`
- `esp32ulp-elf`
- `openocd-esp32`
- `esp-rom-elfs`
- `esptool v5.3.1`

## Workspace Commands

Default project: `firmware/esp32-smoke`

```bash
./scripts/esp32-build.sh
ESPPORT=/dev/ttyUSB0 ./scripts/esp32-flash.sh
./scripts/esp32-monitor.sh firmware/esp32-smoke /dev/ttyUSB0
./scripts/esp32-reset.sh /dev/ttyUSB0
./scripts/esp32-erase-flash.sh /dev/ttyUSB0
```

If exactly one `/dev/ttyUSB*` or `/dev/ttyACM*` exists, the scripts auto-detect it. Otherwise set `ESPPORT`.

## Verified

```bash
./scripts/esp32-build.sh
```

This successfully built:

- `firmware/esp32-smoke/build/bootloader/bootloader.bin`
- `firmware/esp32-smoke/build/partition_table/partition-table.bin`
- `firmware/esp32-smoke/build/esp32_smoke.bin`

## Current Hardware Blocker

Windows detects the ESP board's USB bridge:

- Device: `CP2102 USB to UART Bridge Controller`
- Device ID: `USB\VID_10C4&PID_EA60\0001`
- Current status: `OK`
- Windows port: `COM3`
- `usbipd-win` bus ID: `1-4`
- `usbipd-win` executable: `C:\Program Files\usbipd-win\usbipd.exe`

WSL serial device:

- Port: `/dev/ttyUSB0`
- Stable ID path: `/dev/serial/by-id/usb-Silicon_Labs_CP2102_USB_to_UART_Bridge_Controller_0001-if00-port0`

The official CP210x driver was downloaded to:

```text
C:\Users\magnet\AppData\Local\Temp\cp210x-driver\silabser.inf
```

Driver installation succeeded from elevated Windows PowerShell.

## Windows Attach Steps

Run these from an elevated Windows PowerShell:

```powershell
& "C:\Program Files\usbipd-win\usbipd.exe" bind --busid 1-4
& "C:\Program Files\usbipd-win\usbipd.exe" attach --wsl --busid 1-4
```

Back in WSL:

```bash
ls -l /dev/ttyUSB* /dev/ttyACM*
ESPPORT=/dev/ttyUSB0 ./scripts/esp32-flash.sh
```

## Flash Verification

`ESPPORT=/dev/ttyUSB0 ./scripts/esp32-flash.sh` completed successfully.

Detected chip:

- `ESP32-D0WDQ6`
- Revision: `v1.0`
- Flash: `4MB`
- MAC: `0c:b8:15:c1:9b:50`

The flashed smoke firmware prints:

```text
ESP32 smoke firmware is running
cores=2 revision=100 flash=4MB
heartbeat
```

## Serial Permissions

The current user was added to `dialout`:

```bash
sudo usermod -aG dialout "$USER"
```

Open a fresh WSL shell after this setup so the new group membership is active. For the current session, `/dev/ttyUSB0` was temporarily opened with `sudo chmod a+rw /dev/ttyUSB0`.
