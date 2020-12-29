package redis

import (
	"context"
	"errors"

	"github.com/go-redis/redis/v8"
)

var ErrWrongKey = errors.New("key doesn't exist")

type RedisClient struct {
	rdb     *redis.Client
	setName string
	ctx     context.Context
	data    []string
}

func New(ctx context.Context, url, pass, set string) *RedisClient {
	return &RedisClient{
		rdb: redis.NewClient(&redis.Options{
			Addr:     url,
			Password: pass,
			DB:       0,
		}),
		ctx:     ctx,
		setName: set,
	}
}

// LoadSet loads from Redis data about blocked IDs.
func (r *RedisClient) LoadSet() error {
	keys, _, err := r.rdb.SScan(r.ctx, r.setName, 0, "", 0).Result()
	r.data = keys
	return err
}

// IsBlocked checks if some ID is blocked.
func (r *RedisClient) IsBlocked(val string) bool {
	for _, id := range r.data {
		if id == val {
			return true
		}
	}
	return false
}
