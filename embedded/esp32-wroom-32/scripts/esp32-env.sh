#!/usr/bin/env bash
set -euo pipefail

idf_path="${IDF_PATH:-$HOME/esp/esp-idf}"

if [ -e "$idf_path" ]; then
  idf_path="$(readlink -f "$idf_path")"
fi

export IDF_PATH="$idf_path"

if [ ! -f "$IDF_PATH/export.sh" ]; then
  echo "ESP-IDF export script not found at $IDF_PATH/export.sh" >&2
  exit 1
fi

# shellcheck disable=SC1091
source "$IDF_PATH/export.sh" >/dev/null

esp32_clear_stale_build_dir() {
  local build_dir="$1"
  local cache_file="$build_dir/CMakeCache.txt"
  local cmake_path=""
  local ninja_path=""

  if [ ! -f "$cache_file" ]; then
    return
  fi

  cmake_path="$(sed -n 's/^CMAKE_COMMAND:INTERNAL=//p' "$cache_file" | tail -n 1)"
  ninja_path="$(sed -n 's/^CMAKE_MAKE_PROGRAM:FILEPATH=//p' "$cache_file" | tail -n 1)"

  if { [ -n "$cmake_path" ] && [ ! -x "$cmake_path" ]; } || { [ -n "$ninja_path" ] && [ ! -x "$ninja_path" ]; }; then
    echo "Removing stale ESP-IDF build directory at $build_dir; cached CMake/Ninja paths no longer exist." >&2
    rm -rf "$build_dir"
  fi
}

esp32_ccache_enabled() {
  command -v ccache >/dev/null 2>&1 && [ "${IDF_CCACHE_ENABLE:-1}" != "0" ]
}

esp32_idf_py() {
  if esp32_ccache_enabled; then
    IDF_CCACHE_ENABLE=1 idf.py --ccache "$@"
  else
    idf.py "$@"
  fi
}

esp32_reconfigure_for_ccache() {
  local project_dir="$1"
  local build_dir="$2"

  if esp32_ccache_enabled && grep -q '^CCACHE_ENABLE:.*=False$' "$build_dir/CMakeCache.txt" 2>/dev/null; then
    esp32_idf_py -C "$project_dir" -B "$build_dir" reconfigure
  fi
}
