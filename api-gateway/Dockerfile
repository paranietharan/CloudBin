# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS builder
WORKDIR /src/api-gateway

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api-gateway ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 appuser
WORKDIR /app

RUN mkdir -p /app/api-gateway
COPY --from=builder /out/api-gateway /app/api-gateway/api-gateway

ENV GATEWAY_PORT=8080

EXPOSE 8080
USER appuser
ENTRYPOINT ["/app/api-gateway/api-gateway"]