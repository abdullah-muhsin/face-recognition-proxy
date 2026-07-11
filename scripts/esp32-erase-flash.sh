#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
port="${1:-}"

# shellcheck disable=SC1091
source "$repo_root/scripts/esp32-env.sh"

if [ -z "$port" ]; then
  port="$("$repo_root/scripts/esp32-port.sh")"
fi

idf.py -p "$port" erase-flash
