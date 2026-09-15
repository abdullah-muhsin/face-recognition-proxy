#!/usr/bin/env bash
set -euo pipefail

# Releases only this Compose project. It does not touch the host Nginx
# configuration, certificates, or any unrelated containers.
root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
runtime_root=/home/abdullah/services/config/pushsdk-gateway/production
source_root=/home/abdullah/services/apps/pushsdk-gateway/releases
remote=aruvo-vps
release_id=$(git -C "$root_dir" rev-parse --short=12 HEAD)

if ! git -C "$root_dir" diff --quiet || ! git -C "$root_dir" diff --cached --quiet; then
  echo "Refusing to release an uncommitted tree." >&2
  exit 1
fi

ssh "$remote" "test -f '$runtime_root/gateway.env' && test -f '$runtime_root/terminal.env' && test -f '$runtime_root/terminals.json'"
git -C "$root_dir" archive --format=tar --prefix="$release_id/" HEAD |
  ssh "$remote" "install -d -o abdullah -g abdullah -m 0750 '$source_root' && tar --no-same-owner --owner=abdullah --group=abdullah -xof - -C '$source_root'"

ssh "$remote" "runuser -u abdullah -- /home/abdullah/services/bin/pushsdk-activate-release '$release_id'"
ssh "$remote" "curl --fail --silent --show-error http://127.0.0.1:18080/readyz >/dev/null"
echo "Released pushsdk-gateway:$release_id"
