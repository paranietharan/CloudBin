# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS builder
WORKDIR /src/migrations

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/migrations .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 appuser
WORKDIR /app/migrations

RUN mkdir -p /app/migrations
COPY --from=builder /out/migrations /app/migrations/migrations
COPY auth /app/migrations/auth
COPY object /app/migrations/object
COPY docker-entrypoint.sh /app/migrations/docker-entrypoint.sh
RUN chmod +x /app/migrations/docker-entrypoint.sh

USER appuser
ENTRYPOINT ["/app/migrations/docker-entrypoint.sh"]