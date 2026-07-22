package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewRedisClient(addr, password string, db int) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

func (c *Client) GetClient() *redis.Client {
	return c.rdb
}

func (c *Client) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("idempotency:lock:%s", key)
	ok, err := c.rdb.SetNX(ctx, lockKey, "locked", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire redis lock: %w", err)
	}
	return ok, nil
}

func (c *Client) ReleaseLock(ctx context.Context, key string) error {
	lockKey := fmt.Sprintf("idempotency:lock:%s", key)
	return c.rdb.Del(ctx, lockKey).Err()
}

func (c *Client) SetResponseCache(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	valKey := fmt.Sprintf("idempotency:val:%s", key)
	return c.rdb.Set(ctx, valKey, value, ttl).Err()
}

func (c *Client) GetResponseCache(ctx context.Context, key string) ([]byte, error) {
	valKey := fmt.Sprintf("idempotency:val:%s", key)
	val, err := c.rdb.Get(ctx, valKey).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get redis cache: %w", err)
	}
	return val, nil
}
