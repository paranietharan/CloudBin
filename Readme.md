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