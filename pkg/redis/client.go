package redis

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/go-redis/redis/v8"
)

var ErrWrongKey = errors.New("key doesn't exist")

type RedisClient struct {
	mx      sync.Mutex
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

// LoadSet loads list of blocked IDs from Redis with retries.
func (r *RedisClient) LoadSet() error {
	be := backoff.NewExponentialBackOff()
	be.MaxElapsedTime = 3 * time.Minute
	be.InitialInterval = 1 * time.Second
	be.Multiplier = 2
	be.MaxInterval = 30 * time.Second

	b := backoff.WithContext(be, context.Background())
	for {
		d := b.NextBackOff()
		if d == backoff.Stop {
			return fmt.Errorf("failed to connect")
		}
		//nolint:gosimple
		select {
		case <-time.After(d):
			err := r.scan()
			if err != nil {
				continue
			}
			return nil
		}
	}
}

// scan loads list of blocked IDs from Redis.
func (r *RedisClient) scan() error {
	r.mx.Lock()
	defer r.mx.Unlock()
	keys, _, err := r.rdb.SScan(r.ctx, r.setName, 0, "", 0).Result()
	if err != nil {
		return err
	}
	r.data = keys
	return nil
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
