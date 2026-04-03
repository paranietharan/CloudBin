CREATE TABLE IF NOT EXISTS objects (
    id UUID PRIMARY KEY,
    owner_user_id UUID NOT NULL,
    object_key TEXT NOT NULL,
    content_type TEXT,
    size_bytes BIGINT NOT NULL,
    etag TEXT,
    permission TEXT NOT NULL DEFAULT 'private-read',
    visibility TEXT NOT NULL DEFAULT 'visible',
    hidden_by UUID,
    hidden_at TIMESTAMPTZ,
    deleted_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (owner_user_id, object_key)
);

CREATE INDEX IF NOT EXISTS idx_objects_owner_user_id ON objects(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_objects_object_key ON objects(object_key);

CREATE TABLE IF NOT EXISTS object_placements (
    object_id UUID PRIMARY KEY REFERENCES objects(id) ON DELETE CASCADE,
    primary_node_id TEXT NOT NULL,
    replica_node_id TEXT NOT NULL,
    placement_version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (primary_node_id <> replica_node_id)
);

CREATE INDEX IF NOT EXISTS idx_object_placements_primary_node ON object_placements(primary_node_id);
CREATE INDEX IF NOT EXISTS idx_object_placements_replica_node ON object_placements(replica_node_id);

CREATE TABLE IF NOT EXISTS object_access_audit (
    id UUID PRIMARY KEY,
    object_id UUID REFERENCES objects(id) ON DELETE SET NULL,
    owner_user_id UUID,
    action TEXT NOT NULL,
    status TEXT NOT NULL,
    source_ip TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_object_access_audit_object_id ON object_access_audit(object_id);
CREATE INDEX IF NOT EXISTS idx_object_access_audit_owner_user_id ON object_access_audit(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_object_access_audit_created_at ON object_access_audit(created_at);

CREATE TABLE IF NOT EXISTS object_visibility_events (
    id UUID PRIMARY KEY,
    object_id UUID NOT NULL REFERENCES objects(id) ON DELETE CASCADE,
    actor_user_id UUID NOT NULL,
    actor_role TEXT NOT NULL,
    action TEXT NOT NULL,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_object_visibility_events_object_id ON object_visibility_events(object_id);
CREATE INDEX IF NOT EXISTS idx_object_visibility_events_actor_user_id ON object_visibility_events(actor_user_id);
CREATE INDEX IF NOT EXISTS idx_object_visibility_events_created_at ON object_visibility_events(created_at);

