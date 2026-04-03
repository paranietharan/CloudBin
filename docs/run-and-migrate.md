# Run and Migrate Guide

This guide explains how to apply database migrations and run each CloudBin service locally.

## Prerequisites

- Go 1.22 or later
- PostgreSQL running for auth database
- A populated `.env` file in each module folder (`api-gateway`, `auth-service`, `object-api`, `storage-node`, `migrations`)

## 1. Configure Environment

Copy the module templates and update values.

```bash
cp api-gateway/.env.example api-gateway/.env
cp auth-service/.env.example auth-service/.env
cp object-api/.env.example object-api/.env
cp storage-node/.env.example storage-node/.env
cp migrations/.env.example migrations/.env
```

Minimum required values:

```env
JWT_SECRET=change-me
JWT_ISSUER=cloudbin-auth
JWT_ACCESS_TTL=24h
GATEWAY_PORT=8080
AUTH_SERVICE_URL=http://localhost:8081
AUTH_PORT=8081
AUTH_DB_DSN=postgres://postgres:postgres@localhost:5432/auth_db?sslmode=disable
```

Object service values:

```env
OBJECT_DB_DSN=postgres://postgres:postgres@localhost:5432/object_db?sslmode=disable
OBJECT_API_URL=http://localhost:8082
OBJECT_API_PORT=8082
STORAGE_NODES=http://localhost:8083,http://localhost:8084,http://localhost:8085
REPLICATION_FACTOR=2

# For each storage-node process
STORAGE_NODE_ID=node-1
STORAGE_PORT=8083
STORAGE_ROOT=./data
```

## 2. Run Migrations

CloudBin uses a Go migration CLI in `migrations/main.go`.

Run from the `migrations` directory:

```bash
cd migrations

# Apply migrations
go run main.go -migrate=up -target=auth
go run main.go -migrate=up -target=object

# Revert migrations
go run main.go -migrate=down -target=auth
go run main.go -migrate=down -target=object
```

Behavior:

- Uses `golang-migrate` with `up/down` SQL files.
- Supports target selection with `-target=auth|object`.
- Reads DSNs from `migrations/.env` (`AUTH_DB_DSN` or `OBJECT_DB_DSN`).

Current state:

- `migrations/auth` supports up/down.
- `migrations/object` supports up/down.

## 3. Run Seed

Run from the `migrations` directory:

```bash
cd migrations
go run main.go -seed -target=auth
```

Required seed env values:

```env
ADMIN_INITIAL_PASSWORD=admin123
USER_INITIAL_PASSWORD=user123
```

Optional seed env values:

```env
ADMIN_EMAIL=admin@example.com
USER_EMAIL=user@example.com
```

## 4. Run Auth Service

In terminal 1:

```bash
cd auth-service
go run ./cmd
```

Default port is `AUTH_PORT` (usually `8081`).

## 5. Run API Gateway

In terminal 2:

```bash
cd api-gateway
go run ./cmd
```

Default port is `GATEWAY_PORT` (usually `8080`).

## 6. Run Object API

In terminal 3:

```bash
cd object-api
go run ./cmd
```

Default port is `OBJECT_API_PORT` (usually `8082`).

## 7. Run Storage Nodes

Run one process per node (example for 3 local nodes):

```bash
cd storage-node
STORAGE_NODE_ID=node-1 STORAGE_PORT=8083 STORAGE_ROOT=./data/node-1 go run ./cmd
STORAGE_NODE_ID=node-2 STORAGE_PORT=8084 STORAGE_ROOT=./data/node-2 go run ./cmd
STORAGE_NODE_ID=node-3 STORAGE_PORT=8085 STORAGE_ROOT=./data/node-3 go run ./cmd
```

## 8. Quick Health Check

```bash
curl http://localhost:8081/healthz
curl http://localhost:8080/healthz
curl http://localhost:8082/healthz
curl http://localhost:8083/healthz
```

## 9. Typical Dev Loop

1. Update SQL in `migrations/auth`.
2. Run `cd migrations && go run main.go -migrate=up -target=auth`.
3. Restart affected service if needed.
4. Test endpoints through API Gateway (`:8080`).
