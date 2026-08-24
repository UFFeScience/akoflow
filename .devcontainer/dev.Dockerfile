FROM docker:26-cli AS dockercli

FROM golang:1.25-trixie

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
    cmake \
    ninja-build \
    libsimgrid-dev \
    nlohmann-json3-dev \
 && rm -rf /var/lib/apt/lists/*

COPY --from=dockercli /usr/local/bin/docker /usr/local/bin/docker

COPY go.mod go.sum ./
RUN go mod download

RUN go install golang.org/x/tools/gopls@v0.16.2 && \
    go install honnef.co/go/tools/cmd/staticcheck@v0.5.0

RUN go install github.com/go-delve/delve/cmd/dlv@v1.23.1

RUN mkdir -p storage && chmod 777 storage

RUN mkdir -p build/simgrid-runner

EXPOSE 8080
