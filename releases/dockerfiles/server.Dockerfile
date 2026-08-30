ARG BUILDKIT_VERSION=v0.30.0

FROM debian:trixie AS simgrid-builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake \
    g++ \
    libsimgrid-dev \
    ninja-build \
    nlohmann-json3-dev \
    pkg-config \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /build
COPY simgrid-runner ./simgrid-runner
RUN cmake -S simgrid-runner -B output -G Ninja -DCMAKE_BUILD_TYPE=Release \
 && cmake --build output --parallel

FROM golang:1.25-trixie AS go-builder

WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends gcc libsqlite3-dev \
 && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /output/akoflow-server ./cmd/server

FROM moby/buildkit:${BUILDKIT_VERSION} AS buildkit-client

# Apptainer is built from source because Debian trixie does not ship the
# runtime package. The build is architecture-native, so Docker Buildx produces
# matching amd64/arm64 images without copying a host binary.
FROM golang:1.25-trixie AS apptainer-builder
ARG APPTAINER_VERSION=v1.5.3
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential git libseccomp-dev libglib2.0-dev libgpgme-dev \
    libssl-dev libnss3-dev uuid-dev \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN git clone --depth 1 --branch ${APPTAINER_VERSION} https://github.com/apptainer/apptainer.git . \
 && ./mconfig --prefix=/usr/local --with-suid \
 && make -C builddir -j4 \
 && make -C builddir install

FROM debian:trixie-slim

ARG AKOFLOW_VERSION=dev
LABEL org.opencontainers.image.title="AkôFlow Daemon" \
      org.opencontainers.image.description="AkôFlow workflow control plane" \
      org.opencontainers.image.vendor="UFFeScience" \
      org.opencontainers.image.version="${AKOFLOW_VERSION}"

ENV AKOFLOW_SERVER_VERSION=${AKOFLOW_VERSION} \
    AKOFLOW_HTTP_ADDRESS=0.0.0.0:8080 \
    AKOFLOW_DATABASE_PATH=/var/lib/akoflow/database.db \
    AKOFLOW_ARTIFACT_STORE_ROOT=/var/lib/akoflow/artifacts \
    AKOFLOW_SIMGRID_WORKSPACE=/var/lib/akoflow/simgrid \
    AKOFLOW_BUILDCTL=buildctl \
    AKOFLOW_APPTAINER=apptainer \
    AKOFLOW_SIMGRID_BINARY=/usr/local/bin/akoflow-simgrid-runner

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    docker-cli \
    libsimgrid4.0 \
    libsqlite3-0 \
    libseccomp2 \
    libgpgme11 \
    libfuse3-4 \
    openssh-client \
    rsync \
    kubernetes-client \
    squashfs-tools \
    uidmap \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=go-builder /output/akoflow-server /usr/local/bin/akoflow-server
COPY --from=buildkit-client /usr/bin/buildctl /usr/local/bin/buildctl
COPY --from=apptainer-builder /usr/local /usr/local
COPY --from=simgrid-builder /build/output/akoflow-simgrid-runner /usr/local/bin/akoflow-simgrid-runner

RUN mkdir -p /var/lib/akoflow/artifacts \
    /var/lib/akoflow/credentials/ssh \
    /var/lib/akoflow/credentials/kubernetes \
    /var/lib/akoflow/simgrid \
 && apptainer --version
EXPOSE 8080
VOLUME ["/var/lib/akoflow"]
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=12 \
  CMD ["curl", "--fail", "--silent", "http://127.0.0.1:8080/akoflow-api/instance/"]
CMD ["akoflow-server"]
