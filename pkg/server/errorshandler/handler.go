package errorshandler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/codex-team/hawk.collector/pkg/accounts"

	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/codex-team/hawk.collector/pkg/redis"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

const DefaultQueueName = "errors/default"

// Handler of error messages
type Handler struct {
	Broker    *broker.Broker
	JwtSecret string

	// Maximum POST body size in bytes for error messages
	MaxErrorCatcherMessageSize int

	ErrorsBlockedByLimit prometheus.Counter
	ErrorsProcessed      prometheus.Counter

	RedisClient           *redis.RedisClient
	AccountsMongoDBClient *accounts.AccountsMongoDBClient

	NonDefaultQueues map[string]bool
}

func (handler *Handler) process(body []byte) ResponseMessage {
	// Check if the body is a valid JSON with the Message structure
	message := CatcherMessage{}
	err := json.Unmarshal(body, &message)
	if err != nil {
		return ResponseMessage{400, true, "Invalid JSON format"}
	}

	if len(message.Payload) == 0 {
		return ResponseMessage{400, true, "Payload is empty"}
	}
	if message.Token == "" {
		return ResponseMessage{400, true, "Token is empty"}
	}
	if message.CatcherType == "" {
		return ResponseMessage{400, true, "CatcherType is empty"}
	}

	integrationSecret, err := accounts.DecodeToken(string(message.Token))
	if err != nil {
		log.Warnf("[release] Token decoding error: %s", err)
		return ResponseMessage{400, true, "Token decoding error"}
	}

	projectId, ok := handler.AccountsMongoDBClient.GetValidToken(integrationSecret)
	if !ok {
		log.Debugf("Token %s is not in the accounts cache", integrationSecret)
		return ResponseMessage{400, true, fmt.Sprintf("Integration token invalid: %s", integrationSecret)}
	}
	log.Debugf("Found project with ID %s for integration token %s", projectId, integrationSecret)

	projectLimits, ok := handler.AccountsMongoDBClient.GetProjectLimits(projectId)
	if !ok {
		log.Warnf("Project %s is not in the projects limits cache", projectId)
	} else {
		log.Debugf("Project %s limits: %+v", projectId, projectLimits)
	}

	if handler.RedisClient.IsBlocked(projectId) {
		handler.ErrorsBlockedByLimit.Inc()
		return ResponseMessage{402, true, "Project has exceeded the events limit"}
	}

	rateWithinLimit, err := handler.RedisClient.UpdateRateLimit(projectId, projectLimits.EventsLimit, projectLimits.EventsPeriod)
	if err != nil {
		log.Errorf("Failed to update rate limit: %s", err)
		return ResponseMessage{402, true, "Failed to update rate limit"}
	}
	if !rateWithinLimit {
		return ResponseMessage{402, true, "Rate limit exceeded"}
	}

	// Validate if message is a valid JSON
	stringPayload := string(message.Payload)
	if !gjson.Valid(stringPayload) {
		return ResponseMessage{400, true, "Invalid payload JSON format"}
	}

	// convert message to JSON format
	messageToSend := BrokerMessage{Timestamp: time.Now().Unix(), ProjectId: projectId, Payload: []byte(stringPayload), CatcherType: message.CatcherType}
	rawMessage, err := json.Marshal(messageToSend)
	if err != nil {
		log.Errorf("Message marshalling error: %v", err)
		return ResponseMessage{400, true, "Cannot encode message to JSON"}
	}

	// send serialized message to a broker
	brokerMessage := broker.Message{Payload: rawMessage, Route: handler.determineQueue(message.CatcherType)}
	log.Debugf("Send to queue: %s", brokerMessage)
	handler.Broker.Chan <- brokerMessage

	// increment processed errors counter
	handler.ErrorsProcessed.Inc()

	// add event to time series (minutely, hourly, and daily)
	minutelyKey := fmt.Sprintf("ts:events:%s:minutely", projectId)
	hourlyKey := fmt.Sprintf("ts:events:%s:hourly", projectId)
	dailyKey := fmt.Sprintf("ts:events:%s:daily", projectId)

	labels := map[string]string{
		"type":    "error",
		"status":  "accepted",
		"project": projectId,
	}

	// minutely: храним 24 часа
	if err := handler.RedisClient.SafeTSAdd(minutelyKey, 1, labels, 24*time.Hour); err != nil {
		log.Errorf("failed to add to minutely TS: %v", err)
	}

	// hourly: store for 7 days
	if err := handler.RedisClient.SafeTSAdd(hourlyKey, 1, labels, 7*24*time.Hour); err != nil {
		log.Errorf("failed to add to hourly TS: %v", err)
	}

	// daily: храним 90 дней
	if err := handler.RedisClient.SafeTSAdd(dailyKey, 1, labels, 90*24*time.Hour); err != nil {
		log.Errorf("failed to add to daily TS: %v", err)
	}

	return ResponseMessage{200, false, "OK"}
}

// determineQueue - determine RabbitMQ route from catcherType
func (handler *Handler) determineQueue(catcherType string) string {
	if _, ok := handler.NonDefaultQueues[catcherType]; ok {
		return catcherType
	}
	return DefaultQueueName
}

// GetQueueCache - construct searching set from array of queue names
func GetQueueCache(nonDefaultQueues []string) map[string]bool {
	cache := make(map[string]bool)
	for _, queue := range nonDefaultQueues {
		cache[fmt.Sprintf("errors/%s", queue)] = true
	}
	return cache
}

// GenerateTestTimeSeriesData - generates test data for minutely, hourly, and daily time series
// This should be called after manually deleting the Redis keys
// Usage: handler.GenerateTestTimeSeriesData(projectId)
func (handler *Handler) GenerateTestTimeSeriesData(projectId string) error {
	minutelyKey := fmt.Sprintf("ts:events:%s:minutely", projectId)
	hourlyKey := fmt.Sprintf("ts:events:%s:hourly", projectId)
	dailyKey := fmt.Sprintf("ts:events:%s:daily", projectId)

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
			if err := handler.RedisClient.TSAdd(minutelyKey, 1, timestamp, labels); err != nil {
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
			if err := handler.RedisClient.TSAdd(hourlyKey, 1, timestamp, labels); err != nil {
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
			if err := handler.RedisClient.TSAdd(dailyKey, 1, timestamp, labels); err != nil {
				return fmt.Errorf("failed to add daily test data: %w", err)
			}
		}
	}

	log.Infof("Test data generation completed for project %s", projectId)
	return nil
}
