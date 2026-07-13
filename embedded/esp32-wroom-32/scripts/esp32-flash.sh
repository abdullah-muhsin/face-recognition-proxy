#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
project="${1:-firmware/attendance-bridge}"
port="${2:-}"

case "$project" in
  /*) project_path="$project" ;;
  *) project_path="$repo_root/$project" ;;
esac

project_path="$(cd "$project_path" && pwd)"

# shellcheck disable=SC1091
source "$repo_root/scripts/esp32-env.sh"

if [ -z "$port" ]; then
  port="$("$repo_root/scripts/esp32-port.sh")"
fi

if ! grep -q '^CONFIG_IDF_TARGET="esp32"$' "$project_path/sdkconfig" 2>/dev/null; then
  idf.py -C "$project_path" -B "$project_path/build" set-target esp32
fi

idf.py -C "$project_path" -B "$project_path/build" -p "$port" flash
