
## Docker Deployment (3 Nodes)

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