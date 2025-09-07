#!/bin/bash

set -e

IMAGE_NAME="akoflow-installer"
PORT=8080
AKOSPACE="${HOME}/akospace"
ENV_FILE="${AKOSPACE}/.env"

# Verifica se o Docker está instalado
echo "🔍 Checking Docker..."
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker and try again. To get started, visit https://docs.docker.com/get-docker/"
    exit 1
fi

# Verifica se a porta 8080 está em uso
if lsof -i :$PORT &> /dev/null; then
    echo "❌ Port $PORT is already in use. Please stop the process using it and try again."
    echo ""
    lsof -i :$PORT
    exit 1
fi

mkdir -p "$AKOSPACE"

if [ ! -f "$AKOSPACE/database.db" ]; then
    echo "🔧 Creating empty database file at $AKOSPACE/database.db"
    touch "$AKOSPACE/database.db"
fi


if [ ! -f "$ENV_FILE" ]; then
    echo "🔧 Creating default .env at $ENV_FILE"
    cat <<EOF > "$ENV_FILE"
AKOFLOW_ENV=dev
AKOFLOW_PORT=$PORT
EOF
fi

TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

echo "📄 Generating Dockerfile..."
cat <<'EOF' > Dockerfile
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    unzip \
    sqlite3 \
    ssh \
    sshpass \
    rsync \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN set -eux; \
    TAG=$(curl -s https://api.github.com/repos/UFFeScience/akoflow/releases/latest | grep tag_name | cut -d '"' -f 4 | sed 's/^v//'); \
    echo "AKOFLOW_VERSION=$TAG" >> /etc/environment; \
    echo "Using Tag v$TAG"; \
    ARCH=$(uname -m); \
    if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then \
      BARCH="arm64"; \
    else \
      BARCH="amd64"; \
    fi; \
    curl -fsSL -o /usr/local/bin/akoflow-server "https://github.com/UFFeScience/akoflow/releases/download/v$TAG/akoflow-server_${TAG}_linux_${BARCH}"; \
    curl -fsSL -o /usr/local/bin/akoflow "https://github.com/UFFeScience/akoflow/releases/download/v$TAG/akoflow-client_${TAG}_linux_${BARCH}"; \
    chmod +x /usr/local/bin/akoflow-server /usr/local/bin/akoflow; \
    curl -fsSL -o source.zip "https://github.com/UFFeScience/akoflow/archive/refs/tags/v$TAG.zip"; \
    unzip -qq source.zip "akoflow-$TAG/pkg/server/engine/httpserver/handlers/akoflow_admin_handler/*"; \
    mkdir -p /app/pkg/server/engine/httpserver/handlers; \
    mv "akoflow-$TAG/pkg/server/engine/httpserver/handlers/akoflow_admin_handler" /app/pkg/server/engine/httpserver/handlers/; \
    rm -rf "akoflow-$TAG" source.zip; \
    echo "$TAG" > /app/AKOFLOW_VERSION

EXPOSE 8080

ENTRYPOINT ["/bin/sh", "-c", "echo 🔧 Running AkôFlow version: $(cat /app/AKOFLOW_VERSION) && exec akoflow-server"]
EOF

echo "🐳 Building Docker image..."
docker build -t $IMAGE_NAME . --no-cache

echo "🚀 Running container on port $PORT and mounting $AKOSPACE"
docker run --rm \
  -p $PORT:8080 \
  -v "$HOME/akospace/.env:/app/.env" \
  -v "$HOME/akospace/ako.log:/app/ako.log" \
  -v "$HOME/akospace/database.db:/storage/database.db" \
  $IMAGE_NAME