package redis

import (
	"context"
	"fmt"
	"time"
)

type TokenSessionStore struct {
	client *Client
}

func NewTokenSessionStore(client *Client) *TokenSessionStore {
	return &TokenSessionStore{client: client}
}

func (s *TokenSessionStore) SaveTokenSession(ctx context.Context, jti, clientID string, ttl time.Duration) error {
	key := fmt.Sprintf("token:session:%s", jti)
	return s.client.GetClient().Set(ctx, key, clientID, ttl).Err()
}

func (s *TokenSessionStore) IsTokenActive(ctx context.Context, jti string) (bool, error) {
	key := fmt.Sprintf("token:session:%s", jti)
	exists, err := s.client.GetClient().Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check token session in redis: %w", err)
	}
	return exists > 0, nil
}

func (s *TokenSessionStore) RevokeTokenSession(ctx context.Context, jti string) error {
	key := fmt.Sprintf("token:session:%s", jti)
	return s.client.GetClient().Del(ctx, key).Err()
}
