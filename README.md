# Face Recognition Integration Demos

This workspace contains two independent Laravel demonstrations for the same Hikvision attendance-terminal outcome. They intentionally do not share an application or database.

- `apps/attendance-receiver-esp32` — an ESP32 bridge polls the terminal locally through ISAPI and sends attendance records to Laravel.
- `apps/attendance-receiver-pushsdk` — a dedicated Push SDK gateway receives the terminal's outbound Push SDK connection and sends canonical attendance records to Laravel privately.

The ESP32 firmware remains under `embedded/esp32-wroom-32`. Terminal research and tested ISAPI notes are under `docs/devices/`.

## Choose one demo

Use the ESP32 receiver when demonstrating a simple local-network bridge. Use the Push SDK receiver when demonstrating outbound, stateful device integration without exposing the terminal's management interface.

Each project has its own README, SQLite storage, Docker image, environment file, Nginx template, and release target. Do not point both demos at the same runtime directory or database.

The vendor Push SDK archives in `third_party/hikvision` are local reference material only. They include executables, databases, and certificate files and must not be committed or deployed as application dependencies.
