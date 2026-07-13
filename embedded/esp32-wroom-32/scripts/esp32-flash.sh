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
build_path="$project_path/build"

# shellcheck disable=SC1091
source "$repo_root/scripts/esp32-env.sh"

esp32_clear_stale_build_dir "$build_path"
esp32_reconfigure_for_ccache "$project_path" "$build_path"

if [ -z "$port" ]; then
  port="$("$repo_root/scripts/esp32-port.sh")"
fi

if ! grep -q '^CONFIG_IDF_TARGET="esp32"$' "$project_path/sdkconfig" 2>/dev/null; then
  esp32_idf_py -C "$project_path" -B "$build_path" set-target esp32
fi

esp32_idf_py -C "$project_path" -B "$build_path" -p "$port" flash
