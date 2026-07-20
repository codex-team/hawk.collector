package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

func setupTestRedis(t *testing.T) (*RedisClient, *miniredis.Miniredis) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mock redis: %v", err)
	}

	client := &RedisClient{
		rdb: goredis.NewClient(&goredis.Options{
			Addr: mr.Addr(),
		}),
		ctx: context.Background(),
	}

	return client, mr
}

func TestCheckRateLimit(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	tests := []struct {
		name         string
		projectID    string
		eventsLimit  int64
		eventsPeriod int64
		setup        func()
		wantAllowed  bool
		wantValue    string
		checkValue   bool
	}{
		{
			name:         "allows when no previous events",
			projectID:    "project1",
			eventsLimit:  10,
			eventsPeriod: 60,
			wantAllowed:  true,
			checkValue:   false,
		},
		{
			name:         "allows when under limit without incrementing",
			projectID:    "project2",
			eventsLimit:  10,
			eventsPeriod: 60,
			setup: func() {
				client.rdb.HSet(client.ctx, "rate_limits", "project2",
					fmt.Sprintf("%d:%d", time.Now().Unix()-30, 5))
			},
			wantAllowed: true,
			wantValue:   fmt.Sprintf("%d:%d", time.Now().Unix()-30, 5),
			checkValue:  true,
		},
		{
			name:         "denies when at limit without changing counter",
			projectID:    "project3",
			eventsLimit:  5,
			eventsPeriod: 60,
			setup: func() {
				client.rdb.HSet(client.ctx, "rate_limits", "project3",
					fmt.Sprintf("%d:%d", time.Now().Unix()-30, 5))
			},
			wantAllowed: false,
			checkValue:  true,
		},
		{
			name:         "clears expired window to zero and allows",
			projectID:    "project4",
			eventsLimit:  5,
			eventsPeriod: 60,
			setup: func() {
				client.rdb.HSet(client.ctx, "rate_limits", "project4",
					fmt.Sprintf("%d:%d", time.Now().Unix()-61, 5))
			},
			wantAllowed: true,
			checkValue:  true,
		},
		{
			name:         "allows all when limit is 0",
			projectID:    "project5",
			eventsLimit:  0,
			eventsPeriod: 60,
			wantAllowed:  true,
			checkValue:   false,
		},
		{
			name:         "clears malformed value to zero and allows",
			projectID:    "project6",
			eventsLimit:  5,
			eventsPeriod: 60,
			setup: func() {
				client.rdb.HSet(client.ctx, "rate_limits", "project6", "broken")
			},
			wantAllowed: true,
			checkValue:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			before, _ := client.rdb.HGet(client.ctx, "rate_limits", tt.projectID).Result()

			allowed, err := client.CheckRateLimit(tt.projectID, tt.eventsLimit, tt.eventsPeriod)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantAllowed, allowed)

			after, _ := client.rdb.HGet(client.ctx, "rate_limits", tt.projectID).Result()
			if !tt.checkValue {
				return
			}

			switch tt.name {
			case "allows when under limit without incrementing", "denies when at limit without changing counter":
				assert.Equal(t, before, after, "counter must not be incremented")
			case "clears expired window to zero and allows", "clears malformed value to zero and allows":
				assert.Regexp(t, `^\d+:0$`, after)
			}
		})
	}
}

// Regression: load() must return all entries from large blocked ID sets.
func TestLoadBlockedIDsLargeSet(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	client.blockedIDsSetName = "DisabledProjectsSet"

	const total = 100
	expected := make([]string, 0, total)
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("%024x", i)
		client.rdb.SAdd(client.ctx, client.blockedIDsSetName, id)
		expected = append(expected, id)
	}

	err := client.load()
	assert.NoError(t, err)

	for _, id := range expected {
		assert.True(t, client.IsBlocked(id), "expected %q to be reported as blocked", id)
	}
	assert.False(t, client.IsBlocked("not-in-set"))
}
