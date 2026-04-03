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

## Access Control Rules

1. Admin can deactivate or delete users.
2. Owner and admin can hide or delete resources.
3. Hidden resources remain visible only to the owner and admins.
4. Owners can create multiple JWT tokens for different service integrations.
5. Owners can list and delete their own tokens; admins can revoke tokens when needed.
6. Token creation must require an authenticated login session first.

## Database Ownership Rules

1. Auth Service owns `auth_db` tables.
2. Object API owns `object_db` tables.
3. Do not directly query another service's database from a different service.
4. Share data across services through APIs/events, not cross-DB access.

## Deployment Rules

1. Maintain an automated deployment script to deploy services to all configured nodes.
2. Keep deployment inputs configuration-driven so topology changes do not require code changes.