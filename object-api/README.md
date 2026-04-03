# Object API

Object API handles object metadata, placement, replication orchestration, and visibility actions.

## Responsibilities

- Accept object upload/download/delete/hide requests behind API Gateway
- Compute deterministic placement using hash-and-modulo
- Write bytes to primary and replica storage nodes
- Persist metadata in `object_db`

## Required Configuration

| Variable | Required | Description |
|---|---|---|
| OBJECT_API_PORT | Yes | HTTP port to run Object API |
| OBJECT_DB_DSN | Yes | PostgreSQL DSN for object database |
| STORAGE_NODES | Yes | Comma-separated list of storage node base URLs |
| REPLICATION_FACTOR | Yes | Replica count (current implementation expects 2) |
| JWT_SECRET | Yes | Shared JWT signing secret |
| JWT_ISSUER | Yes | Expected JWT issuer |

## Run Locally

```bash
cp .env.example .env
go run ./cmd
```

## Health Check

```bash
curl http://localhost:8082/healthz
```

