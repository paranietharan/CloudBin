# CloudBin Architecture

This document describes the CloudBin service architecture, request flow, and database design.

CloudBin uses a microservice model with a single public gateway and internal services for authentication and object handling.

## High-Level Architecture

```text
Client
  |
  v
+------------------------+
| API Gateway            |  (public)
| - Auth validation      |
| - Request routing      |
+-----------+------------+
			|
	  +-----+-------------------+
	  |                         |
	  v                         v
+--------------+         +----------------+
| Auth Service |         | Object API      |
| - User auth  |         | - Placement     |
| - OTP flow   |         | - Replication   |
+------+-------+         +--------+--------+
	   |                          |
	   v                          v
+--------------+          +----------------+
| auth_db      |          | object_db       |
| PostgreSQL   |          | PostgreSQL      |
+--------------+          +--------+--------+
									 |
						   +---------+---------+
						   | Storage Nodes     |
						   | node1, node2, ... |
						   +-------------------+
```

## Service Responsibilities

### API Gateway

- Only public-facing entry point.
- Validates JWT and routes requests.
- Forwards auth operations to Auth Service.
- Forwards object operations to Object API.

### Auth Service

- Handles register, login, logout, OTP verification, profile updates, and user deletion.
- Stores and reads identity data only from `auth_db`.

### Object API

- Handles upload, download, and delete file operations.
- Computes primary and replica nodes using modulo hashing.
- Stores object metadata only in `object_db`.
- Reads configured storage node list from environment values.

### Storage Node Service

- Executes file data operations at node level:
  - PUT object
  - GET object
  - DELETE object

## Two PostgreSQL Databases

CloudBin separates persistence into two PostgreSQL databases:

1. `auth_db`: authentication and account-related data
2. `object_db`: object metadata and placement-related data

This keeps domain ownership clear and reduces cross-service coupling.

## Request Flows

### Upload

1. Client sends upload request through API Gateway.
2. Gateway validates JWT via Auth Service contract.
3. Object API calculates placement (`primary`, `replica`) from configured nodes.
4. Object API writes object bytes to both storage nodes in parallel.
5. On success, Object API writes metadata to `object_db`.

### Download

1. Gateway validates JWT.
2. Object API reads metadata from `object_db`.
3. Object API reads from primary node.
4. If primary fails, Object API falls back to replica node.

### Delete

1. Gateway validates JWT.
2. Object API reads metadata from `object_db`.
3. Object API deletes object from both nodes.
4. Object API removes metadata row from `object_db`.

## PostgreSQL Schema

The SQL below is a practical baseline schema for current APIs.

## Auth DB (`auth_db`) Tables

### 1) users

Stores user identity and account status.

```sql
CREATE TABLE users (
	id UUID PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	is_verified BOOLEAN NOT NULL DEFAULT FALSE,
	is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	deleted_at TIMESTAMPTZ
);
```

### 2) user_otps

Stores OTP challenges used for email verification and sensitive actions.

```sql
CREATE TABLE user_otps (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	email TEXT NOT NULL,
	otp_code TEXT NOT NULL,
	purpose TEXT NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	consumed_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_otps_user_id ON user_otps(user_id);
CREATE INDEX idx_user_otps_email ON user_otps(email);
CREATE INDEX idx_user_otps_expires_at ON user_otps(expires_at);
```

### 3) user_sessions

Tracks active JWT sessions and logout invalidation.

```sql
CREATE TABLE user_sessions (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	jti TEXT NOT NULL UNIQUE,
	issued_at TIMESTAMPTZ NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	revoked_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX idx_user_sessions_expires_at ON user_sessions(expires_at);
```

## Object DB (`object_db`) Tables

### 1) objects

Stores logical object records owned by users.

```sql
CREATE TABLE objects (
	id UUID PRIMARY KEY,
	owner_user_id UUID NOT NULL,
	object_key TEXT NOT NULL,
	content_type TEXT,
	size_bytes BIGINT NOT NULL,
	etag TEXT,
	permission TEXT NOT NULL DEFAULT 'private-read',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	deleted_at TIMESTAMPTZ,
	UNIQUE (owner_user_id, object_key)
);

CREATE INDEX idx_objects_owner_user_id ON objects(owner_user_id);
CREATE INDEX idx_objects_object_key ON objects(object_key);
```

### 2) object_placements

Stores primary and replica placement for each object.

```sql
CREATE TABLE object_placements (
	object_id UUID PRIMARY KEY REFERENCES objects(id) ON DELETE CASCADE,
	primary_node_id TEXT NOT NULL,
	replica_node_id TEXT NOT NULL,
	placement_version INT NOT NULL DEFAULT 1,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CHECK (primary_node_id <> replica_node_id)
);

CREATE INDEX idx_object_placements_primary_node ON object_placements(primary_node_id);
CREATE INDEX idx_object_placements_replica_node ON object_placements(replica_node_id);
```

### 3) object_access_audit

Optional but recommended for traceability of reads/writes/deletes.

```sql
CREATE TABLE object_access_audit (
	id UUID PRIMARY KEY,
	object_id UUID REFERENCES objects(id) ON DELETE SET NULL,
	owner_user_id UUID,
	action TEXT NOT NULL,
	status TEXT NOT NULL,
	source_ip TEXT,
	user_agent TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_object_access_audit_object_id ON object_access_audit(object_id);
CREATE INDEX idx_object_access_audit_owner_user_id ON object_access_audit(owner_user_id);
CREATE INDEX idx_object_access_audit_created_at ON object_access_audit(created_at);
```

## Configuration Notes

Recommended environment variables:

```env
AUTH_DB_DSN=postgres://user:pass@auth-postgres:5432/auth_db?sslmode=disable
OBJECT_DB_DSN=postgres://user:pass@object-postgres:5432/object_db?sslmode=disable
STORAGE_NODES=node1:8083,node2:8084,node3:8085
REPLICATION_FACTOR=2
```

## Non-Goals for Current Phase

- No cross-database transactions between `auth_db` and `object_db`.
- No object byte storage in PostgreSQL (bytes stay in storage nodes).
- No automatic placement rebalance when node topology changes.

