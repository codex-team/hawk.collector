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
	mx                   sync.RWMutex
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
	attempt := 0
	for {
		d := b.NextBackOff()
		if d == backoff.Stop {
			log.Errorf("LoadBlockedIDs gave up after %d attempts (key=%q)", attempt, r.blockedIDsSetName)
			return fmt.Errorf("failed to connect")
		}
		<-time.After(d)
		attempt++
		err := r.load()
		if err != nil {
			log.Warnf("LoadBlockedIDs attempt %d failed (key=%q, redis_available=%t): %s",
				attempt, r.blockedIDsSetName, r.CheckAvailability(), err)
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
	exists, existsErr := r.rdb.Exists(r.ctx, r.blockedIDsSetName).Result()
	if existsErr != nil {
		log.Errorf("Failed to check existence of blocked IDs set %q: %s", r.blockedIDsSetName, existsErr)
	} else if exists == 0 {
		log.Warnf("Blocked IDs set %q does not exist in Redis", r.blockedIDsSetName)
	}

	cardinality, cardErr := r.rdb.SCard(r.ctx, r.blockedIDsSetName).Result()
	if cardErr != nil {
		log.Errorf("Failed to SCARD blocked IDs set %q: %s", r.blockedIDsSetName, cardErr)
	}

	keys, err := r.rdb.SMembers(r.ctx, r.blockedIDsSetName).Result()
	if err != nil {
		log.Errorf("Failed to SMembers blocked IDs set %q: %s", r.blockedIDsSetName, err)
		return err
	}

	r.mx.Lock()
	prevCount := len(r.blockedIDs)
	r.blockedIDs = keys
	r.mx.Unlock()

	if len(keys) != prevCount {
		if cardErr == nil {
			log.Infof("Banned projects list updated %d -> %d (key=%q, scard=%d)", prevCount, len(keys), r.blockedIDsSetName, cardinality)
		} else {
			log.Infof("Banned projects list updated %d -> %d (key=%q, scard_err=%q)", prevCount, len(keys), r.blockedIDsSetName, cardErr)
		}
	}
	if cardErr == nil && int64(len(keys)) != cardinality {
		log.Warnf("SMembers returned %d entries but SCARD reports %d for key %q (concurrent modification?)", len(keys), cardinality, r.blockedIDsSetName)
	}
	if len(keys) == 0 && prevCount > 0 {
		log.Warnf("Blocked projects list is now empty (was %d). Key %q may have been cleared", prevCount, r.blockedIDsSetName)
	}

	const sampleSize = 5
	if len(keys) == 0 {
		log.Debugf("Loaded blocked project IDs from %q (count=0)", r.blockedIDsSetName)
	} else {
		sample := keys
		if len(sample) > sampleSize {
			sample = sample[:sampleSize]
		}
		log.Debugf("Loaded blocked project IDs from %q (count=%d, sample=%v)", r.blockedIDsSetName, len(keys), sample)
	}

	return nil
}

// updateBlacklist loads IPs blacklist and resets current period map.
func (r *RedisClient) updateBlacklist() ([]string, []string, error) {
	ipAddrs, err := r.rdb.HKeys(r.ctx, r.currentPeriodMapName).Result()
	if err != nil {
		return nil, nil, err
	}

	ips, err := r.rdb.SMembers(r.ctx, r.blacklistSetName).Result()
	if err != nil {
		return nil, nil, err
	}

	r.mx.Lock()
	r.blacklistIPs = ips
	r.mx.Unlock()

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
	r.mx.RLock()
	defer r.mx.RUnlock()
	for _, id := range r.blockedIDs {
		if id == val {
			log.Debugf("IsBlocked: project %q matched in cache (size=%d)", val, len(r.blockedIDs))
			return true
		}
	}
	log.Tracef("IsBlocked: project %q not in cache (size=%d, key=%q)", val, len(r.blockedIDs), r.blockedIDsSetName)
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
	r.mx.RLock()
	defer r.mx.RUnlock()
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

	labelArgs := []interface{}{"LABELS"}
	for k, v := range labels {
		labelArgs = append(labelArgs, k, v)
	}

	args := []interface{}{key}
	if retention > 0 {
		args = append(args, "RETENTION", int64(retention/time.Millisecond))
	}
	args = append(args, labelArgs...)

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

// DeleteKey deletes a key from Redis
func (r *RedisClient) DeleteKey(key string) error {
	res := r.rdb.Del(r.ctx, key)
	return res.Err()
}

// TSAdd adds a sample to a time series
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

	args := []interface{}{key, timestamp, value, "ON_DUPLICATE", "SUM"}
	args = append(args, labelArgs...)

	cmdArgs := append([]interface{}{"TS.ADD"}, args...)
	res := r.rdb.Do(r.ctx, cmdArgs...)
	return res.Err()
}

// SafeTSAdd ensures that a TS key exists and adds a sample safely.
// timestamp is the bucket start time in milliseconds.
func (r *RedisClient) SafeTSAdd(
	key string,
	value int64,
	labels map[string]string,
	retention time.Duration,
	timestamp int64,
) error {
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
