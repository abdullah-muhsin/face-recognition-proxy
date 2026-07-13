#!/usr/bin/env bash
set -euo pipefail

if [ -n "${ESPPORT:-}" ]; then
  port="$ESPPORT"
  if [ ! -w "$port" ]; then
    echo "ESP32 serial port $port is not writable. Add this user to dialout and start a new login session." >&2
    exit 4
  fi

  echo "$port"
  exit 0
fi

mapfile -t stable_ports < <(find /dev/serial/by-id -maxdepth 1 -type l -print 2>/dev/null | sort)
mapfile -t direct_ports < <(find /dev -maxdepth 1 \( -name 'ttyUSB*' -o -name 'ttyACM*' \) -print 2>/dev/null | sort)

if [ "${#stable_ports[@]}" -gt 0 ]; then
  ports=("${stable_ports[@]}")
else
  ports=("${direct_ports[@]}")
fi

case "${#ports[@]}" in
  0)
    echo "No ESP32 serial port found. Expected /dev/ttyUSB* or /dev/ttyACM*." >&2
    exit 2
    ;;
  1)
    port="${ports[0]}"
    if [ ! -w "$port" ]; then
      echo "ESP32 serial port $port is not writable. Add this user to dialout and start a new login session." >&2
      exit 4
    fi

    echo "$port"
    ;;
  *)
    echo "Multiple serial ports found. Set ESPPORT explicitly:" >&2
    printf '  %s\n' "${ports[@]}" >&2
    exit 3
    ;;
esac
