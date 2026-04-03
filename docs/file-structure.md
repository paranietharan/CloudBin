---
# File Structure for CloudBin
---

Cloudbin/
├── api-gateway/
├── auth-service/
├── object-api/
├── storage-node/
├── scripts/
├── internal/ (shared libraries if needed)
├── docker/
├── docker-compose.yml

Example script under `scripts/`:

- deploy-all-nodes.sh

## Explanation for Each Directory

| Directory | Explanation |
|---|---|
| api-gateway | This is the one which exposed to the internet |
| auth-service | Handles authentication |
| object-api | Communicates with the storage nodes and computes placement from configured node list |
| storage-node | Provides file operations on each node (GET, DELETE, UPLOAD) |
| scripts | Contains operational automation scripts such as multi-node deployment |
| internal | Shared libraries if needed |
| docker | Docker-related assets |
| docker-compose.yml | Local multi-service container orchestration |