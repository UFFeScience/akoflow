FROM golang:1.23-bullseye

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    g++ \
    libc-dev \
    gcc-aarch64-linux-gnu \
    libc6-dev-arm64-cross \
    libsqlite3-dev \
    sqlite3 \
    git \
    curl \
    graphviz \
    ssh \
    sshpass \
    rsync \
    pkg-config \
 && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

RUN go install golang.org/x/tools/gopls@v0.16.2 && \
    go install honnef.co/go/tools/cmd/staticcheck@v0.5.0

RUN CGO_ENABLED=1 go install -ldflags "-s -w -extldflags '-static'" github.com/go-delve/delve/cmd/dlv@latest

RUN mkdir -p storage && chmod 777 storage

EXPOSE 8080