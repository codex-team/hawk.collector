package errorshandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/codex-team/hawk.collector/pkg/redis"
	"github.com/dgrijalva/jwt-go"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Handler of error messages
type Handler struct {
	Broker    *broker.Broker
	JwtSecret string

	// Maximum POST body size in bytes for error messages
	MaxErrorCatcherMessageSize int

	ErrorsProcessed prometheus.Counter

	RedisClient *redis.RedisClient
}

func (handler *Handler) process(body []byte) ResponseMessage {
	// Check if the body is a valid JSON with the Message structure
	message := CatcherMessage{}
	err := json.Unmarshal(body, &message)
	if err != nil {
		return ResponseMessage{true, "Invalid JSON format"}
	}

	if len(message.Payload) == 0 {
		return ResponseMessage{true, "Payload is empty"}
	}
	if message.Token == "" {
		return ResponseMessage{true, "Token is empty"}
	}
	if message.CatcherType == "" {
		return ResponseMessage{true, "CatcherType is empty"}
	}

	// Validate JWT token
	projectId, err := handler.DecodeJWT(message.Token)
	if err != nil {
		return ResponseMessage{true, fmt.Sprintf("%s", err)}
	}

	if handler.RedisClient.IsBlocked(projectId) {
		return ResponseMessage{true, "Project has exceeded the events limit"}
	}

	// Validate if message is a valid JSON
	stringMessage := string(message.Payload)
	if !gjson.Valid(stringMessage) {
		return ResponseMessage{true, "Invalid payload JSON format"}
	}

	modifiedMessage, err := sjson.Set(stringMessage, "timestamp", time.Now().Unix())
	if err != nil {
		return ResponseMessage{true, fmt.Sprintf("%s", err)}
	}

	// convert message to JSON format
	messageToSend := BrokerMessage{ProjectId: projectId, Payload: []byte(modifiedMessage)}
	rawMessage, err := json.Marshal(messageToSend)
	if err != nil {
		log.Errorf("Message marshalling error: %v", err)
		return ResponseMessage{true, "Cannot encode message to JSON"}
	}

	// send serialized message to a broker
	brokerMessage := broker.Message{Payload: rawMessage, Route: message.CatcherType}
	log.Debugf("Send to queue: %s", brokerMessage)
	handler.Broker.Chan <- brokerMessage

	// increment processed errors counter
	handler.ErrorsProcessed.Inc()

	return ResponseMessage{false, "OK"}
}

// DecodeJWT – check JWT and return projectId
func (handler *Handler) DecodeJWT(token string) (string, error) {
	var tokenData JWTClaim
	_, err := jwt.ParseWithClaims(token, &tokenData, func(token *jwt.Token) (interface{}, error) {
		return []byte(handler.JwtSecret), nil
	})
	if err != nil {
		return "", errors.New("invalid JWT signature")
	}

	log.Debugf("Token data: %s", tokenData)
	if tokenData.ProjectId == "" {
		return "", errors.New("empty projectId")
	}

	return tokenData.ProjectId, nil
}
