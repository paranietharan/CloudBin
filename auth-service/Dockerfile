# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src/auth-service

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/auth-service ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 appuser
WORKDIR /app

RUN mkdir -p /app/auth-service
COPY --from=builder /out/auth-service /app/auth-service/auth-service

ENV AUTH_PORT=8081

EXPOSE 8081
USER appuser
ENTRYPOINT ["/app/auth-service/auth-service"]