package errorshandler

import (
	"fmt"
	"time"

	log "github.com/codex-team/hawk.collector/pkg/logger"
)

// GenerateTestTimeSeriesData - generates test data for minutely, hourly, and daily time series
// Automatically deletes existing keys before generating new data
// Usage: handler.GenerateTestTimeSeriesData(projectId)
func (handler *Handler) GenerateTestTimeSeriesData(projectId string) error {
	metricType := "events-accepted"
	minutelyKey := getTimeSeriesKey(projectId, metricType, "minutely", true)
	hourlyKey := getTimeSeriesKey(projectId, metricType, "hourly", true)
	dailyKey := getTimeSeriesKey(projectId, metricType, "daily", true)
	projectLogger := log.With("projectId", projectId)

	// Delete existing keys to avoid accumulation
	projectLogger.Info(fmt.Sprintf("Deleting existing test data keys for project %s...", projectId))
	if err := handler.RedisClient.DeleteKey(minutelyKey); err != nil {
		projectLogger.Warn(fmt.Sprintf("Failed to delete minutely key: %v", err))
	}
	if err := handler.RedisClient.DeleteKey(hourlyKey); err != nil {
		projectLogger.Warn(fmt.Sprintf("Failed to delete hourly key: %v", err))
	}
	if err := handler.RedisClient.DeleteKey(dailyKey); err != nil {
		projectLogger.Warn(fmt.Sprintf("Failed to delete daily key: %v", err))
	}

	labels := map[string]string{
		"type":    "error",
		"status":  "test",
		"project": projectId,
	}

	now := time.Now()

	// Minutely data: last 24 hours (1440 minutes)
	projectLogger.Info(fmt.Sprintf("Generating minutely test data for project %s...", projectId))
	minuteStart := now.Add(-24 * time.Hour)
	for t := minuteStart; t.Before(now); t = t.Add(1 * time.Minute) {
		// Hash-based pseudo-random: 0-10 events per minute with realistic peaks/valleys
		hash := (t.Unix() * 2654435761) ^ 0xdeadbeef
		eventsCount := int64((hash % 11))
		// Use the minute timestamp for all events in this minute
		// ON_DUPLICATE SUM will accumulate them
		timestamp := t.UnixNano() / int64(time.Millisecond)
		if eventsCount > 0 {
			if err := handler.RedisClient.TSAdd(minutelyKey, eventsCount, timestamp, labels); err != nil {
				return fmt.Errorf("failed to add minutely test data: %w", err)
			}
		}
	}

	// Hourly data: last 7 days (168 hours)
	projectLogger.Info(fmt.Sprintf("Generating hourly test data for project %s...", projectId))
	hourStart := now.Add(-7 * 24 * time.Hour)
	for t := hourStart; t.Before(now); t = t.Add(1 * time.Hour) {
		// Hash-based pseudo-random: 5-95 events per hour
		hash := (t.Unix() * 2654435761) ^ 0xcafebabe
		eventsCount := int64(5 + (hash % 90))
		// Use the hour timestamp for all events in this hour
		timestamp := t.UnixNano() / int64(time.Millisecond)
		if err := handler.RedisClient.TSAdd(hourlyKey, eventsCount, timestamp, labels); err != nil {
			return fmt.Errorf("failed to add hourly test data: %w", err)
		}
	}

	// Daily data: last 90 days
	projectLogger.Info(fmt.Sprintf("Generating daily test data for project %s...", projectId))
	dayStart := now.Add(-90 * 24 * time.Hour)
	for t := dayStart; t.Before(now); t = t.Add(24 * time.Hour) {
		// Hash-based pseudo-random: 100-1900 events per day
		hash := (t.Unix() * 2654435761) ^ 0xbaadf00d
		eventsCount := int64(100 + (hash % 1800))
		// Use the day timestamp for all events in this day
		timestamp := t.UnixNano() / int64(time.Millisecond)
		if err := handler.RedisClient.TSAdd(dailyKey, eventsCount, timestamp, labels); err != nil {
			return fmt.Errorf("failed to add daily test data: %w", err)
		}
	}

	projectLogger.Info(fmt.Sprintf("Test data generation completed for project %s", projectId))
	return nil
}
