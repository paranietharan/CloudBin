package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloudbin-object-api/internal/object/model"
)

type mockRepo struct {
	setPermissionOK  bool
	setPermissionErr error
	lastPermission   string
}

func (m *mockRepo) UpsertObject(ctx context.Context, ownerID, objectKey, contentType string, sizeBytes int64, etag, primaryNode, replicaNode string) (string, error) {
	return "", nil
}

func (m *mockRepo) FindByOwnerAndKey(ctx context.Context, ownerID, objectKey string) (model.ObjectRecord, error) {
	return model.ObjectRecord{}, errors.New("not implemented")
}

func (m *mockRepo) SetPermissionByOwnerAndKey(ctx context.Context, ownerID, objectKey, permission string) (bool, error) {
	m.lastPermission = permission
	return m.setPermissionOK, m.setPermissionErr
}

func (m *mockRepo) HideByOwnerAndKey(ctx context.Context, ownerID, objectKey, actorID, actorRole, reason string) (bool, error) {
	return false, nil
}

func (m *mockRepo) DeleteByOwnerAndKey(ctx context.Context, ownerID, objectKey string) (bool, error) {
	return false, nil
}

func (m *mockRepo) ListByOwner(ctx context.Context, ownerID string) ([]model.ObjectRecord, error) {
	return nil, nil
}

func (m *mockRepo) ListByOwnerWithFilters(ctx context.Context, ownerID, permission, visibility, keyQuery string, limit, offset int) ([]model.ObjectRecord, error) {
	return nil, nil
}

func (m *mockRepo) CountByOwnerWithFilters(ctx context.Context, ownerID, permission, visibility, keyQuery string) (int, error) {
	return 0, nil
}

func (m *mockRepo) MarkAccessAudit(ctx context.Context, objectID, ownerID, action, status, sourceIP, userAgent string) error {
	return nil
}

func TestMakePublicReadSuccess(t *testing.T) {
	repo := &mockRepo{setPermissionOK: true}
	svc, err := New(repo, []string{"http://n1", "http://n2"}, 2, nil)
	if err != nil {
		t.Fatalf("unexpected new service error: %v", err)
	}

	err = svc.MakePublicRead(context.Background(), "owner-1", "file-key")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if repo.lastPermission != "public-read" {
		t.Fatalf("expected permission public-read, got: %s", repo.lastPermission)
	}
}

func TestMakePrivateReadNotFound(t *testing.T) {
	repo := &mockRepo{setPermissionOK: false}
	svc, err := New(repo, []string{"http://n1", "http://n2"}, 2, nil)
	if err != nil {
		t.Fatalf("unexpected new service error: %v", err)
	}

	err = svc.MakePrivateRead(context.Background(), "owner-1", "missing-key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
	if repo.lastPermission != "private-read" {
		t.Fatalf("expected permission private-read, got: %s", repo.lastPermission)
	}
}

func TestCreateTemporaryShareLinkInvalidKey(t *testing.T) {
	repo := &mockRepo{setPermissionOK: true}
	svc, err := New(repo, []string{"http://n1", "http://n2"}, 2, nil)
	if err != nil {
		t.Fatalf("unexpected new service error: %v", err)
	}

	_, _, err = svc.CreateTemporaryShareLink(context.Background(), "owner-1", "", 15*time.Minute)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got: %v", err)
	}
}
