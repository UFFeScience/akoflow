FROM golang:1.23-alpine

WORKDIR /app

RUN apk add --no-cache gcc libc-dev sqlite
COPY go.mod ./
RUN go mod download

COPY . .

RUN  go build -o server cmd/server/main.go

CMD [ "./server" ]
