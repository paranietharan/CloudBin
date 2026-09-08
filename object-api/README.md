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

## Endpoints

### Canonical RESTful Endpoints
- `GET /api/v1/objects` - List owner's objects (`limit`, `offset`, `permission`, `visibility`, `key`)
- `POST /api/v1/objects` - Upload object (auto-generated key or `X-Object-Key` / query `?key=`)
- `PUT /api/v1/objects/{key}` - Upload/replace object with explicit key in URI
- `GET /api/v1/objects/{key}` - Download object binary stream
- `HEAD /api/v1/objects/{key}` - Get object metadata headers (Content-Type, Content-Length, ETag)
- `GET /api/v1/objects/{key}/exists` - Check if object exists (returns JSON `{ "exists": bool }`)
- `DELETE /api/v1/objects/{key}` - Delete object
- `PUT|PATCH /api/v1/objects/{key}/permission` - Update permission (`{"permission": "public-read"|"private-read"}`)
- `PUT|PATCH /api/v1/objects/{key}/visibility` - Update visibility / hide object
- `POST /api/v1/objects/{key}/shares` - Generate temporary expiring share link
- `GET /api/v1/shares/{token}` - Download object via temporary share token (public)
- `GET /api/v1/public/objects/{key}?owner_id={id}` - Download public object (public)
- `DELETE /api/v1/admin/objects/{key}?owner_id={id}` - Admin force delete object
- `PUT|PATCH /api/v1/admin/objects/{key}/visibility?owner_id={id}` - Admin force hide object

### Direct S3-Style & Legacy Aliases
- Direct: `PUT|GET|DELETE /objects/{key}`
- Legacy: `/api/v1/upload-file`, `/api/v1/download-file`, `/api/v1/delete-file`, `/api/v1/hide-file`, `/api/v1/get-user-files`, `/api/v1/file-exists`, `/api/v1/make-private-read`, `/api/v1/make-public-read`, `/api/v1/share-link`, `/api/v1/share/download`, `/api/v1/public/download-file`


