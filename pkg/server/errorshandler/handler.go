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

	ErrorsBlockedByLimit           prometheus.Counter
	ErrorsProcessed                prometheus.Counter
	ErrorsRejectedMessageTooLarge  prometheus.Counter

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
		handler.recordProjectMetrics(projectId, "events-rate-limited", false)
		return ResponseMessage{402, true, "Project has exceeded the events limit"}
	}

	rateWithinLimit, err := handler.RedisClient.UpdateRateLimit(projectId, projectLimits.EventsLimit, projectLimits.EventsPeriod)
	if err != nil {
		log.Errorf("Failed to update rate limit: %s", err)
		return ResponseMessage{402, true, "Failed to update rate limit"}
	}
	if !rateWithinLimit {
		handler.recordProjectMetrics(projectId, "events-rate-limited", false)
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

	// record project metrics
	handler.recordProjectMetrics(projectId, "events-accepted", true)

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

// getTimeSeriesKey generates a Redis TimeSeries key for project metrics
func getTimeSeriesKey(projectId, metricType, granularity string, isSystemMetric bool) string {
	// flag determines which counter would be incremented
	if isSystemMetric {
		// ts:collector-project-%s:%s:%s could be used in admin
		return fmt.Sprintf("ts:collector-project-%s:%s:%s", metricType, projectId, granularity)
	}

	// ts:project-%s:%s:%s is used in api for chart retrieving
	return fmt.Sprintf("ts:project-%s:%s:%s", metricType, projectId, granularity)
}

// bucketTimestampMs returns the current UTC time truncated to the start of the
// given granularity bucket, in milliseconds. Truncating ensures that all events
// within the same bucket share one timestamp so ON_DUPLICATE SUM accumulates
// them into a single sample instead of creating a separate sample per event.
func bucketTimestampMs(granularity string) int64 {
	now := time.Now().UTC()
	var t time.Time
	switch granularity {
	case "hourly":
		t = now.Truncate(time.Hour)
	case "daily":
		t = now.Truncate(24 * time.Hour)
	default: // minutely
		t = now.Truncate(time.Minute)
	}
	return t.UnixNano() / int64(time.Millisecond)
}

// recordProjectMetrics records project metrics to Redis TimeSeries
// metricType can be: "events-accepted", "events-rate-limited", etc.
func (handler *Handler) recordProjectMetrics(projectId, metricType string, isSystemMetric bool) {
	minutelyKey := getTimeSeriesKey(projectId, metricType, "minutely", isSystemMetric)
	hourlyKey := getTimeSeriesKey(projectId, metricType, "hourly", isSystemMetric)
	dailyKey := getTimeSeriesKey(projectId, metricType, "daily", isSystemMetric)

	labels := map[string]string{
		"type":    "error",
		"status":  metricType,
		"project": projectId,
	}

	// minutely: store for 24 hours
	if err := handler.RedisClient.SafeTSAdd(minutelyKey, 1, labels, 24*time.Hour, bucketTimestampMs("minutely")); err != nil {
		log.Errorf("failed to add minutely TS for %s: %v", metricType, err)
	}

	// hourly: store for 7 days
	if err := handler.RedisClient.SafeTSAdd(hourlyKey, 1, labels, 7*24*time.Hour, bucketTimestampMs("hourly")); err != nil {
		log.Errorf("failed to add hourly TS for %s: %v", metricType, err)
	}

	// daily: store for 90 days
	if err := handler.RedisClient.SafeTSAdd(dailyKey, 1, labels, 90*24*time.Hour, bucketTimestampMs("daily")); err != nil {
		log.Errorf("failed to add daily TS for %s: %v", metricType, err)
	}
}
