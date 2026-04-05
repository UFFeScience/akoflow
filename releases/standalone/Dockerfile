# syntax=docker/dockerfile:1.7

ARG BACKEND_REPO=https://github.com/UFFeScience/akoflow-deployment-control-plane.git
ARG BACKEND_REF=main
ARG FRONTEND_REPO=https://github.com/UFFeScience/akoflow-deployment-control-plane-ui.git
ARG FRONTEND_REF=main
ARG NODE_VERSION=20
ARG TARGETARCH

FROM composer:2 AS backend-builder

ARG BACKEND_REPO
ARG BACKEND_REF

WORKDIR /src/backend

RUN apk add --no-cache \
    git \
    ca-certificates \
    unzip

RUN git clone --depth 1 --branch "${BACKEND_REF}" "${BACKEND_REPO}" . 

RUN composer install --no-interaction --no-dev --prefer-dist --optimize-autoloader

FROM node:${NODE_VERSION}-bookworm-slim AS frontend-builder

ARG FRONTEND_REPO
ARG FRONTEND_REF

ENV NEXT_PUBLIC_API_URL=/api

WORKDIR /src/frontend

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN corepack enable

RUN git clone --depth 1 --branch "${FRONTEND_REF}" "${FRONTEND_REPO}" .

RUN npm ci
RUN npm run build
RUN npm prune --omit=dev
RUN rm -rf .git .next/cache

FROM node:${NODE_VERSION}-bookworm-slim AS runtime

ARG TARGETARCH

ENV NODE_ENV=production \
    APP_ENV=production \
    HOME=/tmp \
    DEBIAN_FRONTEND=noninteractive

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    gnupg \
    nginx \
    openssh-client \
    sshpass \
    python3 \
    python3-apt \
    python3-yaml \
    supervisor \
    unzip \
    && rm -rf /var/lib/apt/lists/*

RUN apt-get update && apt-get install -y --no-install-recommends ansible \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://packages.sury.org/php/apt.gpg | gpg --dearmor -o /usr/share/keyrings/sury-php.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/sury-php.gpg] https://packages.sury.org/php/ bookworm main" > /etc/apt/sources.list.d/sury-php.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
    php8.4-cli \
    php8.4-common \
    php8.4-fpm \
    php8.4-curl \
    php8.4-mbstring \
    php8.4-xml \
    php8.4-zip \
    php8.4-sqlite3 \
    php8.4-bcmath \
    php8.4-intl \
    php8.4-opcache \
    && rm -rf /var/lib/apt/lists/*

RUN TERRAFORM_VERSION=1.9.5 \
    && case "$(uname -m)" in \
        aarch64|arm64) TERRAFORM_ARCH=arm64 ;; \
        x86_64|amd64) TERRAFORM_ARCH=amd64 ;; \
        *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;; \
    esac \
    && curl -fsSL "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_${TERRAFORM_ARCH}.zip" -o /tmp/terraform.zip \
    && unzip /tmp/terraform.zip -d /usr/local/bin \
    && rm /tmp/terraform.zip \
    && terraform version

RUN rm -f /etc/nginx/sites-enabled/default /etc/nginx/sites-available/default

COPY --from=backend-builder /src/backend /app/backend
COPY --from=frontend-builder /src/frontend/package.json /app/frontend/package.json
COPY --from=frontend-builder /src/frontend/package-lock.json /app/frontend/package-lock.json
COPY --from=frontend-builder /src/frontend/next.config.mjs /app/frontend/next.config.mjs
COPY --from=frontend-builder /src/frontend/public /app/frontend/public
COPY --from=frontend-builder /src/frontend/.next /app/frontend/.next
COPY --from=frontend-builder /src/frontend/node_modules /app/frontend/node_modules
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY docker/php-fpm-pool.conf /etc/php/8.4/fpm/pool.d/www.conf
COPY docker/supervisord.conf /etc/supervisor/conf.d/akoflow.conf
COPY docker/entrypoint.sh /usr/local/bin/akoflow-entrypoint

RUN chmod +x /usr/local/bin/akoflow-entrypoint \
    && mkdir -p /app/backend/database /app/backend/storage /app/backend/bootstrap/cache /app/backend/storage/app/ansible/tmp /var/log/supervisor /run/nginx /run/php \
    && chown -R www-data:www-data /app/backend/database /app/backend/storage /app/backend/bootstrap/cache \
    && rm -rf /app/backend/.git /app/backend/tests /app/backend/.github /app/backend/README.md /app/backend/Dockerfile /app/backend/Dockerfile.deploy /app/backend/docker-compose.yml /app/backend/Makefile /app/backend/package.json /app/backend/package-lock.json /app/backend/vite.config.js /app/backend/node_modules

EXPOSE 80

ENTRYPOINT ["/usr/local/bin/akoflow-entrypoint"]