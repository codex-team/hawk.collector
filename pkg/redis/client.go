package redis

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

var ErrWrongKey = errors.New("key doesn't exist")

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
