#!/usr/bin/env bash
set -euo pipefail

BACKEND_DIR=/app/backend
DB_FILE="$BACKEND_DIR/database/database.sqlite"

mkdir -p "$BACKEND_DIR/database" "$BACKEND_DIR/storage" "$BACKEND_DIR/bootstrap/cache" /var/log/supervisor /run/nginx /run/php
touch "$DB_FILE"
chown -R www-data:www-data "$BACKEND_DIR/database" "$BACKEND_DIR/storage" "$BACKEND_DIR/bootstrap/cache"
chmod 664 "$DB_FILE"

if [ ! -f "$BACKEND_DIR/.env" ]; then
    cp "$BACKEND_DIR/.env.example" "$BACKEND_DIR/.env"
fi

if [ -z "$(sed -n 's/^APP_KEY=//p' "$BACKEND_DIR/.env" | head -n1)" ]; then
    (cd "$BACKEND_DIR" && php artisan key:generate --force --no-interaction)
fi

(cd "$BACKEND_DIR" && php artisan migrate --force --no-interaction)
(cd "$BACKEND_DIR" && php artisan db:seed --force --no-interaction)
(cd "$BACKEND_DIR" && php artisan storage:link --no-interaction) || true

mkdir -p "$BACKEND_DIR/storage/app/terraform/health-check" "$BACKEND_DIR/storage/app/ansible/tmp" "$BACKEND_DIR/storage/framework/cache/data" "$BACKEND_DIR/storage/framework/sessions" "$BACKEND_DIR/storage/framework/views" "$BACKEND_DIR/storage/logs"
chown -R www-data:www-data "$BACKEND_DIR/database" "$BACKEND_DIR/storage" "$BACKEND_DIR/bootstrap/cache"
chmod -R ug+rwX "$BACKEND_DIR/storage" "$BACKEND_DIR/bootstrap/cache"

exec /usr/bin/supervisord -n -c /etc/supervisor/supervisord.conf