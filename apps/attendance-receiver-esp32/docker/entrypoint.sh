#!/usr/bin/env bash
set -Eeuo pipefail

data_dir="${ATTENDANCE_RECEIVER_DATA_DIR:-/var/lib/attendance-receiver}"
db_path="${DB_DATABASE:-$data_dir/database.sqlite}"

mkdir -p \
    "$data_dir" \
    "$(dirname "$db_path")" \
    storage/app/private \
    storage/app/public \
    storage/framework/cache/data \
    storage/framework/sessions \
    storage/framework/testing \
    storage/framework/views \
    storage/logs \
    bootstrap/cache

if [ "${DB_CONNECTION:-sqlite}" = "sqlite" ]; then
    touch "$db_path"
fi

if [ -z "${APP_KEY:-}" ]; then
    echo "APP_KEY must be set for the attendance receiver container." >&2
    exit 1
fi

chown -R www-data:www-data "$data_dir" storage bootstrap/cache

php artisan optimize:clear --no-interaction
php artisan migrate --force --no-interaction
php artisan optimize --no-interaction

chown -R www-data:www-data "$data_dir" storage bootstrap/cache

exec "$@"
