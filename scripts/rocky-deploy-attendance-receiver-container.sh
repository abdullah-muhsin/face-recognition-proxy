#!/usr/bin/env bash
set -Eeuo pipefail

if [ "$(id -u)" -eq 0 ]; then
  echo "Run this as the project user. The receiver uses rootless Docker." >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
app_src="$repo_root/apps/attendance-receiver"

container_name="${ATTENDANCE_RECEIVER_CONTAINER_NAME:-attendance_receiver}"
image_name="${ATTENDANCE_RECEIVER_IMAGE_NAME:-attendance-receiver:local}"
bind_address="${ATTENDANCE_RECEIVER_BIND_ADDRESS:-127.0.0.1}"
host_port="${ATTENDANCE_RECEIVER_HOST_PORT:-8001}"
public_url="${ATTENDANCE_RECEIVER_PUBLIC_URL:-http://127.0.0.1}"
data_root="${ATTENDANCE_RECEIVER_DATA_ROOT:-$HOME/attendance-receiver-runtime}"
env_file="$data_root/runtime.env"
app_key_file="$data_root/app-key"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_command docker
require_command curl

docker build -t "$image_name" "$app_src"

mkdir -p "$data_root/data" "$data_root/storage"
chmod 700 "$data_root"

if [ ! -s "$app_key_file" ]; then
  docker run --rm --entrypoint php "$image_name" artisan key:generate --show > "$app_key_file"
  chmod 600 "$app_key_file"
fi

app_key="$(tr -d '\r\n' < "$app_key_file")"

cat > "$env_file" <<ENV
APP_NAME=Attendance Receiver
APP_ENV=production
APP_KEY=$app_key
APP_DEBUG=false
APP_URL=$public_url
ASSET_URL=
DB_CONNECTION=sqlite
DB_DATABASE=/var/lib/attendance-receiver/database.sqlite
SESSION_DRIVER=file
SESSION_PATH=/
CACHE_STORE=file
QUEUE_CONNECTION=sync
FILESYSTEM_DISK=local
LOG_CHANNEL=stack
ATTENDANCE_BRIDGE_TOKEN=${ATTENDANCE_BRIDGE_TOKEN:-}
ENV
chmod 600 "$env_file"

if docker ps -a --format '{{.Names}}' | grep -Fxq "$container_name"; then
  docker rm -f "$container_name" >/dev/null
fi

if command -v ss >/dev/null 2>&1; then
  if ss -ltnH | awk -v port=":$host_port" '$4 ~ /^127\.0\.0\.1:/ && $4 ~ port { found = 1 } END { exit(found ? 0 : 1) }'; then
    echo "127.0.0.1:$host_port is already in use." >&2
    exit 1
  fi
  if ss -ltnH | awk -v port=":$host_port" '$4 ~ /^\[::1\]:/ && $4 ~ port { found = 1 } END { exit(found ? 0 : 1) }'; then
    echo "[::1]:$host_port is already in use." >&2
    exit 1
  fi
fi

docker run -d \
  --name "$container_name" \
  --restart unless-stopped \
  --env-file "$env_file" \
  --publish "$bind_address:$host_port:80" \
  --volume "$data_root/data:/var/lib/attendance-receiver" \
  --volume "$data_root/storage:/var/www/html/storage" \
  "$image_name" >/dev/null

for _ in $(seq 1 30); do
  if curl -fsS "http://$bind_address:$host_port/" >/dev/null; then
    echo "Attendance receiver is running at $public_url"
    exit 0
  fi
  sleep 1
done

docker logs --tail 80 "$container_name" >&2 || true
echo "Attendance receiver container started, but did not become healthy on $bind_address:$host_port." >&2
exit 1
