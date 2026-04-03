---
# File Structure for CloudBin
---

This document outlines the file structure and organization of the CloudBin project, including the purpose of each directory and key files.

```
Cloudbin/
├── api-gateway/
├── auth-service/
├── object-api/
├── storage-node/
├── scripts/
├── docker/postgres-auth/
├── docker/postgres-object/
├── internal/ (shared libraries if needed)
├── docker/
├── docker-compose.yml
├── docs/architecture.md
```
Example script under `scripts/`:

- deploy-all-nodes.sh

## Explanation for Each Directory

| Directory | Explanation |
|---|---|
| api-gateway | This is the one which exposed to the internet |
| auth-service | Handles authentication, admin user control, and JWT token lifecycle |
| object-api | Communicates with the storage nodes, computes placement from configured node list, and manages resource visibility |
| storage-node | Provides file operations on each node (GET, DELETE, UPLOAD) |
| scripts | Contains operational automation scripts such as multi-node deployment |
| docker/postgres-auth | Docker assets for Auth Service PostgreSQL database |
| docker/postgres-object | Docker assets for Object API PostgreSQL database |
| internal | Shared libraries if needed |
| docker | Docker-related assets |
| docker-compose.yml | Local multi-service container orchestration |
| docs/architecture.md | Full architecture details, PostgreSQL table definitions, and access-control rules |