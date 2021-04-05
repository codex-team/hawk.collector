package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	blacklistThreshold   int
}

func New(ctx context.Context, url, pass, blockedIDsSet, blacklistSet, IPsMap, currentMap string, threshold int) *RedisClient {
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
		blacklistThreshold:   threshold,
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
func (r *RedisClient) LoadBlacklist() error {
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
		err := r.updateBlacklist()
		if err != nil {
			continue
		}
		<-time.After(d)
		return nil
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
func (r *RedisClient) updateBlacklist() error {
	r.mx.Lock()
	defer r.mx.Unlock()

	ipAddrs, err := r.rdb.HKeys(r.ctx, r.currentPeriodMapName).Result()
	if err != nil {
		return err
	}

	if len(ipAddrs) > 0 {
		requests, err := r.rdb.HVals(r.ctx, r.currentPeriodMapName).Result()
		if err != nil {
			return err
		}

		for i := 0; i < len(ipAddrs); i++ {
			requestsQty, _ := strconv.Atoi(requests[i])
			if requestsQty >= r.blacklistThreshold {
				_, err = r.rdb.SAdd(r.ctx, r.blacklistSetName, ipAddrs[i]).Result()
				if err != nil {
					return err
				}
			}
		}
		cmdResult := r.rdb.Del(r.ctx, r.currentPeriodMapName)
		if cmdResult.Err() != nil {
			return cmdResult.Err()
		}
	}

	ips, _, err := r.rdb.SScan(r.ctx, r.blacklistSetName, 0, "", 0).Result()
	if err != nil {
		return err
	}
	r.blacklistIPs = ips

	return nil
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
	for _, blockedIP := range r.blacklistIPs {
		if ip == blockedIP {
			return true
		}
	}
	return false
}
