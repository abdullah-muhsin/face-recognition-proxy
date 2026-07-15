FROM node:24-bookworm-slim AS frontend

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

COPY resources ./resources
COPY vite.config.js ./
RUN npm run build

FROM composer:2 AS vendor

WORKDIR /app

COPY composer.json composer.lock ./
RUN composer install \
    --no-dev \
    --no-autoloader \
    --no-interaction \
    --no-progress \
    --no-scripts \
    --prefer-dist

COPY . .
RUN composer dump-autoload \
    --classmap-authoritative \
    --no-dev \
    --no-interaction

FROM php:8.4-apache-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libonig-dev \
        libsqlite3-dev \
    && docker-php-ext-install \
        mbstring \
        opcache \
        pdo_sqlite \
    && a2enmod headers rewrite \
    && rm -rf /var/lib/apt/lists/*

RUN echo "ServerName localhost" > /etc/apache2/conf-available/servername.conf \
    && a2enconf servername

ENV APP_ENV=production \
    APP_DEBUG=false \
    CACHE_STORE=file \
    DB_CONNECTION=sqlite \
    DB_DATABASE=/var/lib/attendance-receiver/database.sqlite \
    FILESYSTEM_DISK=local \
    LOG_CHANNEL=stack \
    QUEUE_CONNECTION=sync \
    SESSION_DRIVER=file \
    SESSION_PATH=/

WORKDIR /var/www/html

COPY docker/apache/000-default.conf /etc/apache2/sites-available/000-default.conf
COPY docker/entrypoint.sh /usr/local/bin/attendance-receiver-entrypoint
COPY --from=vendor /app /var/www/html
COPY --from=frontend /app/public/build /var/www/html/public/build

RUN chmod +x /usr/local/bin/attendance-receiver-entrypoint \
    && mkdir -p \
        /var/lib/attendance-receiver \
        storage/app/private \
        storage/app/public \
        storage/framework/cache/data \
        storage/framework/sessions \
        storage/framework/testing \
        storage/framework/views \
        storage/logs \
    && chown -R www-data:www-data \
        /var/lib/attendance-receiver \
        storage \
        bootstrap/cache

VOLUME ["/var/lib/attendance-receiver", "/var/www/html/storage"]

ENTRYPOINT ["attendance-receiver-entrypoint"]
CMD ["apache2-foreground"]
