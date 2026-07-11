#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
project="${1:-firmware/esp32-smoke}"
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

idf.py -C "$project_path" -B "$project_path/build" -p "$port" monitor
