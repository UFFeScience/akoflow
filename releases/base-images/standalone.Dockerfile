FROM node:20-bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

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
