package errorshandler

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

// GenerateTestTimeSeriesData - generates test data for minutely, hourly, and daily time series
// Automatically deletes existing keys before generating new data
// Usage: handler.GenerateTestTimeSeriesData(projectId)
func (handler *Handler) GenerateTestTimeSeriesData(projectId string) error {
	metricType := "events-accepted"
	minutelyKey := getTimeSeriesKey(projectId, metricType, "minutely")
	hourlyKey := getTimeSeriesKey(projectId, metricType, "hourly")
	dailyKey := getTimeSeriesKey(projectId, metricType, "daily")

	// Delete existing keys to avoid accumulation
	log.Infof("Deleting existing test data keys for project %s...", projectId)
	if err := handler.RedisClient.DeleteKey(minutelyKey); err != nil {
		log.Warnf("Failed to delete minutely key: %v", err)
	}
	if err := handler.RedisClient.DeleteKey(hourlyKey); err != nil {
		log.Warnf("Failed to delete hourly key: %v", err)
	}
	if err := handler.RedisClient.DeleteKey(dailyKey); err != nil {
		log.Warnf("Failed to delete daily key: %v", err)
	}

	labels := map[string]string{
		"type":    "error",
		"status":  "test",
		"project": projectId,
	}

	now := time.Now()

	// Minutely data: last 24 hours (1440 minutes)
	log.Infof("Generating minutely test data for project %s...", projectId)
	minuteStart := now.Add(-24 * time.Hour)
	for t := minuteStart; t.Before(now); t = t.Add(1 * time.Minute) {
		// Hash-based pseudo-random: 0-10 events per minute with realistic peaks/valleys
		hash := (t.Unix() * 2654435761) ^ 0xdeadbeef
		eventsCount := int64((hash % 11))
		for i := int64(0); i < eventsCount; i++ {
			timestamp := t.UnixNano()/int64(time.Millisecond) + i*100
			if err := handler.RedisClient.TSIncrBy(minutelyKey, 1, timestamp, labels); err != nil {
				return fmt.Errorf("failed to add minutely test data: %w", err)
			}
		}
	}

	// Hourly data: last 7 days (168 hours)
	log.Infof("Generating hourly test data for project %s...", projectId)
	hourStart := now.Add(-7 * 24 * time.Hour)
	for t := hourStart; t.Before(now); t = t.Add(1 * time.Hour) {
		// Hash-based pseudo-random: 5-95 events per hour
		hash := (t.Unix() * 2654435761) ^ 0xcafebabe
		eventsCount := int64(5 + (hash % 90))
		for i := int64(0); i < eventsCount; i++ {
			timestamp := t.UnixNano()/int64(time.Millisecond) + i*1000
			if err := handler.RedisClient.TSIncrBy(hourlyKey, 1, timestamp, labels); err != nil {
				return fmt.Errorf("failed to add hourly test data: %w", err)
			}
		}
	}

	// Daily data: last 90 days
	log.Infof("Generating daily test data for project %s...", projectId)
	dayStart := now.Add(-90 * 24 * time.Hour)
	for t := dayStart; t.Before(now); t = t.Add(24 * time.Hour) {
		// Hash-based pseudo-random: 100-1900 events per day
		hash := (t.Unix() * 2654435761) ^ 0xbaadf00d
		eventsCount := int64(100 + (hash % 1800))
		for i := int64(0); i < eventsCount; i++ {
			timestamp := t.UnixNano()/int64(time.Millisecond) + i*10000
			if err := handler.RedisClient.TSIncrBy(dailyKey, 1, timestamp, labels); err != nil {
				return fmt.Errorf("failed to add daily test data: %w", err)
			}
		}
	}

	log.Infof("Test data generation completed for project %s", projectId)
	return nil
}
