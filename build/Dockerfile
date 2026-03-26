FROM golang:1.23-alpine

WORKDIR /app

ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download
COPY . ./

RUN go build -o ./bin/server ./cmd/server/main.go

CMD ["./bin/server"]
