#!/usr/bin/env bash
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  exec sudo "$0" "$@"
fi

dnf install -y dnf-plugins-core epel-release
dnf config-manager --set-enabled crb

dnf install -y \
  php php-cli php-fpm php-common php-pdo php-mysqlnd php-xml php-mbstring \
  php-bcmath php-intl php-process php-pecl-zip composer \
  nginx mariadb-server nodejs \
  git curl wget unzip tar rsync gcc make flex bison gperf cmake ninja-build ccache \
  python3 python3-pip python3-devel libusb1 libxcrypt-compat policycoreutils-python-utils

if [ -n "${SUDO_USER:-}" ] && [ "$SUDO_USER" != "root" ] && getent group dialout >/dev/null; then
  usermod -aG dialout "$SUDO_USER"

  user_home="$(getent passwd "$SUDO_USER" | cut -d: -f6)"
  idf_pip="$user_home/.espressif/python_env/idf6.0_py3.12_env/bin/pip"
  if [ -x "$idf_pip" ]; then
    runuser -u "$SUDO_USER" -- "$idf_pip" uninstall -y cmake ninja >/dev/null 2>&1 || true
  fi
fi

systemctl enable mariadb php-fpm nginx

echo "System dependencies installed. Start services with: sudo systemctl start mariadb php-fpm nginx"
