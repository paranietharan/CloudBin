# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src/object-api

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/object-api ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 appuser
WORKDIR /app

RUN mkdir -p /app/object-api
COPY --from=builder /out/object-api /app/object-api/object-api

ENV OBJECT_API_PORT=8082

EXPOSE 8082
USER appuser
ENTRYPOINT ["/app/object-api/object-api"]