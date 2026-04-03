package repository

import (
	"context"
	"errors"
	"time"

	"cloudbin-object-api/internal/object/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("object not found")

type Postgres struct {
	db *pgxpool.Pool
}

func NewPostgres(db *pgxpool.Pool) *Postgres {
	return &Postgres{db: db}
}

func (p *Postgres) UpsertObject(ctx context.Context, ownerID, objectKey, contentType string, sizeBytes int64, etag, primaryNode, replicaNode string) (string, error) {
	id := uuid.New()
	var objectID uuid.UUID
	err := p.db.QueryRow(ctx, `
		WITH upserted AS (
			INSERT INTO objects (id, owner_user_id, object_key, content_type, size_bytes, etag, visibility, hidden_by, hidden_at, deleted_by, deleted_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'visible', NULL, NULL, NULL, NULL, NOW(), NOW())
			ON CONFLICT (owner_user_id, object_key)
			DO UPDATE SET
				content_type = EXCLUDED.content_type,
				size_bytes = EXCLUDED.size_bytes,
				etag = EXCLUDED.etag,
				visibility = 'visible',
				hidden_by = NULL,
				hidden_at = NULL,
				deleted_by = NULL,
				deleted_at = NULL,
				updated_at = NOW()
			RETURNING id
		)
		INSERT INTO object_placements (object_id, primary_node_id, replica_node_id, placement_version, created_at, updated_at)
		SELECT id, $7, $8, 1, NOW(), NOW() FROM upserted
		ON CONFLICT (object_id)
		DO UPDATE SET
			primary_node_id = EXCLUDED.primary_node_id,
			replica_node_id = EXCLUDED.replica_node_id,
			updated_at = NOW()
		RETURNING object_id
	`, id, ownerID, objectKey, contentType, sizeBytes, etag, primaryNode, replicaNode).Scan(&objectID)
	if err != nil {
		return "", err
	}
	return objectID.String(), nil
}

func (p *Postgres) FindByOwnerAndKey(ctx context.Context, ownerID, objectKey string) (model.ObjectRecord, error) {
	var rec model.ObjectRecord
	err := p.db.QueryRow(ctx, `
		SELECT o.id, o.owner_user_id, o.object_key, COALESCE(o.content_type, ''), o.size_bytes, COALESCE(o.etag, ''),
		       o.visibility, op.primary_node_id, op.replica_node_id, o.created_at, o.updated_at
		FROM objects o
		JOIN object_placements op ON op.object_id = o.id
		WHERE o.owner_user_id = $1 AND o.object_key = $2 AND o.deleted_at IS NULL
	`, ownerID, objectKey).Scan(
		&rec.ID,
		&rec.OwnerUserID,
		&rec.ObjectKey,
		&rec.ContentType,
		&rec.SizeBytes,
		&rec.ETag,
		&rec.Visibility,
		&rec.PrimaryNode,
		&rec.ReplicaNode,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ObjectRecord{}, ErrNotFound
	}
	return rec, err
}

func (p *Postgres) HideByOwnerAndKey(ctx context.Context, ownerID, objectKey, actorID, actorRole, reason string) (bool, error) {
	var objectID uuid.UUID
	err := p.db.QueryRow(ctx, `
		UPDATE objects
		SET visibility = 'hidden', hidden_by = $3, hidden_at = NOW(), updated_at = NOW()
		WHERE owner_user_id = $1 AND object_key = $2 AND deleted_at IS NULL
		RETURNING id
	`, ownerID, objectKey, actorID).Scan(&objectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	_, err = p.db.Exec(ctx, `
		INSERT INTO object_visibility_events (id, object_id, actor_user_id, actor_role, action, reason, created_at)
		VALUES ($1, $2, $3, $4, 'hide', $5, NOW())
	`, uuid.New(), objectID, actorID, actorRole, reason)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (p *Postgres) DeleteByOwnerAndKey(ctx context.Context, ownerID, objectKey string) (bool, error) {
	tag, err := p.db.Exec(ctx, `
		DELETE FROM objects
		WHERE owner_user_id = $1 AND object_key = $2
	`, ownerID, objectKey)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (p *Postgres) ListByOwner(ctx context.Context, ownerID string) ([]model.ObjectRecord, error) {
	rows, err := p.db.Query(ctx, `
		SELECT o.id, o.owner_user_id, o.object_key, COALESCE(o.content_type, ''), o.size_bytes, COALESCE(o.etag, ''),
		       o.visibility, op.primary_node_id, op.replica_node_id, o.created_at, o.updated_at
		FROM objects o
		JOIN object_placements op ON op.object_id = o.id
		WHERE o.owner_user_id = $1 AND o.deleted_at IS NULL
		ORDER BY o.updated_at DESC
	`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ObjectRecord, 0)
	for rows.Next() {
		var rec model.ObjectRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.OwnerUserID,
			&rec.ObjectKey,
			&rec.ContentType,
			&rec.SizeBytes,
			&rec.ETag,
			&rec.Visibility,
			&rec.PrimaryNode,
			&rec.ReplicaNode,
			&rec.CreatedAt,
			&rec.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (p *Postgres) MarkAccessAudit(ctx context.Context, objectID, ownerID, action, status, sourceIP, userAgent string) error {
	_, err := p.db.Exec(ctx, `
		INSERT INTO object_access_audit (id, object_id, owner_user_id, action, status, source_ip, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`, uuid.New(), nullableUUID(objectID), nullableUUID(ownerID), action, status, sourceIP, userAgent)
	return err
}

func nullableUUID(v string) any {
	if v == "" {
		return nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return nil
	}
	return id
}

func (p *Postgres) NowUTC() time.Time {
	return time.Now().UTC()
}

