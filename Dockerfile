FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o server ./cmd/server

FROM alpine:latest

WORKDIR ./site
COPY --from=builder /app/server .
COPY --from=builder /app/public .

CMD ["./server"]