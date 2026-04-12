
## Docker Deployment (3 Nodes)

## Local Backend Scripts

- `scripts/run-services.sh` starts `auth-service`, `object-api`, and `api-gateway` in the background.
- `scripts/stop-services.sh` stops those services using PID files.
- `scripts/health-check.sh` checks health endpoints for gateway, services, and storage nodes.

### Start Services

```bash
./scripts/run-services.sh
```

Logs are written to `.logs/` and pids are stored in `.pids/`.

### Health Check

```bash
./scripts/health-check.sh
```

### Stop Services

```bash
./scripts/stop-services.sh
```

### Integration Main Flow

Runs a practical end-to-end flow using an existing verified user:

```bash
EMAIL="<email>" PASSWORD="<password>" FILE_PATH="/absolute/path/to/file.bin" ./scripts/integration-main-flow.sh
```

This script checks:

- login
- upload
- list files
- make public
- public download
- make private

CloudBin includes an automated deployment script for storage nodes:

- `scripts/deploy-storage-nodes.sh`

It will:

- Build the `cloudbin-storage-node:latest` image from `storage-node/Dockerfile`
- Create Docker network `cloudbin-net` (if missing)
- Create local data directories under `storage-data/node1..node3`
- Start 3 storage-node containers on ports `8083`, `8084`, and `8085`

### Prerequisites

- Docker Desktop (or compatible Docker daemon) is running

### Deploy

From repository root:

```bash
chmod +x scripts/deploy-storage-nodes.sh
./scripts/deploy-storage-nodes.sh
```

### Verify

```bash
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep cloudbin-storage-node

curl http://localhost:8083/healthz
curl http://localhost:8084/healthz
curl http://localhost:8085/healthz
```

Expected response shape:

```json
{"node_id":"node-1","status":"ok"}
```

(`node-2` and `node-3` for the other ports)

### Redeploy / Restart

Run the same script again. It removes and recreates all 3 storage-node containers.

```bash
./scripts/deploy-storage-nodes.sh
```

# Remove the nodes from docker

```bash
./scripts/remove-storage-nodes.sh --image --network --data
```