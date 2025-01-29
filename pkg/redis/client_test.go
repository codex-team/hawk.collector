package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

func setupTestRedis(t *testing.T) (*RedisClient, *miniredis.Miniredis) {
	// Create a mock Redis server
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mock redis: %v", err)
	}

	// Create Redis client connected to mock server
	client := &RedisClient{
		rdb: redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		}),
		ctx: context.Background(),
	}

	return client, mr
}

func TestUpdateRateLimit(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	tests := []struct {
		name         string
		projectID    string
		eventsLimit  int64
		eventsPeriod int64
		setup        func()
		calls        int
		wantAllowed  bool
		wantErr      bool
	}{
		{
			name:         "should allow when no previous events",
			projectID:    "project1",
			eventsLimit:  10,
			eventsPeriod: 60,
			calls:        1,
			wantAllowed:  true,
			wantErr:      false,
		},
		{
			name:         "should allow when under limit",
			projectID:    "project2",
			eventsLimit:  10,
			eventsPeriod: 60,
			setup: func() {
				client.rdb.HSet(client.ctx, "rate_limits", "project2",
					fmt.Sprintf("%d:%d", time.Now().Unix()-30, 5))
			},
			calls:       1,
			wantAllowed: true,
			wantErr:     false,
		},
		{
			name:         "should deny when at limit",
			projectID:    "project3",
			eventsLimit:  5,
			eventsPeriod: 60,
			setup: func() {
				client.rdb.HSet(client.ctx, "rate_limits", "project3",
					fmt.Sprintf("%d:%d", time.Now().Unix()-30, 5))
			},
			calls:       1,
			wantAllowed: false,
			wantErr:     false,
		},
		{
			name:         "should reset count after period expires",
			projectID:    "project4",
			eventsLimit:  5,
			eventsPeriod: 60,
			setup: func() {
				client.rdb.HSet(client.ctx, "rate_limits", "project4",
					fmt.Sprintf("%d:%d", time.Now().Unix()-61, 5))
			},
			calls:       1,
			wantAllowed: true,
			wantErr:     false,
		},
		{
			name:         "should allow all when limit is 0",
			projectID:    "project5",
			eventsLimit:  0,
			eventsPeriod: 60,
			calls:        5,
			wantAllowed:  true,
			wantErr:      false,
		},
		{
			name:         "should handle multiple calls up to limit",
			projectID:    "project6",
			eventsLimit:  3,
			eventsPeriod: 60,
			calls:        4,
			wantAllowed:  false, // Last call should be denied
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run setup if provided
			if tt.setup != nil {
				tt.setup()
			}

			var lastAllowed bool
			var lastErr error

			// Make the specified number of calls
			for i := 0; i < tt.calls; i++ {
				lastAllowed, lastErr = client.UpdateRateLimit(tt.projectID, tt.eventsLimit, tt.eventsPeriod)
			}

			if tt.wantErr {
				assert.Error(t, lastErr)
			} else {
				assert.NoError(t, lastErr)
			}
			assert.Equal(t, tt.wantAllowed, lastAllowed)
		})
	}
}

func TestUpdateRateLimitConcurrent(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	const (
		projectID       = "concurrent-project"
		eventsLimit     = 90
		eventsPeriod    = 60
		goroutines      = 10
		callsPerRoutine = 20
	)

	var rejectedCount int = 0

	done := make(chan bool)

	// Launch multiple goroutines to test concurrent access
	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < callsPerRoutine; j++ {
				allowed, err := client.UpdateRateLimit(projectID, eventsLimit, eventsPeriod)
				assert.NoError(t, err)
				if !allowed {
					rejectedCount++
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Verify the total number of successful updates doesn't exceed the limit
	val, err := client.rdb.HGet(client.ctx, "rate_limits", projectID).Result()
	assert.NoError(t, err)
	assert.NotEmpty(t, val)

	// The total count should not exceed the events limit
	count := 0
	_, err = fmt.Sscanf(val, "%d:%d", &count, &count)
	assert.NoError(t, err)
	assert.Equal(t, count, eventsLimit)
	assert.Equal(t, rejectedCount, goroutines*callsPerRoutine-eventsLimit)
	t.Logf("count: %d", count)
	t.Logf("rejectedCount: %d", rejectedCount)
}
