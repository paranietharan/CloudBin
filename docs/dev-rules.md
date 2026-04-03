---
# Dev rules
---

## Core Rules

1. Make important runtime values configurable through `.env` files.
2. Keep code comments minimal and only where they improve clarity.

## Configuration Rules

1. Storage node definitions must be configurable (do not hardcode node list or node count).
2. Placement logic must read node configuration from environment values.
3. Use two PostgreSQL DSNs: one for auth service and one for object service.

## Database Ownership Rules

1. Auth Service owns `auth_db` tables.
2. Object API owns `object_db` tables.
3. Do not directly query another service's database from a different service.
4. Share data across services through APIs/events, not cross-DB access.

## Deployment Rules

1. Maintain an automated deployment script to deploy services to all configured nodes.
2. Keep deployment inputs configuration-driven so topology changes do not require code changes.