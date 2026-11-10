FROM golang:1.25-trixie AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/server ./cmd/server
COPY internal ./internal
RUN go test -race ./internal/hubpublish ./internal/api/httpserver ./internal \
 && go vet ./internal/hubpublish ./internal/api/httpserver \
 && go build -o /output/akoflow-server ./cmd/server

FROM akoflow/daemon:v1.0.15
COPY --from=go-builder /output/akoflow-server /usr/local/bin/akoflow-server
ENV AKOFLOW_SERVER_VERSION=v1.0.15-hub-local
