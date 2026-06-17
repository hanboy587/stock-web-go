package cache

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb     *redis.Client
	enabled bool
}

func Connect(ctx context.Context, addr string) *Client {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("redis disabled: %v", err)
		return &Client{}
	}
	return &Client{rdb: rdb, enabled: true}
}

func (c *Client) Close() {
	if c != nil && c.rdb != nil {
		_ = c.rdb.Close()
	}
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || !c.enabled {
		return redis.Nil
	}
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) GetJSON(ctx context.Context, key string, dest any) bool {
	if c == nil || !c.enabled {
		return false
	}
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(raw, dest) == nil
}

func (c *Client) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	if c == nil || !c.enabled {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, key, raw, ttl).Err()
}

func (c *Client) Delete(ctx context.Context, keys ...string) {
	if c == nil || !c.enabled || len(keys) == 0 {
		return
	}
	_ = c.rdb.Del(ctx, keys...).Err()
}
