FROM golang:1.22-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y \
    gcc \
    libc6-dev \
    sqlite3 \
    git \
    curl \
    graphviz \
    tar \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

RUN go install golang.org/x/tools/gopls@v0.16.2 && \
    go install honnef.co/go/tools/cmd/staticcheck@v0.5.0 && \
    go install github.com/go-delve/delve/cmd/dlv@latest

COPY . .
RUN CGO_ENABLED=1 go build -gcflags "all=-N -l" -o akoflow cmd/server/main.go

RUN mkdir -p storage && chmod 777 storage

RUN curl -L https://github.com/rqlite/rqlite/releases/download/v8.36.16/rqlite-v8.36.16-linux-amd64.tar.gz -o rqlite.tar.gz && \
    tar -xzf rqlite.tar.gz && \
    mv rqlite-v8.36.16-linux-amd64/rqlited /usr/local/bin/rqlited && \
    rm -rf rqlite*

EXPOSE 8080

RUN mkdir -p /app/data
