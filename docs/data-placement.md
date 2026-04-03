# Data Placement

This document explains how CloudBin decides which storage node(s) to use for each object, and how replication is handled.

CloudBin uses two PostgreSQL databases:

- `auth_db` for authentication and user lifecycle data
- `object_db` for object metadata and placement data

This document focuses on `object_db` usage.

Resource visibility changes, such as hide or unhide, do not change placement. They only update metadata in `object_db`.

---

## The Problem

When a client uploads an object, the system must decide:

1. **Which node stores the primary copy?**
2. **Which node stores the replica?**

This decision must be deterministic — given the same key and the same cluster state, the system must always reach the same answer. This is what makes retrieval and deletion possible without a full index scan.

---

## Placement via Modulo Hashing

CloudBin uses a simple **hash-and-modulo** strategy.

The node set is **configurable**. The Object API reads storage node definitions from environment variables, then computes placement based on the active configured list.

Example:

```env
STORAGE_NODES=node1:8083,node2:8084,node3:8085
```

`numberOfNodes` is derived from the parsed `STORAGE_NODES` list.

### Step 1: Hash the Key

The object key (e.g. `"reports/q3.pdf"`) is hashed using FNV-1a or a similar non-cryptographic hash function that produces a consistent integer.

```go
func hashKey(key string) uint64 {
    h := fnv.New64a()
    h.Write([]byte(key))
    return h.Sum64()
}
```

### Step 2: Map to a Node

The hash is taken modulo the number of configured storage nodes.

```
primaryNodeIndex = hash(objectKey) % numberOfNodes
```

For example, with 3 configured nodes and a hash of `18374687`:

```
primaryNodeIndex = 18374687 % 3 = 2  →  Node 3
```

### Step 3: Assign the Replica

The replica is placed on the next node in the ring (wrapping around at the end).

```
replicaNodeIndex = (primaryNodeIndex + 1) % numberOfNodes
```

Continuing the example:

```
replicaNodeIndex = (2 + 1) % 3 = 0  →  Node 1
```

So the object is stored on **Node 3** (primary) and **Node 1** (replica).

---

## Worked Example

Assume 3 nodes: `[node1, node2, node3]`

| Object Key | Hash | Primary Index | Replica Index |
|---|---|---|---|
| `photos/cat.jpg` | `9823764` | `9823764 % 3 = 1` → node2 | `2` → node3 |
| `reports/q3.pdf` | `18374687` | `18374687 % 3 = 2` → node3 | `0` → node1 |
| `data/users.csv` | `33102945` | `33102945 % 3 = 0` → node1 | `1` → node2 |

---

## Replication Strategy

### On Upload

1. Object API loads the configured node list and computes primary and replica node indices
2. Sends the file to **both nodes in parallel**
3. Both writes must succeed
4. On success, metadata (including both node IDs) is committed to `object_db`
5. If either write fails, the upload is rejected and no metadata is written

```
Object API
  ├──► PUT /objects/{key}  →  Primary Node  ✓
  └──► PUT /objects/{key}  →  Replica Node  ✓
         │
         ▼
  Write metadata to object_db
```

### On Download

1. Object API looks up metadata to find the primary node
2. Requests the file from the primary node
3. If the primary node is unavailable, falls back to the replica node

```
Object API
  ├──► GET /objects/{key}  →  Primary Node  ✓ (used if available)
  └──► GET /objects/{key}  →  Replica Node  ✓ (fallback)
```

### On Delete

1. Object API looks up both node locations in metadata
2. Sends delete requests to **both nodes**
3. Removes the metadata row from `object_db`

```
Object API
  ├──► DELETE /objects/{key}  →  Primary Node
  └──► DELETE /objects/{key}  →  Replica Node
         │
         ▼
  Delete metadata from object_db
```

---

## Limitations of Modulo Hashing

The current strategy is intentionally simple. It has known trade-offs:

| Limitation | Impact |
|---|---|
| Adding/removing a node changes placement for most keys | Objects need to be re-distributed manually |
| No automatic rebalancing | Uneven distribution if nodes differ in capacity |
| No weighted placement | All nodes treated equally regardless of size |

---

## Operational Note: Deployment Across All Nodes

Because placement depends on a shared node configuration, deployment should roll out Object API, node services, and config updates together.

Use an automated deployment script to deploy service updates to all configured nodes in one workflow. This keeps runtime topology and placement logic aligned.

---

## Future: Consistent Hashing

A production-grade system would use a **consistent hashing ring**, where:

- Nodes are placed at positions on a virtual ring
- Each key maps to the nearest node clockwise on the ring
- Adding or removing a node only redistributes a fraction of keys (≈ 1/N)

CloudBin is designed so the placement logic is isolated in the Object API service, making it straightforward to swap in consistent hashing as an improvement.

See [roadmap.md](roadmap.md) for more detail.

---

## Key Takeaway

The placement formula is:

```
primary = hash(key) % N
replica = (primary + 1) % N
```

It is deterministic, cheap to compute, and requires no external coordination. The Object API is the sole owner of this logic, using a configurable node list at runtime.