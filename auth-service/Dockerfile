# syntax=docker/dockerfile:1

FROM golang:1.27.3-alpine3.24 AS builder
WORKDIR /src/auth-service

COPY auth-service/go.mod auth-service/go.sum ./
COPY shared/go.mod /src/shared/
RUN go mod download

COPY shared /src/shared
COPY auth-service/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/auth-service ./cmd

FROM alpine:3.24
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 appuser
WORKDIR /app

RUN mkdir -p /app/auth-service
COPY --from=builder /out/auth-service /app/auth-service/auth-service

ENV AUTH_PORT=8081

EXPOSE 8081
USER appuser
ENTRYPOINT ["/app/auth-service/auth-service"]