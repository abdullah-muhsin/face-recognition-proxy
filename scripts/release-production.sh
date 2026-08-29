#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
    cat <<'USAGE'
Release Attendance Receiver to the Aruvo production VPS.

Usage:
  scripts/release-production.sh --project-dir PATH [options]

Required:
  --project-dir PATH      Absolute path for this repository's VPS checkout.

Options:
  --host SSH_TARGET       SSH target (default: vps-aruvo).
  --deploy-user USER      VPS user that owns rootless Docker (default: abdullah).
  --runtime-dir PATH      Existing data/config directory
                          (default: /home/abdullah/attendance-receiver-runtime).
  --branch NAME           Git branch to release (default: main).
  --bootstrap             Permit the one-time clone when --project-dir does not
                          yet exist. The existing runtime data is retained.
  --dry-run               Validate the server and report the pending release;
                          do not change the checkout, image, or container.
  -h, --help              Show this help text.

Environment equivalents:
  ARUVO_SSH_TARGET, ARUVO_DEPLOY_USER, ARUVO_PROJECT_DIR,
  ARUVO_RUNTIME_DIR, ARUVO_RELEASE_BRANCH

The script builds a new image before stopping the live attendance container.
It validates and reuses the existing data and storage bind mounts, then starts
the replacement on 127.0.0.1:8001. It verifies the application response from
inside the container, never touches unrelated containers, and retains the old
container as a stopped rollback candidate.
USAGE
}

ssh_target="${ARUVO_SSH_TARGET:-vps-aruvo}"
deploy_user="${ARUVO_DEPLOY_USER:-abdullah}"
project_dir="${ARUVO_PROJECT_DIR:-}"
runtime_dir="${ARUVO_RUNTIME_DIR:-/home/abdullah/attendance-receiver-runtime}"
branch="${ARUVO_RELEASE_BRANCH:-main}"
bootstrap=false
dry_run=false

require_value() {
    [ "$#" -ge 2 ] || {
        echo "$1 requires a value." >&2
        exit 2
    }
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --host)
            require_value "$@"
            ssh_target="$2"
            shift 2
            ;;
        --deploy-user)
            require_value "$@"
            deploy_user="$2"
            shift 2
            ;;
        --project-dir)
            require_value "$@"
            project_dir="$2"
            shift 2
            ;;
        --runtime-dir)
            require_value "$@"
            runtime_dir="$2"
            shift 2
            ;;
        --branch)
            require_value "$@"
            branch="$2"
            shift 2
            ;;
        --bootstrap)
            bootstrap=true
            shift
            ;;
        --dry-run)
            dry_run=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if [ -z "$project_dir" ]; then
    echo "--project-dir is required." >&2
    usage >&2
    exit 2
fi

if [[ "$project_dir" != /* ]] || [[ "$runtime_dir" != /* ]]; then
    echo "--project-dir and --runtime-dir must be absolute VPS paths." >&2
    exit 2
fi

# SSH joins remote command arguments into one command string. Restrict values
# passed after `bash -s --` so they cannot change that command's meaning.
is_safe_remote_argument() {
    [[ "$1" =~ ^[A-Za-z0-9._/@:+,=-]+$ ]]
}

for value in "$deploy_user" "$project_dir" "$runtime_dir" "$branch"; do
    if ! is_safe_remote_argument "$value"; then
        echo "Unsupported character in a remote argument: $value" >&2
        exit 2
    fi
done

git check-ref-format --branch "$branch" >/dev/null 2>&1 \
    || { echo "--branch is not a valid Git branch name: $branch" >&2; exit 2; }

origin_url="$(git remote get-url origin)"
is_safe_remote_argument "$origin_url" \
    || { echo "The origin URL contains unsupported characters." >&2; exit 2; }

echo "Releasing '$branch' to '$ssh_target' as '$deploy_user'"

ssh -- "$ssh_target" bash -s -- \
    "$deploy_user" "$project_dir" "$runtime_dir" "$branch" \
    "$origin_url" "$bootstrap" "$dry_run" <<'REMOTE_SCRIPT'
#!/usr/bin/env bash
set -Eeuo pipefail

deploy_user="$1"
project_dir="$2"
runtime_dir="$3"
branch="$4"
origin_url="$5"
bootstrap="$6"
dry_run="$7"
app_dir="$project_dir/apps/attendance-receiver"
runtime_env="$runtime_dir/runtime.env"
data_dir="$runtime_dir/data"
storage_dir="$runtime_dir/storage"
container_name="attendance_receiver"
bind_address="127.0.0.1"
host_port="8001"

fail() {
    echo "release-production: $*" >&2
    exit 1
}

deploy_home="$(getent passwd "$deploy_user" | cut -d: -f6)"
[ -n "$deploy_home" ] || fail "VPS user does not exist: $deploy_user"
deploy_uid="$(id -u "$deploy_user")"
docker_path="$deploy_home/bin:/usr/local/bin:/usr/bin:/bin"

as_deploy() {
    if [ "$(id -un)" = "$deploy_user" ]; then
        env HOME="$deploy_home" XDG_RUNTIME_DIR="/run/user/$deploy_uid" PATH="$docker_path" "$@"
    elif [ "$(id -u)" -eq 0 ]; then
        runuser -u "$deploy_user" -- env \
            HOME="$deploy_home" \
            XDG_RUNTIME_DIR="/run/user/$deploy_uid" \
            PATH="$docker_path" \
            "$@"
    else
        fail "SSH user must be '$deploy_user' or root so it can run Docker as '$deploy_user'"
    fi
}

[ -d "$runtime_dir" ] || fail "runtime directory not found: $runtime_dir"
[ -d "$data_dir" ] || fail "data directory not found: $data_dir"
[ -d "$storage_dir" ] || fail "storage directory not found: $storage_dir"
[ -f "$runtime_env" ] || fail "runtime environment file not found: $runtime_env"
[ -r "$runtime_env" ] || fail "runtime environment file is not readable: $runtime_env"
as_deploy docker version >/dev/null 2>&1 || fail "rootless Docker is unavailable for '$deploy_user'"

expected_mounts="$(printf '%s\n%s\n' \
    "$data_dir|/var/lib/attendance-receiver" \
    "$storage_dir|/var/www/html/storage")"
existing_id="$(as_deploy docker container ls -aq --filter "name=^/${container_name}$")"

if [ -n "$existing_id" ]; then
    [ "$(as_deploy docker inspect --format '{{.State.Running}}' "$existing_id")" = true ] \
        || fail "existing '$container_name' container is not running; inspect it before releasing"
    existing_mounts="$(as_deploy docker inspect --format '{{range .Mounts}}{{printf "%s|%s\n" .Source .Destination}}{{end}}' "$existing_id")"
    while IFS= read -r expected_mount; do
        printf '%s\n' "$existing_mounts" | grep -Fqx "$expected_mount" \
            || fail "existing '$container_name' does not use expected mount: $expected_mount"
    done <<< "$expected_mounts"
    existing_port="$(as_deploy docker inspect --format '{{range $port, $bindings := .HostConfig.PortBindings}}{{range $bindings}}{{printf "%s:%s\n" .HostIp .HostPort}}{{end}}{{end}}' "$existing_id")"
    printf '%s\n' "$existing_port" | grep -Fqx "$bind_address:$host_port" \
        || fail "existing '$container_name' is not bound to $bind_address:$host_port"
fi

if [ ! -d "$project_dir/.git" ]; then
    [ ! -e "$project_dir" ] || fail "not a Git checkout: $project_dir"
    [ "$bootstrap" = true ] || fail "missing checkout; rerun with --bootstrap to clone it"

    if [ "$dry_run" = true ]; then
        origin_commit="$(as_deploy git ls-remote --exit-code --refs "$origin_url" "refs/heads/$branch" | awk 'NR == 1 { print $1 }')" \
            || fail "could not read $branch from origin"
        echo "Bootstrap dry run passed. Would clone $origin_url at $origin_commit into $project_dir."
        exit 0
    fi

    if [ "$(id -un)" = "$deploy_user" ]; then
        mkdir -p "$(dirname "$project_dir")"
    else
        install -d -o "$deploy_user" -g "$(id -gn "$deploy_user")" "$(dirname "$project_dir")"
    fi
    as_deploy git clone --branch "$branch" --single-branch "$origin_url" "$project_dir"
fi

[ -d "$app_dir" ] || fail "attendance receiver directory not found: $app_dir"
as_deploy git -C "$project_dir" diff --quiet \
    || fail "the VPS checkout has unstaged tracked changes"
as_deploy git -C "$project_dir" diff --cached --quiet \
    || fail "the VPS checkout has staged tracked changes"
[ -z "$(as_deploy git -C "$project_dir" status --porcelain --untracked-files=all)" ] \
    || fail "the VPS checkout has untracked files; move them outside the checkout"

current_branch="$(as_deploy git -C "$project_dir" symbolic-ref --quiet --short HEAD)" \
    || fail "the VPS checkout is detached; check out '$branch' before releasing"
[ "$current_branch" = "$branch" ] \
    || fail "the VPS checkout is on '$current_branch', expected '$branch'"

if [ "$dry_run" = true ]; then
    origin_commit="$(as_deploy git -C "$project_dir" ls-remote --exit-code --refs origin "refs/heads/$branch" | awk 'NR == 1 { print $1 }')" \
        || fail "could not read origin/$branch"
    current_commit="$(as_deploy git -C "$project_dir" rev-parse HEAD)"
    echo "Dry run passed. VPS commit: $current_commit"
    echo "Origin commit:  $origin_commit"
    exit 0
fi

as_deploy git -C "$project_dir" fetch --prune origin "$branch"
as_deploy git -C "$project_dir" pull --ff-only origin "$branch"

deployed_commit="$(as_deploy git -C "$project_dir" rev-parse HEAD)"
origin_commit="$(as_deploy git -C "$project_dir" rev-parse "origin/$branch")"
[ "$deployed_commit" = "$origin_commit" ] \
    || fail "VPS checkout is not exactly at origin/$branch after fast-forward"

image_tag="attendance-receiver:release-${deployed_commit:0:12}"
as_deploy docker build --tag "$image_tag" "$app_dir"

previous_container=""
new_container_started=false
rollback() {
    status="${1:-$?}"
    if [ "$new_container_started" = true ]; then
        as_deploy docker rm --force "$container_name" >/dev/null 2>&1 || true
    fi
    if [ -n "$previous_container" ]; then
        as_deploy docker rename "$previous_container" "$container_name" >/dev/null 2>&1 || true
        as_deploy docker start "$container_name" >/dev/null 2>&1 || true
        echo "release-production: restored the previous attendance container" >&2
    fi
    exit "$status"
}
trap 'rollback $?' ERR

if [ -n "$existing_id" ]; then
    previous_container="${container_name}_previous_$(date -u +%Y%m%dT%H%M%SZ)"
    as_deploy docker rename "$container_name" "$previous_container"
    as_deploy docker stop "$previous_container" >/dev/null
fi

as_deploy docker run --detach \
    --name "$container_name" \
    --restart unless-stopped \
    --env-file "$runtime_env" \
    --publish "$bind_address:$host_port:80" \
    --volume "$data_dir:/var/lib/attendance-receiver" \
    --volume "$storage_dir:/var/www/html/storage" \
    "$image_tag" >/dev/null
new_container_started=true

deadline=$((SECONDS + 120))
while [ "$SECONDS" -lt "$deadline" ]; do
    if as_deploy docker exec "$container_name" php -r \
        'exit(@file_get_contents("http://127.0.0.1/") === false ? 1 : 0);'; then
        trap - ERR
        echo "Release complete at $deployed_commit; attendance receiver is responding."
        if [ -n "$previous_container" ]; then
            echo "Previous container retained for rollback: $previous_container"
        fi
        exit 0
    fi
    sleep 2
done

as_deploy docker logs --tail=100 "$container_name" >&2 || true
echo "release-production: timed out waiting for the attendance receiver response" >&2
rollback 1
REMOTE_SCRIPT
