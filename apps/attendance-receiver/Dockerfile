FROM node:24-bookworm-slim@sha256:ba849c60be29959425b8734d57b8b4b7d56f98edd9504c9af091d5281095a71e AS frontend

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

COPY resources ./resources
COPY vite.config.js ./
RUN npm run build

FROM composer:2@sha256:4d71c3c2109c61d5415544264b59ad4087e4c5b7244481723664138fd36d5040 AS vendor

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

FROM composer:2@sha256:4d71c3c2109c61d5415544264b59ad4087e4c5b7244481723664138fd36d5040 AS vendor-dev

WORKDIR /app

COPY composer.json composer.lock ./
RUN composer install \
    --no-interaction \
    --no-progress \
    --no-scripts \
    --prefer-dist

COPY . .
RUN composer dump-autoload \
    --classmap-authoritative \
    --no-interaction

FROM php:8.4-apache-bookworm@sha256:25d70665acee86d7231af7bc5464794abd14585f80210f85f22dfb0713ac8ec7 AS runtime

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

FROM runtime AS test

COPY --from=vendor-dev /app/vendor /var/www/html/vendor

FROM runtime AS production
