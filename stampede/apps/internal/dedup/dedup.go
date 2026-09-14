package dedup

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Checker struct {
	redis *redis.Client
	ttl   time.Duration
}

func NewChecker(client *redis.Client, ttl time.Duration) *Checker {
	return &Checker{redis: client, ttl: ttl}
}

func (c *Checker) Seen(ctx context.Context, key string) (bool, error) {
	notSeenBefore, err := c.redis.SetNX(ctx, key, "1", c.ttl).Result()
	if err != nil {
		return false, err
	}

	return !notSeenBefore, nil
}

func (c *Checker) Unclaim(ctx context.Context, key string) error {
	return c.redis.Del(ctx, key).Err()
}
