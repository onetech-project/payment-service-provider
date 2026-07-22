package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type ClientKeyCache struct {
	client *Client
}

func NewClientKeyCache(client *Client) *ClientKeyCache {
	return &ClientKeyCache{client: client}
}

func (c *ClientKeyCache) GetClientPublicKey(ctx context.Context, clientID string) (string, error) {
	key := fmt.Sprintf("client:key:%s", clientID)
	val, err := c.client.GetClient().Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get client key from redis cache: %w", err)
	}
	return val, nil
}

func (c *ClientKeyCache) SetClientPublicKey(ctx context.Context, clientID, pubKeyPEM string) error {
	key := fmt.Sprintf("client:key:%s", clientID)
	return c.client.GetClient().Set(ctx, key, pubKeyPEM, 1*time.Hour).Err()
}
