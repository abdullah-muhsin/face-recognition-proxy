#!/usr/bin/env bash
set -euo pipefail

# Releases only this Compose project. It does not touch the host Nginx
# configuration, certificates, or any unrelated containers.
root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
runtime_root=/home/abdullah/pushsdk-gateway-runtime
source_root=/home/abdullah/pushsdk-gateway-src
remote=aruvo-vps
release_id=$(git -C "$root_dir" rev-parse --short=12 HEAD)

if ! git -C "$root_dir" diff --quiet || ! git -C "$root_dir" diff --cached --quiet; then
  echo "Refusing to release an uncommitted tree." >&2
  exit 1
fi

ssh "$remote" "test -f '$runtime_root/gateway.env' && test -f '$runtime_root/terminal.env' && test -f '$runtime_root/terminals.json'"
git -C "$root_dir" archive --format=tar --prefix="$release_id/" HEAD |
  ssh "$remote" "install -d -o abdullah -g abdullah -m 0750 '$source_root' && tar --no-same-owner --owner=abdullah --group=abdullah -xof - -C '$source_root'"

ssh "$remote" "su -s /bin/bash abdullah -c 'export PATH=/home/abdullah/bin:\$PATH XDG_RUNTIME_DIR=/run/user/1000 DOCKER_HOST=unix:///run/user/1000/docker.sock; set -a; . \"$runtime_root/gateway.env\"; set +a; cd \"$source_root/$release_id\"; GATEWAY_ENV_FILE=\"$runtime_root/gateway.env\" TERMINAL_SECRETS_FILE=\"$runtime_root/terminal.env\" TERMINALS_FILE=\"$runtime_root/terminals.json\" GATEWAY_IMAGE=\"pushsdk-gateway:$release_id\" docker compose -p pushsdk-gateway up --detach --build --remove-orphans'"
ssh "$remote" "curl --fail --silent --show-error http://127.0.0.1:18080/readyz >/dev/null"
echo "Released pushsdk-gateway:$release_id"
