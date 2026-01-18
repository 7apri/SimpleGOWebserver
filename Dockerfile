FROM golang:1.25.5-alpine AS builder

WORKDIR /app
COPY . .

RUN go mod download
RUN go build -o server .

FROM alpine:latest

WORKDIR ./site
COPY --from=builder /app/server .
COPY --from=builder /app/public .

CMD ["./server"]