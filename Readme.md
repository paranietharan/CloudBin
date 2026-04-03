# Cloud Bin

CloudBin is a distributed object storage system inspired by systems like Amazon S3. It is built using a microservices architecture with Go and Docker, designed for learning and demonstrating core concepts of distributed storage systems.

## What Is CloudBin?
 
CloudBin is a learning-focused distributed object storage system. It lets you upload, download, and delete files (objects) through a single API, while internally distributing and replicating those files across multiple storage nodes.
 
The goal is to understand the core engineering concepts behind systems like S3, GCS, or MinIO — routing, hashing, replication, metadata management, and API gateway design.

## Features
 
- **Object storage** — Upload, download, and delete files via REST API
- **Hashing-based placement** — Consistent placement of objects across storage nodes
- **Replication** — Objects written to multiple nodes for redundancy
- **JWT authentication** — Secure access via token-based auth
- **API Gateway** — Single public entry point with routing and auth validation
- **Split persistence model** — Separate PostgreSQL databases for auth and object metadata
- **Configurable storage nodes** — Node list and node count are driven by environment configuration
- **Automated multi-node deployment** — A deployment script can deploy all services across configured nodes
- **Fully containerized** — Runs end-to-end with Docker Compose

## Node Configuration and Deployment

CloudBin is designed so storage node topology is configurable instead of hardcoded.

- Storage node addresses should be provided through environment variables.
- Placement logic should use the configured node list length as the active node count.
- Deployment should be automated through a script that deploys the service stack to all configured nodes.

Example configuration values:

```env
AUTH_DB_DSN=postgres://user:pass@auth-postgres:5432/auth_db?sslmode=disable
OBJECT_DB_DSN=postgres://user:pass@object-postgres:5432/object_db?sslmode=disable
STORAGE_NODES=node1:8083,node2:8084,node3:8085
REPLICATION_FACTOR=2
```

Suggested deployment script location:

```text
scripts/deploy-all-nodes.sh
```

## Endpoints

### Auth endpoints
- api/v1/login
- api/v1/register
- api/v1/logout
- api/v1/get-user
- api/v1/update-user
- api/v1/delete-user
- api/v1/get-user-files

### File Management endpoints
- api/v1/upload-file
- api/v1/download-file
- api/v1/delete-file

--------------------------------
## Architecture at a Glance
--------------------------------
```
Client
  │
  ▼
┌─────────────────────┐
│     API Gateway      │  ◄── Only public-facing service
│  (Auth + Routing)    │
└──────────┬──────────┘
           │
     ┌─────┴──────┐
     │            │
     ▼            ▼
  Auth Svc    Object API
    |            |
    v            v
   Auth DB      Object DB
                  │
        ┌─────────┼─────────┐
        ▼         ▼         ▼
      Node 1    Node 2    Node 3
```

See [docs/architecture.md](docs/architecture.md) for a full breakdown.

--------------------------------
### Register and Upload a File
--------------------------------
 
```bash
# Register a user
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "parani@parani.com", "password": "secret"}'

# Verify the user otp
curl -X Post http://localhost:8080/auth/verify-otp \
  -H "Content-Type: application/json" \
  -d '{"email": "parani@parani.com", "otp": "123456"}'
 
# Login to get a JWT
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "parani@parani.com", "password": "secret"}' | jq -r '.token')

# Update the user details
curl -X PUT http://localhost:8080/auth/update-user \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email": "parani@parani.com", "password": "secret"}'

# Delete the user
curl -X DELETE http://localhost:8080/auth/delete-user \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email": "parani@parani.com", "password": "secret"}'

# Confirm User deletion by verify the otp sent to the email
curl -X Post http://localhost:8080/auth/verify-otp \
  -H "Content-Type: application/json" \
  -d '{"email": "parani@parani.com", "otp": "123456"}'

# List all the files in the user
curl -X GET http://localhost:8080/auth/get-user-files \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" 

# Change the Permissions of the file
curl -X PUT http://localhost:8080/auth/update-file-permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"file_id": "123456", "permissions": "public-read"}'

(permissions can be public-read or private-read)
 
# Upload a file
curl -X PUT http://localhost:8080/objects/my-file.txt \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: text/plain" \
  --data-binary @my-file.txt
 
# Download the file
curl http://localhost:8080/objects/my-file.txt \
  -H "Authorization: Bearer $TOKEN" \
  -o downloaded.txt
 
# Delete the file
curl -X DELETE http://localhost:8080/objects/my-file.txt \
  -H "Authorization: Bearer $TOKEN"
```

## Tech Stack
 
| Layer | Technology |
|---|---|
| Language | Go 1.26 |
| Containerization | Docker, Docker Compose |
| Databases | PostgreSQL (Auth DB + Object DB) |
| Auth | JWT (HS256) |
| Communication | REST / HTTP |

## Services Overview
 
| Service | Internal Port | External Port | Exposed? |
|---|---|---|---|
| API Gateway | `8080` | `8080` | ✅ Yes |
| Auth Service | `8081` | — | ❌ No |
| Object API | `8082` | — | ❌ No |
| Storage Node 1 | `8083` | — | ❌ No |
| Storage Node 2 | `8084` | — | ❌ No |
| Storage Node 3 | `8085` | — | ❌ No |
| PostgreSQL Auth DB | `5432` | — | ❌ No |
| PostgreSQL Object DB | `5432` | — | ❌ No |

# Other docs
- [File Structure](docs/file-structure.md)
- [Dev Rules](docs/dev-rules.md)
- [Data Placement](docs/data-placement.md)
- [Architecture](docs/architecture.md)
