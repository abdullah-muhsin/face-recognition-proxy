#!/usr/bin/env bash
set -euo pipefail

export IDF_PATH="${IDF_PATH:-$HOME/esp/esp-idf}"

if [ ! -f "$IDF_PATH/export.sh" ]; then
  echo "ESP-IDF export script not found at $IDF_PATH/export.sh" >&2
  exit 1
fi

# shellcheck disable=SC1091
source "$IDF_PATH/export.sh" >/dev/null
