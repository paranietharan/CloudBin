---
# Dev rules
---

## Core Rules

1. Make important runtime values configurable through `.env` files.
2. Keep code comments minimal and only where they improve clarity.

## Configuration Rules

1. Storage node definitions must be configurable (do not hardcode node list or node count).
2. Placement logic must read node configuration from environment values.

## Deployment Rules

1. Maintain an automated deployment script to deploy services to all configured nodes.
2. Keep deployment inputs configuration-driven so topology changes do not require code changes.