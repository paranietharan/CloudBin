# Storage Node

Storage Node stores and serves raw object bytes on local disk.

## Endpoints

- `PUT /objects/{key}`
- `GET /objects/{key}`
- `DELETE /objects/{key}`
- `GET /healthz`

## Required Configuration

| Variable | Required | Description |
|---|---|---|
| STORAGE_NODE_ID | Yes | Logical node name used in health output |
| STORAGE_PORT | Yes | HTTP port for this storage node |
| STORAGE_ROOT | Yes | Root directory for object files |

## Run Locally

```bash
cp .env.example .env
go run ./cmd
```

## Health Check

```bash
curl http://localhost:8083/healthz
```

