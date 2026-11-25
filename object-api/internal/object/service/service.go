package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"cloudbin-object-api/internal/object/model"
	"cloudbin-object-api/internal/object/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrNotFound             = repository.ErrNotFound
	ErrForbidden            = errors.New("forbidden")
	ErrConflict             = repository.ErrConflict
	ErrInvalidPermission    = errors.New("invalid permission: must be 'public-read' or 'private-read'")
	ErrInvalidKey           = errors.New("object key is required")
	ErrInvalidNodeConfig    = errors.New("invalid storage node configuration")
	ErrReplicationWriteFail = errors.New("failed to replicate to storage nodes")
)

type Repository interface {
	Stage(context.Context, string, []string) error
	CommitVersion(context.Context, model.ObjectRecord, *model.ObjectRecord) error
	FindByOwnerAndKey(ctx context.Context, ownerID, objectKey string) (model.ObjectRecord, error)
	SetPermissionByOwnerAndKey(ctx context.Context, ownerID, objectKey, permission string) (bool, error)
	HideByOwnerAndKey(ctx context.Context, ownerID, objectKey, actorID, actorRole, reason string) (bool, error)
	DeleteVersion(ctx context.Context, rec model.ObjectRecord) (bool, error)

	ListByOwnerWithFilters(ctx context.Context, ownerID, permission, visibility, keyQuery string, limit, offset int) ([]model.ObjectRecord, error)
	CountByOwnerWithFilters(ctx context.Context, ownerID, permission, visibility, keyQuery string) (int, error)
}

type Service struct {
	repo              Repository
	httpClient        *http.Client
	hashRing          *HashRing
	replicationFactor int
	redisClient       *redis.Client
	fallbackShareMu   sync.RWMutex
	fallbackShare     map[string]shareLinkPayload
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

	targetNodes := s.hashRing.GetNodes(model.OwnerKey(ownerID, objectKey), s.replicationFactor)
	if len(targetNodes) < 2 {
		return ErrInvalidNodeConfig
	}
	var previous *model.ObjectRecord
	current, err := s.repo.FindByOwnerAndKey(ctx, ownerID, objectKey)
	if err == nil {
		previous = &current
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	storageKey := "_versions/" + uuid.NewString()
	if err := s.repo.Stage(ctx, storageKey, targetNodes); err != nil {
		return err
	}

	if err := s.writeReplicas(ctx, targetNodes, storageKey, contentType, data); err != nil {
		return err
	}

	sum := sha256.Sum256(data)
	record := model.ObjectRecord{
		OwnerUserID: ownerID,
		ObjectKey:   objectKey,
		StorageKey:  storageKey,
		ContentType: contentType,
		SizeBytes:   int64(len(data)),
		ETag:        hex.EncodeToString(sum[:]),
		PrimaryNode: targetNodes[0],
		ReplicaNode: targetNodes[1],
		Nodes:       targetNodes,
	}
	return s.repo.CommitVersion(ctx, record, previous)
}

func (s *Service) Download(ctx context.Context, ownerID, objectKey string) (model.ObjectRecord, io.ReadCloser, error) {
	rec, err := s.repo.FindByOwnerAndKey(ctx, ownerID, objectKey)
	if err != nil {
		return model.ObjectRecord{}, nil, err
	}

	return s.openObject(ctx, rec)
}

func (s *Service) Delete(ctx context.Context, ownerID, objectKey string) error {
	rec, err := s.repo.FindByOwnerAndKey(ctx, ownerID, objectKey)
	if err != nil {
		return err
	}

	return requireObject(s.repo.DeleteVersion(ctx, rec))
}

func (s *Service) Hide(ctx context.Context, ownerID, objectKey, actorID, actorRole, reason string) error {
	return requireObject(s.repo.HideByOwnerAndKey(ctx, ownerID, objectKey, actorID, actorRole, reason))
}

func (s *Service) SetPermission(ctx context.Context, ownerID, objectKey, permission string) error {
	if permission != "public-read" && permission != "private-read" {
		return ErrInvalidPermission
	}
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return ErrInvalidKey
	}
	return requireObject(s.repo.SetPermissionByOwnerAndKey(ctx, ownerID, objectKey, permission))
}

func requireObject(updated bool, err error) error {
	if err == nil && !updated {
		return ErrNotFound
	}
	return err
}

func (s *Service) FileExists(ctx context.Context, ownerID, objectKey string) (bool, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return false, ErrInvalidKey
	}

	_, err := s.repo.FindByOwnerAndKey(ctx, ownerID, objectKey)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

type ObjectPage struct {
	Files  []model.ObjectRecord
	Total  int
	Limit  int
	Offset int
}

func (s *Service) ListOwnerObjectsPage(ctx context.Context, ownerID, permission, visibility, keyQuery string, limit, offset int) (ObjectPage, error) {
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
		return ObjectPage{}, err
	}
	total, err := s.repo.CountByOwnerWithFilters(ctx, ownerID, permission, visibility, keyQuery)
	if err != nil {
		return ObjectPage{}, err
	}

	return ObjectPage{Files: files, Total: total, Limit: limit, Offset: offset}, nil
}
