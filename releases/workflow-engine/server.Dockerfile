FROM golang:1.23-bookworm AS builder

WORKDIR /build

ARG TARGETOS=linux
ARG TARGETARCH

RUN apt-get update && apt-get install -y --no-install-recommends \
	gcc \
	libc-dev \
	libsqlite3-dev \
	pkg-config \
	git \
	&& rm -rf /var/lib/apt/lists/*

RUN git clone https://github.com/UFFeScience/akoflow-workflow-engine.git .

RUN git checkout main

RUN CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o akoflow-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o akoflow ./cmd/client

FROM akoflow/base-workflow-engine:latest

WORKDIR /app

COPY --from=builder /build/akoflow-server /usr/local/bin/akoflow-server
COPY --from=builder /build/akoflow /usr/local/bin/akoflow

COPY --from=builder /build/pkg/server/engine/httpserver/handlers/akoflow_admin_handler /app/pkg/server/engine/httpserver/handlers/akoflow_admin_handler
COPY --from=builder /build/pkg/server/scripts /app/pkg/server/scripts

RUN echo "main" > /app/AKOFLOW_VERSION

EXPOSE 8080

ENTRYPOINT ["/bin/sh", "-c", "echo Running AkôFlow version: $(cat /app/AKOFLOW_VERSION) && exec akoflow-server"]
