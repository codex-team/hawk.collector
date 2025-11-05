package redis

import (
	"context"
	"fmt"
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

// UpdateRateLimit checks and updates the rate limit for a project using a Lua script
func (r *RedisClient) UpdateRateLimit(projectID string, eventsLimit int64, eventsPeriod int64) (bool, error) {
	// If eventsLimit is 0, we don't need to update the rate limit
	if eventsLimit == 0 {
		return true, nil
	}

	// Lua script for atomic rate limit check and update
	script := `
		local key = KEYS[1]
		local field = ARGV[1]
		local now = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local period = tonumber(ARGV[4])

		local current = redis.call('HGET', key, field)
		if not current then
			-- No existing record, create new window
			redis.call('HSET', key, field, now .. ':1')
			return 1
		end

		local timestamp, count = string.match(current, '(%d+):(%d+)')
		timestamp = tonumber(timestamp)
		count = tonumber(count)

		-- Check if we're in a new time window
		if now - timestamp >= period then
			-- Reset for new window
			redis.call('HSET', key, field, now .. ':1')
			return 1
		end

		-- Check if incrementing would exceed limit
		if count + 1 > limit then
			return 0
		end

		-- Increment counter
		redis.call('HSET', key, field, timestamp .. ':' .. (count + 1))
		return 1
	`

	// Run the script
	result, err := r.rdb.Eval(
		r.ctx,
		script,
		[]string{"rate_limits"}, // KEYS
		projectID,               // field (ARGV[1])
		time.Now().Unix(),       // now (ARGV[2])
		eventsLimit,             // limit (ARGV[3])
		eventsPeriod,            // period (ARGV[4])
	).Result()

	if err != nil {
		return false, fmt.Errorf("failed to execute rate limit script: %w", err)
	}

	// Script returns 1 if rate limit is not exceeded, 0 if it is
	return result.(int64) == 1, nil
}

// TSCreateIfNotExists creates a RedisTimeSeries key if it doesn't exist.
// It sets optional retention policy and attaches labels.
func (r *RedisClient) TSCreateIfNotExists(
	key string,
	labels map[string]string,
	retention time.Duration,
) error {
	exists, err := r.rdb.Exists(r.ctx, key).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return nil // already exists
	}

	args := []interface{}{key}
	if retention > 0 {
		args = append(args, "RETENTION", int64(retention/time.Millisecond))
	}
	// Allow duplicate timestamps by summing their values
	args = append(args, "DUPLICATE_POLICY", "SUM")

	// Add labels at the end
	args = append(args, "LABELS")
	for k, v := range labels {
		args = append(args, k, v)
	}

	res := r.rdb.Do(r.ctx, append([]interface{}{"TS.CREATE"}, args...)...)
	return res.Err()
}

// TSIncrBy increments a RedisTimeSeries key with labels and timestamp.
// Uses RedisTimeSeries command TS.INCRBY.
func (r *RedisClient) TSIncrBy(
	key string,
	value int64,
	timestamp int64,
	labels map[string]string,
) error {
	// Prepare label arguments
	labelArgs := []interface{}{"LABELS"}
	for k, v := range labels {
		labelArgs = append(labelArgs, k, v)
	}

	if timestamp == 0 {
		timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	}

	args := []interface{}{key, value, "TIMESTAMP", timestamp}
	args = append(args, labelArgs...)

	cmdArgs := append([]interface{}{"TS.INCRBY"}, args...)
	res := r.rdb.Do(r.ctx, cmdArgs...)
	return res.Err()
}

// SafeTSIncrBy ensures that a TS key exists and increments it safely.
// Automatically creates the time series if it doesn't exist.
func (r *RedisClient) SafeTSIncrBy(
	key string,
	value int64,
	labels map[string]string,
	retention time.Duration,
) error {
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)

	err := r.TSIncrBy(key, value, timestamp, labels)
	if err != nil && strings.Contains(err.Error(), "TSDB: key does not exist") {
		log.Warnf("TS key %s does not exist, creating it...", key)
		if err2 := r.TSCreateIfNotExists(key, labels, retention); err2 != nil {
			return fmt.Errorf("failed to create TS: %w", err2)
		}
		return r.TSIncrBy(key, value, timestamp, labels)
	}
	return err
}

// TSAdd adds a sample to a RedisTimeSeries key with labels and timestamp.
// Uses RedisTimeSeries command TS.ADD.
func (r *RedisClient) TSAdd(
	key string,
	value int64,
	timestamp int64,
	labels map[string]string,
) error {
	// Prepare label arguments
	labelArgs := []interface{}{"LABELS"}
	for k, v := range labels {
		labelArgs = append(labelArgs, k, v)
	}

	if timestamp == 0 {
		timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	}

	args := []interface{}{key, timestamp, value}
	args = append(args, "ON_DUPLICATE", "SUM")
	args = append(args, labelArgs...)

	cmdArgs := append([]interface{}{"TS.ADD"}, args...)
	res := r.rdb.Do(r.ctx, cmdArgs...)
	return res.Err()
}

// SafeTSAdd ensures that a TS key exists and adds a sample safely.
// Automatically creates the time series if it doesn't exist.
func (r *RedisClient) SafeTSAdd(
	key string,
	value int64,
	labels map[string]string,
	retention time.Duration,
) error {
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)

	err := r.TSAdd(key, value, timestamp, labels)
	if err != nil && strings.Contains(err.Error(), "TSDB: key does not exist") {
		log.Warnf("TS key %s does not exist, creating it...", key)
		if err2 := r.TSCreateIfNotExists(key, labels, retention); err2 != nil {
			return fmt.Errorf("failed to create TS: %w", err2)
		}
		return r.TSAdd(key, value, timestamp, labels)
	}
	return err
}
