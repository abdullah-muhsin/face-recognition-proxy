#!/usr/bin/env bash
set -euo pipefail

if [ "$(id -u)" -eq 0 ]; then
  echo "Run this as the project user. The script uses sudo only for system changes." >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
app_src="$repo_root/apps/attendance-receiver"
env_file="$app_src/.env"
deploy_root="${ATTENDANCE_RECEIVER_DEPLOY_ROOT:-/var/www/attendance-receiver}"
nginx_include="${ATTENDANCE_RECEIVER_NGINX_INCLUDE:-/etc/nginx/default.d/attendance-receiver.conf}"
php_user="${ATTENDANCE_RECEIVER_PHP_USER:-apache}"
php_group="${ATTENDANCE_RECEIVER_PHP_GROUP:-apache}"
db_name="${ATTENDANCE_RECEIVER_DB_NAME:-attendance_receiver}"
db_user="${ATTENDANCE_RECEIVER_DB_USER:-attendance_receiver}"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_safe_identifier() {
  local name="$1"
  local value="$2"

  case "$value" in
    ""|*[!A-Za-z0-9_]*)
      echo "$name must contain only letters, numbers, and underscores." >&2
      exit 1
      ;;
  esac
}

env_value() {
  local key="$1"

  awk -F= -v key="$key" '
    $1 == key {
      value = substr($0, length(key) + 2)
      gsub(/^"|"$/, "", value)
      print value
      found = 1
      exit
    }
    END {
      if (! found) {
        exit 1
      }
    }
  ' "$env_file"
}

set_env_value() {
  local key="$1"
  local value="$2"
  local tmp

  tmp="$(mktemp)"
  awk -v key="$key" -v value="$value" '
    BEGIN { written = 0 }
    $0 ~ "^" key "=" {
      print key "=" value
      written = 1
      next
    }
    { print }
    END {
      if (! written) {
        print key "=" value
      }
    }
  ' "$env_file" >"$tmp"
  mv "$tmp" "$env_file"
}

sql_escape() {
  sed "s/'/''/g" <<<"$1"
}

set_fcontext() {
  local type="$1"
  local pattern="$2"

  sudo semanage fcontext -a -t "$type" "$pattern" >/dev/null 2>&1 \
    || sudo semanage fcontext -m -t "$type" "$pattern" >/dev/null
}

require_safe_identifier ATTENDANCE_RECEIVER_DB_NAME "$db_name"
require_safe_identifier ATTENDANCE_RECEIVER_DB_USER "$db_user"

for command in composer npm php rsync sudo mariadb nginx semanage restorecon getsebool setsebool openssl; do
  require_command "$command"
done

if [ ! -f "$env_file" ]; then
  cp "$app_src/.env.example" "$env_file"
fi

if ! env_value APP_KEY >/dev/null 2>&1 || [ -z "$(env_value APP_KEY)" ]; then
  (cd "$app_src" && php artisan key:generate --force)
fi

set_env_value DB_CONNECTION mysql
set_env_value DB_HOST 127.0.0.1
set_env_value DB_PORT 3306
set_env_value DB_DATABASE "$db_name"
set_env_value DB_USERNAME "$db_user"

db_password="$(env_value DB_PASSWORD 2>/dev/null || true)"
if [ -z "$db_password" ]; then
  db_password="$(openssl rand -hex 24)"
  set_env_value DB_PASSWORD "$db_password"
fi

(cd "$app_src" && composer install)
(cd "$app_src" && npm ci --ignore-scripts)
(cd "$app_src" && npm run build)

sudo systemctl start mariadb

sql_file="$(mktemp)"
escaped_password="$(sql_escape "$db_password")"
cat >"$sql_file" <<SQL
CREATE DATABASE IF NOT EXISTS \`$db_name\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '$db_user'@'localhost' IDENTIFIED BY '$escaped_password';
ALTER USER '$db_user'@'localhost' IDENTIFIED BY '$escaped_password';
CREATE USER IF NOT EXISTS '$db_user'@'127.0.0.1' IDENTIFIED BY '$escaped_password';
ALTER USER '$db_user'@'127.0.0.1' IDENTIFIED BY '$escaped_password';
GRANT ALL PRIVILEGES ON \`$db_name\`.* TO '$db_user'@'localhost';
GRANT ALL PRIVILEGES ON \`$db_name\`.* TO '$db_user'@'127.0.0.1';
FLUSH PRIVILEGES;
SQL
sudo mariadb <"$sql_file"
rm -f "$sql_file"

sudo mkdir -p "$deploy_root"
sudo rsync -a --delete \
  --exclude '.git' \
  --exclude 'node_modules' \
  --exclude '.phpunit.cache' \
  --exclude '.phpunit.result.cache' \
  --exclude 'database/database.sqlite' \
  --exclude 'bootstrap/cache/*.php' \
  --exclude 'storage/framework/cache/data/*' \
  --exclude 'storage/framework/sessions/*' \
  --exclude 'storage/framework/testing/*' \
  --exclude 'storage/framework/views/*' \
  --exclude 'storage/logs/*' \
  "$app_src/" "$deploy_root/"

sudo chown -R root:root "$deploy_root"
sudo find "$deploy_root" -type d -exec chmod 0755 {} +
sudo find "$deploy_root" -type f -exec chmod 0644 {} +
sudo chown root:"$php_group" "$deploy_root/.env"
sudo chmod 0640 "$deploy_root/.env"
sudo chown -R "$php_user:$php_group" "$deploy_root/storage" "$deploy_root/bootstrap/cache"
sudo find "$deploy_root/storage" "$deploy_root/bootstrap/cache" -type d -exec chmod 0775 {} +
sudo find "$deploy_root/storage" "$deploy_root/bootstrap/cache" -type f -exec chmod 0664 {} +

set_fcontext httpd_sys_content_t "$deploy_root(/.*)?"
set_fcontext httpd_sys_rw_content_t "$deploy_root/storage(/.*)?"
set_fcontext httpd_sys_rw_content_t "$deploy_root/bootstrap/cache(/.*)?"
sudo restorecon -RF "$deploy_root"

if getsebool httpd_can_network_connect_db >/dev/null 2>&1; then
  sudo setsebool -P httpd_can_network_connect_db on
fi

sudo tee "$nginx_include" >/dev/null <<NGINX
# Managed by $repo_root/scripts/rocky-deploy-attendance-receiver.sh
location = /attendance-receiver {
    return 301 /attendance-receiver/;
}

location = /attendance-receiver/index.php {
    include fastcgi_params;
    fastcgi_param SCRIPT_FILENAME $deploy_root/public/index.php;
    fastcgi_param SCRIPT_NAME /attendance-receiver/index.php;
    fastcgi_param DOCUMENT_ROOT $deploy_root/public;
    fastcgi_param PATH_INFO "";
    fastcgi_pass php-fpm;
}

location ^~ /attendance-receiver/ {
    rewrite ^/attendance-receiver/(.*)$ /\$1 break;
    root $deploy_root/public;
    try_files \$uri /attendance-receiver/index.php?\$query_string;
}
NGINX

sudo -u "$php_user" php "$deploy_root/artisan" optimize:clear
sudo -u "$php_user" php "$deploy_root/artisan" package:discover --ansi
sudo -u "$php_user" php "$deploy_root/artisan" migrate --force

sudo systemctl start php-fpm nginx
sudo nginx -t
sudo systemctl reload nginx

echo "Attendance receiver deployed at http://localhost/attendance-receiver"
