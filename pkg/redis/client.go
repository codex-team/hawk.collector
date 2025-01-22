package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

type RedisClient struct {
	mx                   sync.Mutex
	rdb                  *redis.Client
	blockedIDsSetName    string
	allIPsMapName        string
	currentPeriodMapName string
	blacklistSetName     string
	ctx                  context.Context
	blockedIDs           []string
	blacklistIPs         []string
}

func New(ctx context.Context, url, pass, blockedIDsSet, blacklistSet, IPsMap, currentMap string) *RedisClient {
	return &RedisClient{
		rdb: redis.NewClient(&redis.Options{
			Addr:     url,
			Password: pass,
			DB:       0,
		}),
		ctx:                  ctx,
		blockedIDsSetName:    blockedIDsSet,
		allIPsMapName:        IPsMap,
		currentPeriodMapName: currentMap,
		blacklistSetName:     blacklistSet,
	}
}

// LoadBlockedIDs loads list of blocked IDs from Redis with retries.
func (r *RedisClient) LoadBlockedIDs() error {
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
		<-time.After(d)
		err := r.load()
		if err != nil {
			continue
		}
		return nil
	}
}

// LoadBlacklist loads list of blocked IP addresses from Redis.
func (r *RedisClient) LoadBlacklist() ([]string, []string, error) {
	be := backoff.NewExponentialBackOff()
	be.MaxElapsedTime = 3 * time.Minute
	be.InitialInterval = 1 * time.Second
	be.Multiplier = 2
	be.MaxInterval = 30 * time.Second

	b := backoff.WithContext(be, context.Background())
	for {
		d := b.NextBackOff()
		if d == backoff.Stop {
			return nil, nil, fmt.Errorf("failed to connect")
		}
		addrs, reqs, err := r.updateBlacklist()
		if err != nil {
			continue
		}
		<-time.After(d)
		return addrs, reqs, nil
	}
}

// loads list with provided name from Redis.
func (r *RedisClient) load() error {
	r.mx.Lock()
	defer r.mx.Unlock()
	keys, _, err := r.rdb.SScan(r.ctx, r.blockedIDsSetName, 0, "", 0).Result()
	if err != nil {
		return err
	}
	if len(keys) != len(r.blockedIDs) {
		log.Debugf("Banned projects list has been updated. Number of projects changed %d -> %d", len(r.blockedIDs), len(keys))
	}
	r.blockedIDs = keys

	return nil
}

// updateBlacklist loads IPs blacklist and resets current period map.
func (r *RedisClient) updateBlacklist() ([]string, []string, error) {
	r.mx.Lock()
	defer r.mx.Unlock()

	ipAddrs, err := r.rdb.HKeys(r.ctx, r.currentPeriodMapName).Result()
	if err != nil {
		return nil, nil, err
	}

	ips, _, err := r.rdb.SScan(r.ctx, r.blacklistSetName, 0, "", 0).Result()
	if err != nil {
		return nil, nil, err
	}
	r.blacklistIPs = ips

	if len(ipAddrs) > 0 {
		requests, err := r.rdb.HVals(r.ctx, r.currentPeriodMapName).Result()
		if err != nil {
			return nil, nil, err
		}

		cmdResult := r.rdb.Del(r.ctx, r.currentPeriodMapName)
		if cmdResult.Err() != nil {
			return nil, nil, cmdResult.Err()
		}

		return ipAddrs, requests, nil
	}

	return nil, nil, nil
}

// IsBlocked checks if the provided ID is blocked.
func (r *RedisClient) IsBlocked(val string) bool {
	for _, id := range r.blockedIDs {
		if id == val {
			return true
		}
	}
	return false
}

// IncrementIP increments the number of requests sent from provided IP.
func (r *RedisClient) IncrementIP(ip string) error {
	cmdResult := r.rdb.HIncrBy(r.ctx, r.currentPeriodMapName, ip, 1)
	if cmdResult.Err() != nil {
		return cmdResult.Err()
	}
	cmdResult = r.rdb.HIncrBy(r.ctx, r.allIPsMapName, ip, 1)
	return cmdResult.Err()
}

// CheckBlacklist checks if the provided IP is in blacklist.
func (r *RedisClient) CheckBlacklist(ip string) bool {
	if len(r.blacklistIPs) == 0 {
		return false
	}
	for _, blockedIP := range r.blacklistIPs {
		if ip == blockedIP {
			return true
		}
	}
	return false
}

// CheckAvailability checks if redis is available
func (r *RedisClient) CheckAvailability() bool {
	pong, err := r.rdb.Ping(r.ctx).Result()
	if err != nil {
		return false
	}
	return pong == "PONG"
}

// UpdateRateLimit checks and updates the rate limit for a project
func (r *RedisClient) UpdateRateLimit(projectID string, eventsLimit int64, eventsPeriod int64) (bool, error) {
	// If eventsLimit is 0, we don't need to update the rate limit
	if eventsLimit == 0 {
		return true, nil
	}

	// Key format: "project_id" -> "timestamp:count"
	now := time.Now().Unix()

	// Get current window data
	val, err := r.rdb.HGet(r.ctx, "rate_limits", projectID).Result()
	if err != nil && err != redis.Nil {
		return false, fmt.Errorf("failed to get rate limit data: %w", err)
	}

	var timestamp int64
	var count int64

	if val != "" {
		// Parse existing "timestamp:count" value
		parts := strings.Split(val, ":")
		timestamp, _ = strconv.ParseInt(parts[0], 10, 64)
		count, _ = strconv.ParseInt(parts[1], 10, 64)

		// Reset count if we're in a new window
		if now-timestamp >= eventsPeriod {
			count = 0
			timestamp = now
		}
	} else {
		// Initialize new window
		timestamp = now
		count = 0
	}

	// Check if incrementing would exceed limit
	if count+1 > eventsLimit {
		return false, nil
	}

	// Update the counter
	newVal := fmt.Sprintf("%d:%d", timestamp, count+1)
	err = r.rdb.HSet(r.ctx, "rate_limits", projectID, newVal).Err()
	if err != nil {
		return false, fmt.Errorf("failed to update rate limit: %w", err)
	}

	return true, nil
}
