package performancehandler

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

const PerformanceQueueName = "performance"
const CatcherType = "performance"

// Handler of performance messages
type Handler struct {
	PerformanceBroker *broker.Broker

	// Maximum POST body size in bytes for performance messages
	MaxPerformanceCatcherMessageSize int

	PerformanceBlockedByLimit prometheus.Counter
	PerformanceProcessed      prometheus.Counter

	RedisClient           *redis.RedisClient
	AccountsMongoDBClient *accounts.AccountsMongoDBClient
}

func (handler *Handler) Process(body []byte) ResponseMessage {
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

	projectId, ok := handler.AccountsMongoDBClient.ValidTokens[integrationSecret]
	if !ok {
		log.Debugf("Token %s is not in the accounts cache", integrationSecret)
		return ResponseMessage{400, true, fmt.Sprintf("Integration token invalid: %s", integrationSecret)}
	}
	log.Debugf("Found project with ID %s for integration token %s", projectId, integrationSecret)

	projectLimits, ok := handler.AccountsMongoDBClient.ProjectLimits[projectId]
	if !ok {
		log.Warnf("Project %s is not in the projects limits cache", projectId)
	} else {
		log.Debugf("Project %s limits: %+v", projectId, projectLimits)
	}

	if handler.RedisClient.IsBlocked(projectId) {
		handler.PerformanceBlockedByLimit.Inc()
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
	stringMessage := string(message.Payload)
	if !gjson.Valid(stringMessage) {
		return ResponseMessage{400, true, "Invalid payload JSON format"}
	}

	// Parse JSON into map
	var jsonMap map[string]interface{}
	if err := json.Unmarshal([]byte(stringMessage), &jsonMap); err != nil {
		return ResponseMessage{400, true, "Failed to parse payload JSON"}
	}

	// Add timestamp
	jsonMap["timestamp"] = time.Now().Unix()

	// Convert back to JSON
	modifiedMessage, err := json.Marshal(jsonMap)
	if err != nil {
		return ResponseMessage{400, true, fmt.Sprintf("Failed to encode modified JSON: %s", err)}
	}

	// convert message to JSON format
	messageToSend := BrokerMessage{ProjectId: projectId, Payload: modifiedMessage, CatcherType: CatcherType}
	rawMessage, err := json.Marshal(messageToSend)
	if err != nil {
		log.Errorf("Message marshalling error: %v", err)
		return ResponseMessage{400, true, "Cannot encode message to JSON"}
	}

	// send serialized message to a broker
	brokerMessage := broker.Message{Payload: rawMessage, Route: PerformanceQueueName}
	log.Debugf("Send to queue: %s", brokerMessage)
	handler.PerformanceBroker.Chan <- brokerMessage

	// increment processed errors counter
	handler.PerformanceProcessed.Inc()

	return ResponseMessage{200, false, "OK"}
}

func (h *Handler) GetMaxMessageSize() int {
	return h.MaxPerformanceCatcherMessageSize
}
