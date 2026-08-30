package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"cloudbin-object-api/internal/object/model"
	"cloudbin-object-api/internal/object/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
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

type HashRing struct {
	nodes        []string
	virtualNodes map[uint64]string
	keys         []uint64
}

func NewHashRing(nodes []string, replicasPerNode int) *HashRing {
	hr := &HashRing{
		nodes:        nodes,
		virtualNodes: make(map[uint64]string),
	}
	if replicasPerNode <= 0 {
		replicasPerNode = 50
	}
	for _, node := range nodes {
		for i := 0; i < replicasPerNode; i++ {
			vKey := fmt.Sprintf("%s#%d", node, i)
			h := fnv.New64a()
			_, _ = h.Write([]byte(vKey))
			sum := h.Sum64()
			hr.virtualNodes[sum] = node
			hr.keys = append(hr.keys, sum)
		}
	}
	sort.Slice(hr.keys, func(i, j int) bool { return hr.keys[i] < hr.keys[j] })
	return hr
}

func (hr *HashRing) GetNodes(key string, count int) []string {
	if len(hr.nodes) == 0 || count <= 0 {
		return nil
	}
	if count > len(hr.nodes) {
		count = len(hr.nodes)
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	target := h.Sum64()

	idx := sort.Search(len(hr.keys), func(i int) bool {
		return hr.keys[i] >= target
	})
	if idx == len(hr.keys) {
		idx = 0
	}

	selected := make([]string, 0, count)
	seen := make(map[string]bool)

	for i := 0; i < len(hr.keys) && len(selected) < count; i++ {
		currIdx := (idx + i) % len(hr.keys)
		node := hr.virtualNodes[hr.keys[currIdx]]
		if !seen[node] {
			seen[node] = true
			selected = append(selected, node)
		}
	}
	return selected
}

type Service struct {
	repo              Repository
	httpClient        *http.Client
	nodes             []string
	hashRing          *HashRing
	replicationFactor int
	redisClient       *redis.Client
	fallbackShareMu   sync.RWMutex
	fallbackShare     map[string]shareLinkPayload
}

type shareLinkPayload struct {
	OwnerID   string    `json:"owner_id"`
	ObjectKey string    `json:"object_key"`
	ExpiresAt time.Time `json:"expires_at"`
}

func New(repo Repository, nodes []string, replicationFactor int, redisClient *redis.Client) (*Service, error) {
	if len(nodes) < 2 {
		return nil, ErrInvalidNodeConfig
	}
	if replicationFactor < 2 {
		replicationFactor = 2
	}
	if replicationFactor > len(nodes) {
		replicationFactor = len(nodes)
	}

	return &Service{
		repo:              repo,
		httpClient:        &http.Client{Timeout: 30 * time.Second},
		nodes:             nodes,
		hashRing:          NewHashRing(nodes, 50),
		replicationFactor: replicationFactor,
		redisClient:       redisClient,
		fallbackShare:     make(map[string]shareLinkPayload),
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

	targetNodes := s.placement(ownerID, objectKey)
	if len(targetNodes) < 2 {
		return ErrInvalidNodeConfig
	}
	primary := targetNodes[0]
	replica := targetNodes[1]
	storageKey := storagePath(ownerID, objectKey)

	var wg sync.WaitGroup
	results := make(chan error, len(targetNodes))
	for _, node := range targetNodes {
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
		for _, node := range targetNodes {
			_ = s.deleteFromNode(context.Background(), node, storageKey)
		}
		return ErrReplicationWriteFail
	}

	sum := sha256.Sum256(data)
	etag := hex.EncodeToString(sum[:])
	_, err := s.repo.UpsertObject(ctx, ownerID, objectKey, contentType, int64(len(data)), etag, primary, replica)
	return err
}

func (s *Service) Download(ctx context.Context, ownerID, objectKey string) (model.ObjectRecord, io.ReadCloser, error) {
	rec, err := s.repo.FindByOwnerAndKey(ctx, ownerID, objectKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ObjectRecord{}, nil, ErrNotFound
		}
		return model.ObjectRecord{}, nil, err
	}

	storageKey := storagePath(rec.OwnerUserID, rec.ObjectKey)
	stream, err := s.getFromNodeStream(ctx, rec.PrimaryNode, storageKey)
	if err != nil && rec.ReplicaNode != "" {
		stream, err = s.getFromNodeStream(ctx, rec.ReplicaNode, storageKey)
		if err != nil {
			return model.ObjectRecord{}, nil, err
		}
	}
	return rec, stream, nil
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
	if rec.PrimaryNode != "" {
		_ = s.deleteFromNode(ctx, rec.PrimaryNode, storageKey)
	}
	if rec.ReplicaNode != "" {
		_ = s.deleteFromNode(ctx, rec.ReplicaNode, storageKey)
	}

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

	ok, err := s.repo.SetPermissionByOwnerAndKey(ctx, ownerID, objectKey, permission)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

func (s *Service) DownloadPublic(ctx context.Context, ownerID, objectKey string) (model.ObjectRecord, io.ReadCloser, error) {
	rec, err := s.repo.FindByOwnerAndKey(ctx, ownerID, objectKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ObjectRecord{}, nil, ErrNotFound
		}
		return model.ObjectRecord{}, nil, err
	}
	if rec.Permission != "public-read" || rec.Visibility != "visible" {
		return model.ObjectRecord{}, nil, ErrForbidden
	}

	storageKey := storagePath(rec.OwnerUserID, rec.ObjectKey)
	stream, err := s.getFromNodeStream(ctx, rec.PrimaryNode, storageKey)
	if err != nil && rec.ReplicaNode != "" {
		stream, err = s.getFromNodeStream(ctx, rec.ReplicaNode, storageKey)
		if err != nil {
			return model.ObjectRecord{}, nil, err
		}
	}
	return rec, stream, nil
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
	payload := shareLinkPayload{OwnerID: ownerID, ObjectKey: objectKey, ExpiresAt: expiresAt}

	if s.redisClient != nil {
		raw, _ := json.Marshal(payload)
		_ = s.redisClient.Set(ctx, "share:link:"+token, raw, ttl).Err()
	} else {
		s.fallbackShareMu.Lock()
		s.fallbackShare[token] = payload
		s.fallbackShareMu.Unlock()
	}

	return token, expiresAt, nil
}

func (s *Service) DownloadByShareToken(ctx context.Context, token string) (model.ObjectRecord, io.ReadCloser, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return model.ObjectRecord{}, nil, ErrInvalidKey
	}

	var entry shareLinkPayload
	if s.redisClient != nil {
		raw, err := s.redisClient.Get(ctx, "share:link:"+token).Result()
		if err != nil {
			return model.ObjectRecord{}, nil, ErrNotFound
		}
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			return model.ObjectRecord{}, nil, ErrNotFound
		}
	} else {
		s.fallbackShareMu.RLock()
		item, ok := s.fallbackShare[token]
		s.fallbackShareMu.RUnlock()
		if !ok || time.Now().After(item.ExpiresAt) {
			return model.ObjectRecord{}, nil, ErrNotFound
		}
		entry = item
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
	stream, err := s.getFromNodeStream(ctx, rec.PrimaryNode, storageKey)
	if err != nil && rec.ReplicaNode != "" {
		stream, err = s.getFromNodeStream(ctx, rec.ReplicaNode, storageKey)
		if err != nil {
			return model.ObjectRecord{}, nil, err
		}
	}

	return rec, stream, nil
}

func (s *Service) placement(ownerID, objectKey string) []string {
	key := storagePath(ownerID, objectKey)
	if s.hashRing != nil {
		return s.hashRing.GetNodes(key, s.replicationFactor)
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	pIdx := int(h.Sum64() % uint64(len(s.nodes)))
	rIdx := (pIdx + 1) % len(s.nodes)
	return []string{s.nodes[pIdx], s.nodes[rIdx]}
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

func (s *Service) getFromNodeStream(ctx context.Context, node, objectKey string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/objects/%s", strings.TrimRight(node, "/"), objectKey), nil)
	if err != nil {
		return nil, err
	}
	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, fmt.Errorf("storage node get failed: %d", res.StatusCode)
	}
	return res.Body, nil
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
