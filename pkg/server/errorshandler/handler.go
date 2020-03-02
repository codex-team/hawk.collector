package errorshandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

// Handler of error messages
type Handler struct {
	Broker    *broker.Broker
	JwtSecret string

	// Maximum POST body size in bytes for error messages
	MaxErrorCatcherMessageSize int
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

	// Validate if message is a valid JSON
	if !gjson.Valid(string(message.Payload)) {
		return ResponseMessage{true, "Invalid payload JSON format"}
	}

	// convert message to JSON format
	messageToSend := BrokerMessage{ProjectId: projectId, Payload: message.Payload}
	rawMessage, err := json.Marshal(messageToSend)
	cmd.PanicOnError(err)

	// send serialized message to a broker
	brokerMessage := broker.Message{Payload: rawMessage, Route: message.CatcherType}
	log.Debugf("Send to queue: %v", brokerMessage)
	handler.Broker.Chan <- brokerMessage

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
