package errorshandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/dgrijalva/jwt-go"
	"github.com/tidwall/gjson"
)

type Handler struct {
	Broker                     *broker.Broker
	MaxErrorCatcherMessageSize int
	JwtSecret                  string
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

	// Validate JWT
	projectId, err := handler.DecodeJWT(message.Token)
	if err != nil {
		return ResponseMessage{true, fmt.Sprintf("%s", err)}
	}

	if !gjson.Valid(string(message.Payload)) {
		return ResponseMessage{true, "Invalid payload JSON format"}
	}

	messageToSend := BrokerMessage{ProjectId: projectId, Payload: message.Payload}
	rawMessage, err := json.Marshal(messageToSend)
	cmd.PanicOnError(err)

	handler.Broker.Chan <- broker.Message{Payload: rawMessage, Route: message.CatcherType}
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

	if tokenData.ProjectId == "" {
		return "", errors.New("empty projectId")
	}

	return tokenData.ProjectId, nil
}
