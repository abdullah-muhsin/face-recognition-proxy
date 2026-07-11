# ESP-WROOM-32 Check

Result from this machine:

- The board is detected on Windows as a `Silicon Labs CP210x USB to UART Bridge (COM3)`
- Device ID: `USB\\VID_10C4&PID_EA60\\0001`
- Status: `OK`
- Problem code: `CM_PROB_NONE`
- `usbipd-win` bus ID: `1-4`

WSL/Linux view:

- The board is exposed as `/dev/ttyUSB0`
- Stable ID path: `/dev/serial/by-id/usb-Silicon_Labs_CP2102_USB_to_UART_Bridge_Controller_0001-if00-port0`

Conclusion:

- The ESP board is physically connected and visible to Windows
- The CP2102 driver is installed cleanly and available to Windows as `COM3`
- WSL flashing works after attaching bus ID `1-4` with `usbipd-win`

Development setup:

- ESP-IDF `v6.0.2` is installed in WSL at `/home/magnet-wsl/esp/esp-idf-v6.0.2`
- Smoke firmware exists at `firmware/esp32-smoke`
- Build command verified: `./scripts/esp32-build.sh`
- Flash command verified: `ESPPORT=/dev/ttyUSB0 ./scripts/esp32-flash.sh`
- Monitor verified the smoke firmware heartbeat

See `ESP32_DEVELOPMENT.md` for the exact build, flash, reset, and Windows elevation commands.
