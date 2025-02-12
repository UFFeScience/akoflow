FROM golang:1.22-alpine

WORKDIR /app

RUN apk add --no-cache gcc libc-dev sqlite git curl graphviz
COPY go.mod go.sum ./
RUN go mod download

RUN go install golang.org/x/tools/gopls@v0.16.2 && \
    go install honnef.co/go/tools/cmd/staticcheck@v0.5.0

COPY . .

RUN CGO_ENABLED=1 go install -ldflags "-s -w -extldflags '-static'" github.com/go-delve/delve/cmd/dlv@latest
RUN CGO_ENABLED=1 go build -gcflags "all=-N -l" -o akoflow cmd/server/main.go
RUN mkdir -p storage && chmod 777 storage

EXPOSE 8080
