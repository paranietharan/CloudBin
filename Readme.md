# Cloud Bin

CloudBin is a distributed object storage system inspired by systems like Amazon S3. It is built using a microservices architecture with Go and Docker, designed for learning and demonstrating core concepts of distributed storage systems.

## Docker Quick Start

```bash
docker compose up --build
```

## Completely run docker
```bash
docker compose --profile seed up --build
```

## Stop the docker
```bash
docker compose down --volumes --remove-orphans
```

```bash
docker compose down --volumes --remove-orphans --rmi all
```

## Important Notes
- Primary/replica selection: `placement()` uses FNV hash; replica = (primary+1)%N.
- Replication is synchronous: write failure triggers rollback and upload fails.
- Reads fall back to replica; no background repair/resync implemented.
- Metadata and bytes are coupled: DB tracks placement; nodes store raw bytes.
- Auth enforced at gateway and service levels via JWT.
- See `migrations` and `docker-compose.yml` for migration and deployment details.

## API Overview

CloudBin exposes an industry-standard RESTful API through the API Gateway (port 8080):

### Authentication & Tokens
- `POST /api/v1/auth/register` - Register with email/password (requests OTP)
- `POST /api/v1/auth/register/verify` - Verify registration OTP
- `POST /api/v1/auth/login` - Login and obtain JWT
- `POST /api/v1/auth/forgot-password` - Request password reset OTP
- `POST /api/v1/auth/forgot-password/verify` - Reset password with OTP
- `GET /api/v1/auth/tokens` - List active API tokens
- `POST /api/v1/auth/tokens` - Create a new API token
- `DELETE /api/v1/auth/tokens/{id}` - Revoke an API token

### Objects & Storage
- `GET /api/v1/objects` - List user objects (supports `limit`, `offset`, `permission`, `visibility`, `key`)
- `POST /api/v1/objects` - Stream upload an object (auto-generated or header key)
- `PUT /api/v1/objects/{key}` - Upload or replace object with explicit key
- `GET /api/v1/objects/{key}` - Download object content
- `HEAD /api/v1/objects/{key}` - Fetch object metadata (Content-Type, ETag, Content-Length)
- `GET /api/v1/objects/{key}/exists` - Check if an object exists
- `DELETE /api/v1/objects/{key}` - Delete an object
- `PUT /api/v1/objects/{key}/permission` - Update permission (`{"permission": "public-read"|"private-read"}`)
- `PUT /api/v1/objects/{key}/visibility` - Update visibility / hide object
- `POST /api/v1/objects/{key}/shares` - Generate expiring download share link
- `GET /api/v1/shares/{token}` - Public download via share token
- `GET /api/v1/public/objects/{key}?owner_id={id}` - Public download for `public-read` objects
- `DELETE /api/v1/admin/objects/{key}?owner_id={id}` - Admin delete
- `PUT /api/v1/admin/objects/{key}/visibility?owner_id={id}` - Admin hide