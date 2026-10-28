# CloudBin

CloudBin is a multi-service object storage backend written in **Go**, backed by **PostgreSQL**, **Redis**, and **Docker**. It separates authentication, object metadata routing, and raw file storage across independent services and replicates uploaded files across multiple storage nodes using consistent hashing.

---

## Architecture

```
                         ┌────────────────────────────┐
                         │    Client / HTTP Caller    │
                         └─────────────┬──────────────┘
                                       │ :8080
                                       ▼
                         ┌────────────────────────────┐
                         │        api-gateway         │
                         └──────┬──────────────┬──────┘
                /api/v1/auth/*  │              │  /api/v1/objects/*
                                ▼              ▼
                  ┌──────────────────┐    ┌──────────────────┐
                  │   auth-service   │    │    object-api    │
                  │      (:8081)     │    │      (:8082)     │
                  └────────┬─────────┘    └────────┬─────────┘
                           │                       │
             ┌─────────────┴─────┐           ┌─────┴─────────────────────────┐
             ▼                   ▼           ▼               ▼               ▼
    ┌─────────────────┐   ┌────────────┐ ┌────────┐  ┌──────────────┐ ┌──────────────┐
    │  postgres-auth  │   │   Redis    │ │objectdb│  │storage-node-1│ │storage-node-2│ ...
    └─────────────────┘   └────────────┘ └────────┘  └──────────────┘ └──────────────┘
```

### Services
- **`api-gateway` (`:8080`)**: Entrypoint reverse proxy that validates JWTs, applies per-user and per-IP rate limits, and forwards requests to `auth-service` and `object-api`.
- **`auth-service` (`:8081`)**: Handles email OTP registration, login, password reset, API token creation, role-based access control (`user`, `admin`), and JWT revocation via Redis (`auth:revoked:<jti>`).
- **`object-api` (`:8082`)**: Manages object metadata in PostgreSQL, maps object keys to storage nodes using an **FNV-1a consistent hash ring (50 virtual nodes per storage server)**, streams concurrent writes to primary and replica nodes, and manages expiring share links in Redis (`share:link:<token>`).
- **`storage-node` (`storage-node-1..3`, `:8083..8085`)**: Lightweight file servers that store, stream, and delete raw object bytes on disk.
- **`migrations`**: Runs database schema migrations for both `auth_db` and `object_db` (and optional seed users when started with `--profile seed`).

### Replication & Placement Behavior
- **Node Selection**: `object-api` places each file on a primary node and replica node(s) using an FNV-1a hash ring (`NewHashRing(nodes, 50)`).
- **Concurrent Writes with Rollback**: Uploads write to the primary and replica nodes concurrently using goroutines. If any replica write fails, `object-api` rolls back the write on the nodes that succeeded and returns an error so partial writes are not left behind.
- **Read Failover**: Downloads read from the primary node first and automatically fall back to the replica node if the primary is unreachable or returns an error.

---

## Quick Start (Docker Compose)

### Start the Full Stack
```bash
docker compose up --build
```

### Start with Seeded Admin & Demo Users
```bash
docker compose --profile seed up --build
```
Default seeded accounts (configured in `docker-compose.yml`):
- **Admin**: `admin@example.com` / `admin123`
- **User**: `user@example.com` / `user123`

### Stop Containers & Clean Volumes
```bash
docker compose down --volumes --remove-orphans
```

---

## REST API Overview (Gateway `:8080`)

### Authentication & API Tokens
- `POST /api/v1/auth/register` — Register with email and password (sends email OTP)
- `POST /api/v1/auth/register/verify` — Verify registration OTP
- `POST /api/v1/auth/login` — Authenticate and receive a JWT
- `POST /api/v1/auth/forgot-password` — Request a password reset OTP
- `POST /api/v1/auth/forgot-password/verify` — Reset password with OTP
- `GET /api/v1/auth/tokens` — List active API tokens
- `POST /api/v1/auth/tokens` — Create a new API token
- `DELETE /api/v1/auth/tokens/{id}` — Revoke an API token

### Object Storage & Sharing
- `GET /api/v1/objects` — List user objects (supports `limit`, `offset`, `permission`, `visibility`, `key`)
- `POST /api/v1/objects` — Stream-upload an object (auto-generated or header key)
- `PUT /api/v1/objects/{key}` — Upload or overwrite an object at a specific key
- `GET /api/v1/objects/{key}` — Download object bytes
- `HEAD /api/v1/objects/{key}` — Fetch object headers (`Content-Type`, `ETag`, `Content-Length`)
- `GET /api/v1/objects/{key}/exists` — Check if an object exists
- `DELETE /api/v1/objects/{key}` — Delete an object from primary and replica nodes
- `PUT /api/v1/objects/{key}/permission` — Set access (`public-read` or `private-read`)
- `PUT /api/v1/objects/{key}/visibility` — Update object visibility
- `POST /api/v1/objects/{key}/shares` — Create an expiring download share link stored in Redis
- `GET /api/v1/shares/{token}` — Download an object using a share token
- `GET /api/v1/public/objects/{key}?owner_id={id}` — Public download for `public-read` objects

### Admin Routes
- `DELETE /api/v1/admin/objects/{key}?owner_id={id}` — Delete any user's object (requires `admin` role)
- `PUT /api/v1/admin/objects/{key}/visibility?owner_id={id}` — Hide/unhide an object (requires `admin` role)
