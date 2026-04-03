DROP INDEX IF EXISTS idx_object_visibility_events_created_at;
DROP INDEX IF EXISTS idx_object_visibility_events_actor_user_id;
DROP INDEX IF EXISTS idx_object_visibility_events_object_id;

DROP INDEX IF EXISTS idx_object_access_audit_created_at;
DROP INDEX IF EXISTS idx_object_access_audit_owner_user_id;
DROP INDEX IF EXISTS idx_object_access_audit_object_id;

DROP INDEX IF EXISTS idx_object_placements_replica_node;
DROP INDEX IF EXISTS idx_object_placements_primary_node;

DROP INDEX IF EXISTS idx_objects_object_key;
DROP INDEX IF EXISTS idx_objects_owner_user_id;

DROP TABLE IF EXISTS object_visibility_events;
DROP TABLE IF EXISTS object_access_audit;
DROP TABLE IF EXISTS object_placements;
DROP TABLE IF EXISTS objects;

