package service

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"cloudbin-object-api/internal/object/model"

	"github.com/google/uuid"
)

type shareLinkPayload struct {
	OwnerID   string    `json:"owner_id"`
	ObjectKey string    `json:"object_key"`
	ExpiresAt time.Time `json:"expires_at"`
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
		return model.ObjectRecord{}, nil, err
	}
	if rec.Visibility != "visible" {
		return model.ObjectRecord{}, nil, ErrForbidden
	}

	return s.openObject(ctx, rec)
}

func (s *Service) DownloadPublic(ctx context.Context, ownerID, objectKey string) (model.ObjectRecord, io.ReadCloser, error) {
	rec, err := s.repo.FindByOwnerAndKey(ctx, ownerID, objectKey)
	if err != nil {
		return model.ObjectRecord{}, nil, err
	}
	if rec.Permission != "public-read" || rec.Visibility != "visible" {
		return model.ObjectRecord{}, nil, ErrForbidden
	}

	return s.openObject(ctx, rec)
}
