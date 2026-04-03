# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src

# Cache module downloads first (go.sum may not exist yet in this module).
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/storage-node ./cmd

FROM alpine:3.20
RUN adduser -D -H -u 10001 appuser
WORKDIR /app

COPY --from=builder /out/storage-node /app/storage-node

ENV STORAGE_PORT=8083
ENV STORAGE_NODE_ID=node-1
ENV STORAGE_ROOT=/data

RUN mkdir -p /data && chown -R appuser:appuser /data /app
USER appuser

EXPOSE 8083
ENTRYPOINT ["/app/storage-node"]
