#!/usr/bin/env bash
set -euo pipefail

if [ -n "${ESPPORT:-}" ]; then
  echo "$ESPPORT"
  exit 0
fi

mapfile -t ports < <(find /dev -maxdepth 1 \( -name 'ttyUSB*' -o -name 'ttyACM*' \) -print 2>/dev/null | sort)

case "${#ports[@]}" in
  0)
    echo "No ESP32 serial port found in WSL. Expected /dev/ttyUSB* or /dev/ttyACM*." >&2
    exit 2
    ;;
  1)
    echo "${ports[0]}"
    ;;
  *)
    echo "Multiple serial ports found. Set ESPPORT explicitly:" >&2
    printf '  %s\n' "${ports[@]}" >&2
    exit 3
    ;;
esac
