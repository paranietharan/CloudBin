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
- **Role-based access control** — Admins can deactivate/delete users and manage resources
- **Resource visibility** — Owners and admins can hide/delete resources; hidden items stay visible only to the owner
- **Multi-token support** — Users can create multiple JWT tokens for different service integrations
- **Configurable storage nodes** — Node list and node count are driven by environment configuration
- **Automated multi-node deployment** — A deployment script can deploy all services across configured nodes
- **Fully containerized** — Runs end-to-end with Docker Compose

## CI Command

Use this command in CI/local checks:

```bash
go test ./...
```

## What I Improved

- Added service orchestration scripts for local backend startup, health checks, and shutdown.
- Added upload key auto-generation and consistent object key response payloads.
- Added object permission toggles (`private-read` and `public-read`) and public download behavior.
- Added temporary share link endpoint with expiry.
- Added pagination and filtering for user file listing.
- Added fixed-window in-memory rate limiting for sensitive object endpoints.
- Added request-id based structured logging in API gateway.
- Added graceful shutdown support for gateway, auth-service, and object-api.

## Known Limitations / Next Steps

- Temporary share links are currently in-memory only (reset on service restart).
- Rate limiting is in-memory and not distributed across multiple instances.
- Integration script assumes services are already running and requires `jq`.
- Dedicated e2e test environment with seeded data is still pending.


# Other docs
- [How to Run](docs/how-to-run.md)
- [Tech Overview](docs/tech.md)
- [Architecture](docs/architecture.md)
- [Data Placement](docs/data-placement.md)
- [Database Setup](docs/db-setup.md)
- [Dev Rules](docs/dev-rules.md)
- [Run the Application & Migrations](docs/run-and-migrate.md)
- [File Structure](docs/file-structure.md)