package repository

import (
	"cloudbin-object-api/internal/object/model"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
)

func TestVersionCommitAndCleanup(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required with object migrations")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewPostgres(db)
	rec := model.ObjectRecord{OwnerUserID: uuid.NewString(), ObjectKey: "report", PrimaryNode: "node-a", ReplicaNode: "node-b", Nodes: []string{"node-a", "node-b"}}
	write := func(prev *model.ObjectRecord) error {
		rec.StorageKey = "_versions/" + uuid.NewString()
		if err := repo.Stage(ctx, rec.StorageKey, rec.Nodes); err != nil {
			return err
		}
		return repo.CommitVersion(ctx, rec, prev)
	}
	if err := write(nil); err != nil {
		t.Fatal(err)
	}
	old, err := repo.FindByOwnerAndKey(ctx, rec.OwnerUserID, rec.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := write(&old); err != nil {
		t.Fatal(err)
	}
	published := rec.StorageKey
	if err := write(&old); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale writer: %v", err)
	}
	got, err := repo.FindByOwnerAndKey(ctx, rec.OwnerUserID, rec.ObjectKey)
	if err != nil || got.StorageKey != published {
		t.Fatalf("published pointer: %v %v", got, err)
	}
	for _, key := range []string{old.StorageKey, rec.StorageKey} {
		var pending bool
		if err := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM storage_cleanup WHERE storage_key=$1)", key).Scan(&pending); err != nil || !pending {
			t.Fatalf("missing cleanup %s: %v", key, err)
		}
	}
	if ok, err := repo.DeleteVersion(ctx, got); err != nil || !ok {
		t.Fatal(ok, err)
	}
}
