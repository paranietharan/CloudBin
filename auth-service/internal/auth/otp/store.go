package otp

import (
	"context"
	"encoding/json"
	"time"

	"cloudbin-auth-service/internal/auth/model"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	redis *redis.Client
	ttl   time.Duration
}

func NewStore(redisClient *redis.Client, ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Store{redis: redisClient, ttl: ttl}
}

func (s *Store) Save(ctx context.Context, tempToken string, payload model.OTPPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.redis.Set(ctx, s.key(tempToken), raw, s.ttl).Err()
}

func (s *Store) Get(ctx context.Context, tempToken string) (model.OTPPayload, error) {
	raw, err := s.redis.Get(ctx, s.key(tempToken)).Result()
	if err != nil {
		return model.OTPPayload{}, err
	}
	var payload model.OTPPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return model.OTPPayload{}, err
	}
	return payload, nil
}

func (s *Store) Delete(ctx context.Context, tempToken string) error {
	return s.redis.Del(ctx, s.key(tempToken)).Err()
}

func (s *Store) key(tempToken string) string {
	return "auth:otp:" + tempToken
}
