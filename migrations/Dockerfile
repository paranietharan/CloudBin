# syntax=docker/dockerfile:1

FROM golang:1.27.3-alpine3.24 AS builder
WORKDIR /src/migrations

COPY migrations/go.mod migrations/go.sum ./
COPY shared/go.mod /src/shared/
RUN go mod download

COPY shared /src/shared
COPY migrations/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/migrations .

FROM alpine:3.24
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 appuser
WORKDIR /app/migrations

COPY --from=builder /out/migrations /app/migrations/migrations
COPY migrations/auth /app/migrations/auth
COPY migrations/object /app/migrations/object
COPY migrations/docker-entrypoint.sh /app/migrations/docker-entrypoint.sh
RUN chmod +x /app/migrations/docker-entrypoint.sh

USER appuser
ENTRYPOINT ["/app/migrations/docker-entrypoint.sh"]