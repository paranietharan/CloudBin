package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"cloudbin-object-api/internal/object/model"
	"cloudbin-object-api/internal/object/repository"

	"github.com/google/uuid"
)

var (
	ErrNotFound             = errors.New("object not found")
	ErrForbidden            = errors.New("forbidden")
	ErrInvalidKey           = errors.New("object key is required")
	ErrInvalidNodeConfig    = errors.New("invalid storage node configuration")
	ErrReplicationWriteFail = errors.New("failed to replicate to storage nodes")
)

type Repository interface {
	UpsertObject(ctx context.Context, ownerID, objectKey, contentType string, sizeBytes int64, etag, primaryNode, replicaNode string) (string, error)
	FindByOwnerAndKey(ctx context.Context, ownerID, objectKey string) (model.ObjectRecord, error)
	SetPermissionByOwnerAndKey(ctx context.Context, ownerID, objectKey, permission string) (bool, error)
	HideByOwnerAndKey(ctx context.Context, ownerID, objectKey, actorID, actorRole, reason string) (bool, error)
	DeleteByOwnerAndKey(ctx context.Context, ownerID, objectKey string) (bool, error)
	ListByOwner(ctx context.Context, ownerID string) ([]model.ObjectRecord, error)
	ListByOwnerWithFilters(ctx context.Context, ownerID, permission, visibility, keyQuery string, limit, offset int) ([]model.ObjectRecord, error)
	CountByOwnerWithFilters(ctx context.Context, ownerID, permission, visibility, keyQuery string) (int, error)
	MarkAccessAudit(ctx context.Context, objectID, ownerID, action, status, sourceIP, userAgent string) error
}

type Service struct {
	repo              Repository
	httpClient        *http.Client
	nodes             []string
	replicationFactor int
	shareMu           sync.RWMutex
	shareLinks        map[string]shareLink
}

type shareLink struct {
	OwnerID   string
	ObjectKey string
	ExpiresAt time.Time
}

func New(repo Repository, nodes []string, replicationFactor int) (*Service, error) {
	if len(nodes) < 2 {
		return nil, ErrInvalidNodeConfig
	}
	if replicationFactor < 2 {
		replicationFactor = 2
	}
	if replicationFactor > len(nodes) {
		return nil, ErrInvalidNodeConfig
	}

	return &Service{
		repo:              repo,
		httpClient:        &http.Client{Timeout: 8 * time.Second},
		nodes:             nodes,
		replicationFactor: replicationFactor,
		shareLinks:        make(map[string]shareLink),
	}, nil
}

func (s *Service) Upload(ctx context.Context, ownerID, objectKey, contentType string, data []byte) error {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return ErrInvalidKey
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	primary, replica := s.placement(objectKey)
	storageKey := storagePath(ownerID, objectKey)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, node := range []string{primary, replica} {
		n := node
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- s.putToNode(ctx, n, storageKey, contentType, data)
		}()
	}
	wg.Wait()
	close(results)

	fails := 0
	for err := range results {
		if err != nil {
			fails++
		}
	}
	if fails > 0 {
		_ = s.deleteFromNode(ctx, primary, storageKey)
		_ = s.deleteFromNode(ctx, replica, storageKey)
		return ErrReplicationWriteFail
	}

	sum := sha256.Sum256(data)
	etag := hex.EncodeToString(sum[:])
	_, err := s.repo.UpsertObject(ctx, ownerID, objectKey, contentType, int64(len(data)), etag, primary, replica)
	return err
}

func (s *Service) Download(ctx context.Context, ownerID, objectKey string) (model.ObjectRecord, []byte, error) {
	rec, err := s.repo.FindByOwnerAndKey(ctx, ownerID, objectKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ObjectRecord{}, nil, ErrNotFound
		}
		return model.ObjectRecord{}, nil, err
	}

	storageKey := storagePath(rec.OwnerUserID, rec.ObjectKey)
	body, err := s.getFromNode(ctx, rec.PrimaryNode, storageKey)
	if err != nil {
		body, err = s.getFromNode(ctx, rec.ReplicaNode, storageKey)
		if err != nil {
			return model.ObjectRecord{}, nil, err
		}
	}
	return rec, body, nil
}

func (s *Service) Delete(ctx context.Context, ownerID, objectKey string) error {
	rec, err := s.repo.FindByOwnerAndKey(ctx, ownerID, objectKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	storageKey := storagePath(rec.OwnerUserID, rec.ObjectKey)
	_ = s.deleteFromNode(ctx, rec.PrimaryNode, storageKey)
	_ = s.deleteFromNode(ctx, rec.ReplicaNode, storageKey)

	ok, err := s.repo.DeleteByOwnerAndKey(ctx, ownerID, objectKey)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Hide(ctx context.Context, ownerID, objectKey, actorID, actorRole, reason string) error {
	ok, err := s.repo.HideByOwnerAndKey(ctx, ownerID, objectKey, actorID, actorRole, reason)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListOwnerObjects(ctx context.Context, ownerID string) ([]model.ObjectRecord, error) {
	return s.repo.ListByOwner(ctx, ownerID)
}

func (s *Service) ListOwnerObjectsPage(ctx context.Context, ownerID, permission, visibility, keyQuery string, limit, offset int) ([]model.ObjectRecord, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	permission = strings.TrimSpace(permission)
	visibility = strings.TrimSpace(visibility)
	keyQuery = strings.TrimSpace(keyQuery)

	files, err := s.repo.ListByOwnerWithFilters(ctx, ownerID, permission, visibility, keyQuery, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountByOwnerWithFilters(ctx, ownerID, permission, visibility, keyQuery)
	if err != nil {
		return nil, 0, err
	}

	return files, total, nil
}

func (s *Service) FileExists(ctx context.Context, ownerID, objectKey string) (bool, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return false, ErrInvalidKey
	}

	_, err := s.repo.FindByOwnerAndKey(ctx, ownerID, objectKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (s *Service) MakePrivateRead(ctx context.Context, ownerID, objectKey string) error {
	return s.setPermission(ctx, ownerID, objectKey, "private-read")
}

func (s *Service) MakePublicRead(ctx context.Context, ownerID, objectKey string) error {
	return s.setPermission(ctx, ownerID, objectKey, "public-read")
}

func (s *Service) setPermission(ctx context.Context, ownerID, objectKey, permission string) error {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return ErrInvalidKey
	}
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return ErrInvalidKey
	}

	log.Printf("set-permission requested: owner_id=%s object_key=%s permission=%s", ownerID, objectKey, permission)

	ok, err := s.repo.SetPermissionByOwnerAndKey(ctx, ownerID, objectKey, permission)
	if err != nil {
		log.Printf("set-permission repo error: owner_id=%s object_key=%s permission=%s err=%v", ownerID, objectKey, permission, err)
		return err
	}
	if !ok {
		log.Printf("set-permission not found: owner_id=%s object_key=%s permission=%s", ownerID, objectKey, permission)
		return ErrNotFound
	}
	log.Printf("set-permission success: owner_id=%s object_key=%s permission=%s", ownerID, objectKey, permission)
	return nil
}

func (s *Service) DownloadPublic(ctx context.Context, ownerID, objectKey string) (model.ObjectRecord, []byte, error) {
	rec, err := s.repo.FindByOwnerAndKey(ctx, ownerID, objectKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			log.Printf("public download not found: owner_id=%s object_key=%s", ownerID, objectKey)
			return model.ObjectRecord{}, nil, ErrNotFound
		}
		log.Printf("public download repo error: owner_id=%s object_key=%s err=%v", ownerID, objectKey, err)
		return model.ObjectRecord{}, nil, err
	}
	if rec.Permission != "public-read" || rec.Visibility != "visible" {
		log.Printf("public download forbidden: owner_id=%s object_key=%s permission=%s visibility=%s", ownerID, objectKey, rec.Permission, rec.Visibility)
		return model.ObjectRecord{}, nil, ErrForbidden
	}
	log.Printf("public download allowed: owner_id=%s object_key=%s permission=%s", ownerID, objectKey, rec.Permission)

	storageKey := storagePath(rec.OwnerUserID, rec.ObjectKey)
	body, err := s.getFromNode(ctx, rec.PrimaryNode, storageKey)
	if err != nil {
		body, err = s.getFromNode(ctx, rec.ReplicaNode, storageKey)
		if err != nil {
			return model.ObjectRecord{}, nil, err
		}
	}
	return rec, body, nil
}

func (s *Service) CreateTemporaryShareLink(ctx context.Context, ownerID, objectKey string, ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if ttl > 24*time.Hour {
		ttl = 24 * time.Hour
	}

	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return "", time.Time{}, ErrInvalidKey
	}

	rec, err := s.repo.FindByOwnerAndKey(ctx, ownerID, objectKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", time.Time{}, ErrNotFound
		}
		return "", time.Time{}, err
	}
	if rec.Visibility != "visible" {
		return "", time.Time{}, ErrForbidden
	}

	token := uuid.NewString()
	expiresAt := time.Now().Add(ttl)

	s.shareMu.Lock()
	s.shareLinks[token] = shareLink{OwnerID: ownerID, ObjectKey: objectKey, ExpiresAt: expiresAt}
	s.shareMu.Unlock()

	return token, expiresAt, nil
}

func (s *Service) DownloadByShareToken(ctx context.Context, token string) (model.ObjectRecord, []byte, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return model.ObjectRecord{}, nil, ErrInvalidKey
	}

	s.shareMu.RLock()
	entry, ok := s.shareLinks[token]
	s.shareMu.RUnlock()
	if !ok {
		return model.ObjectRecord{}, nil, ErrNotFound
	}
	if time.Now().After(entry.ExpiresAt) {
		s.shareMu.Lock()
		delete(s.shareLinks, token)
		s.shareMu.Unlock()
		return model.ObjectRecord{}, nil, ErrNotFound
	}

	rec, err := s.repo.FindByOwnerAndKey(ctx, entry.OwnerID, entry.ObjectKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ObjectRecord{}, nil, ErrNotFound
		}
		return model.ObjectRecord{}, nil, err
	}
	if rec.Visibility != "visible" {
		return model.ObjectRecord{}, nil, ErrForbidden
	}

	storageKey := storagePath(rec.OwnerUserID, rec.ObjectKey)
	body, err := s.getFromNode(ctx, rec.PrimaryNode, storageKey)
	if err != nil {
		body, err = s.getFromNode(ctx, rec.ReplicaNode, storageKey)
		if err != nil {
			return model.ObjectRecord{}, nil, err
		}
	}

	return rec, body, nil
}

func (s *Service) placement(objectKey string) (string, string) {
	h := fnv.New64a()
	_, _ = h.Write([]byte(objectKey))
	primaryIdx := int(h.Sum64() % uint64(len(s.nodes)))
	replicaIdx := (primaryIdx + 1) % len(s.nodes)
	return s.nodes[primaryIdx], s.nodes[replicaIdx]
}

func (s *Service) putToNode(ctx context.Context, node, objectKey, contentType string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/objects/%s", strings.TrimRight(node, "/"), objectKey), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)

	res, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("storage node put failed: %d", res.StatusCode)
}

func (s *Service) getFromNode(ctx context.Context, node, objectKey string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/objects/%s", strings.TrimRight(node, "/"), objectKey), nil)
	if err != nil {
		return nil, err
	}
	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("storage node get failed: %d", res.StatusCode)
	}
	return io.ReadAll(res.Body)
}

func (s *Service) deleteFromNode(ctx context.Context, node, objectKey string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/objects/%s", strings.TrimRight(node, "/"), objectKey), nil)
	if err != nil {
		return err
	}
	res, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound || (res.StatusCode >= 200 && res.StatusCode < 300) {
		return nil
	}
	return fmt.Errorf("storage node delete failed: %d", res.StatusCode)
}

func storagePath(ownerID, objectKey string) string {
	owner := strings.TrimSpace(ownerID)
	key := strings.TrimPrefix(strings.TrimSpace(objectKey), "/")
	return owner + "/" + key
}
