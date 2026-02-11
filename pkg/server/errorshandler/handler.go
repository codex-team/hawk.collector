package errorshandler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/codex-team/hawk.collector/pkg/accounts"

	"github.com/codex-team/hawk.collector/pkg/broker"
	log "github.com/codex-team/hawk.collector/pkg/logger"
	"github.com/codex-team/hawk.collector/pkg/redis"
	"github.com/prometheus/client_golang/prometheus"
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
	projectLogger := log.With("projectId", projectId)
	projectLogger.Debug(fmt.Sprintf("Found project with ID %s for integration token %s", projectId, integrationSecret))

	projectLimits, ok := handler.AccountsMongoDBClient.GetProjectLimits(projectId)
	if !ok {
		projectLogger.Warn(fmt.Sprintf("Project %s is not in the projects limits cache", projectId))
	} else {
		projectLogger.Debug(fmt.Sprintf("Project %s limits: %+v", projectId, projectLimits))
	}

	if handler.RedisClient.IsBlocked(projectId) {
		handler.ErrorsBlockedByLimit.Inc()
		handler.recordProjectMetrics(projectId, "events-rate-limited", false)
		return ResponseMessage{402, true, "Project has exceeded the events limit"}
	}

	rateWithinLimit, err := handler.RedisClient.UpdateRateLimit(projectId, projectLimits.EventsLimit, projectLimits.EventsPeriod)
	if err != nil {
		projectLogger.Error(fmt.Sprintf("Failed to update rate limit: %s", err))
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
	projectLogger.Debug(fmt.Sprintf("Send to queue: %s", brokerMessage))
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

// recordProjectMetrics records project metrics to Redis TimeSeries
// metricType can be: "events-accepted", "events-rate-limited", etc.
func (handler *Handler) recordProjectMetrics(projectId, metricType string, isSystemMetric bool) {
	projectLogger := log.With("projectId", projectId)
	minutelyKey := getTimeSeriesKey(projectId, metricType, "minutely", isSystemMetric)
	hourlyKey := getTimeSeriesKey(projectId, metricType, "hourly", isSystemMetric)
	dailyKey := getTimeSeriesKey(projectId, metricType, "daily", isSystemMetric)

	labels := map[string]string{
		"type":    "error",
		"status":  metricType,
		"project": projectId,
	}

	// minutely: store for 24 hours
	// Use TS.ADD with ON_DUPLICATE SUM to accumulate events within the same timestamp
	if err := handler.RedisClient.SafeTSAdd(minutelyKey, 1, labels, 24*time.Hour); err != nil {
		projectLogger.Error(fmt.Sprintf("failed to add minutely TS for %s: %v", metricType, err))
	}

	// hourly: store for 7 days
	if err := handler.RedisClient.SafeTSAdd(hourlyKey, 1, labels, 7*24*time.Hour); err != nil {
		projectLogger.Error(fmt.Sprintf("failed to add hourly TS for %s: %v", metricType, err))
	}

	// daily: store for 90 days
	if err := handler.RedisClient.SafeTSAdd(dailyKey, 1, labels, 90*24*time.Hour); err != nil {
		projectLogger.Error(fmt.Sprintf("failed to add daily TS for %s: %v", metricType, err))
	}
}
