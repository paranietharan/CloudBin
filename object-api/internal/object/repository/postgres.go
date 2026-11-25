package repository

import (
	"context"
	"errors"

	"cloudbin-object-api/internal/object/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrConflict = errors.New("object changed during upload")

var ErrNotFound = errors.New("object not found")

type Postgres struct {
	db *pgxpool.Pool
}

func NewPostgres(db *pgxpool.Pool) *Postgres {
	return &Postgres{db: db}
}

// Stage records every destination before any bytes are written. If a writer
// crashes, the collector can safely reclaim the attempt after its grace period.
func (p *Postgres) Stage(ctx context.Context, key string, nodes []string) error {
	_, err := p.db.Exec(ctx, "INSERT INTO storage_cleanup(storage_key,nodes) VALUES ($1,$2)", key, nodes)
	return err
}

func (p *Postgres) CommitVersion(ctx context.Context, rec model.ObjectRecord, previous *model.ObjectRecord) error {
	return pgx.BeginFunc(ctx, p.db, func(tx pgx.Tx) error {
		// Claiming this row prevents a publisher racing an expired staging entry.
		tag, err := tx.Exec(ctx, "DELETE FROM storage_cleanup WHERE storage_key=$1 AND eligible_at>now()", rec.StorageKey)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
		var id string
		if previous == nil {
			err = tx.QueryRow(ctx, `
                INSERT INTO objects(id,owner_user_id,object_key,content_type,size_bytes,etag,storage_key)
                VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(owner_user_id,object_key) DO NOTHING RETURNING id`,
				uuid.New(), rec.OwnerUserID, rec.ObjectKey, rec.ContentType, rec.SizeBytes, rec.ETag, rec.StorageKey).Scan(&id)
		} else {
			err = tx.QueryRow(ctx, `
                UPDATE objects SET content_type=$2,size_bytes=$3,etag=$4,storage_key=$5,
                    visibility='visible',hidden_by=NULL,hidden_at=NULL,updated_at=now()
                WHERE id=$1 AND COALESCE(storage_key,'')=$6 RETURNING id`,
				previous.ID, rec.ContentType, rec.SizeBytes, rec.ETag, rec.StorageKey, previous.StorageKey).Scan(&id)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		if previous != nil {
			if err := queueOldVersion(ctx, tx, *previous); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO object_placements(object_id,primary_node_id,replica_node_id,nodes)
            VALUES($1,$2,$3,$4) ON CONFLICT(object_id) DO UPDATE SET
            primary_node_id=EXCLUDED.primary_node_id,replica_node_id=EXCLUDED.replica_node_id,
            nodes=EXCLUDED.nodes, updated_at=now(), placement_version=object_placements.placement_version+1`,
			id, rec.PrimaryNode, rec.ReplicaNode, rec.Nodes)
		return err
	})
}

func queueOldVersion(ctx context.Context, tx pgx.Tx, rec model.ObjectRecord) error {
	_, err := tx.Exec(ctx, `INSERT INTO storage_cleanup(storage_key,nodes) VALUES ($1,$2)
        ON CONFLICT(storage_key) DO NOTHING`, rec.BlobKey(), rec.Nodes)
	return err
}

// CollectOne locks one old version until its bounded, idempotent deletes finish.
func (p *Postgres) CollectOne(ctx context.Context, remove func(context.Context, string, string) error) (bool, error) {
	var found bool
	err := pgx.BeginFunc(ctx, p.db, func(tx pgx.Tx) error {
		var key string
		var nodes []string
		err := tx.QueryRow(ctx, `SELECT storage_key,nodes FROM storage_cleanup WHERE eligible_at<=now()
           ORDER BY eligible_at LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&key, &nodes)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		for _, node := range nodes {
			if err := remove(ctx, node, key); err != nil {
				_, retryErr := tx.Exec(ctx, "UPDATE storage_cleanup SET eligible_at=now()+interval '5 minutes' WHERE storage_key=$1", key)
				return retryErr
			}
		}
		_, err = tx.Exec(ctx, "DELETE FROM storage_cleanup WHERE storage_key=$1", key)
		return err
	})
	return found, err
}

func (p *Postgres) FindByOwnerAndKey(ctx context.Context, ownerID, objectKey string) (model.ObjectRecord, error) {
	var rec model.ObjectRecord
	err := p.db.QueryRow(ctx, `
		SELECT o.id, o.owner_user_id, o.object_key, COALESCE(o.content_type, ''), o.size_bytes, COALESCE(o.etag, ''),
		       o.permission, o.visibility, op.primary_node_id, op.replica_node_id, o.created_at, o.updated_at, COALESCE(o.storage_key,''), COALESCE(op.nodes,ARRAY[op.primary_node_id,op.replica_node_id])
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
		&rec.Permission,
		&rec.Visibility,
		&rec.PrimaryNode,
		&rec.ReplicaNode,
		&rec.CreatedAt,
		&rec.UpdatedAt, &rec.StorageKey, &rec.Nodes,
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

func (p *Postgres) DeleteVersion(ctx context.Context, rec model.ObjectRecord) (bool, error) {
	var deleted bool
	err := pgx.BeginFunc(ctx, p.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, "DELETE FROM objects WHERE id=$1 AND COALESCE(storage_key,'')=$2", rec.ID, rec.StorageKey)
		if err != nil {
			return err
		}
		deleted = tag.RowsAffected() == 1
		if deleted {
			return queueOldVersion(ctx, tx, rec)
		}
		return nil
	})
	return deleted, err
}

func (p *Postgres) ListByOwnerWithFilters(ctx context.Context, ownerID, permission, visibility, keyQuery string, limit, offset int) ([]model.ObjectRecord, error) {
	rows, err := p.db.Query(ctx, `
		SELECT o.id, o.owner_user_id, o.object_key, COALESCE(o.content_type, ''), o.size_bytes, COALESCE(o.etag, ''),
		       o.permission, o.visibility, op.primary_node_id, op.replica_node_id, o.created_at, o.updated_at
		FROM objects o
		JOIN object_placements op ON op.object_id = o.id
		WHERE o.owner_user_id = $1
		  AND o.deleted_at IS NULL
		  AND ($2 = '' OR o.permission = $2)
		  AND ($3 = '' OR o.visibility = $3)
		  AND ($4 = '' OR o.object_key ILIKE '%' || $4 || '%')
		ORDER BY o.updated_at DESC
		LIMIT $5 OFFSET $6
	`, ownerID, permission, visibility, keyQuery, limit, offset)
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
			&rec.Permission,
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

func (p *Postgres) CountByOwnerWithFilters(ctx context.Context, ownerID, permission, visibility, keyQuery string) (int, error) {
	var total int
	err := p.db.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM objects o
		WHERE o.owner_user_id = $1
		  AND o.deleted_at IS NULL
		  AND ($2 = '' OR o.permission = $2)
		  AND ($3 = '' OR o.visibility = $3)
		  AND ($4 = '' OR o.object_key ILIKE '%' || $4 || '%')
	`, ownerID, permission, visibility, keyQuery).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (p *Postgres) SetPermissionByOwnerAndKey(ctx context.Context, ownerID, objectKey, permission string) (bool, error) {
	tag, err := p.db.Exec(ctx, `
		UPDATE objects
		SET permission = $3, updated_at = NOW()
		WHERE owner_user_id = $1 AND object_key = $2 AND deleted_at IS NULL
	`, ownerID, objectKey, permission)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
