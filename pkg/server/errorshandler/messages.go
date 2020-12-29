package errorshandler

import (
	"encoding/json"

	"github.com/dgrijalva/jwt-go"
)

// ResponseMessage represents incoming message from a client
type CatcherMessage struct {
	Token       string          `json:"token"`
	Payload     json.RawMessage `json:"payload"`
	CatcherType string          `json:"catcherType"`
}

// ResponseMessage represents response message to a client
type ResponseMessage struct {
	Code    int
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

// BrokerMessage represents message to a queue
type BrokerMessage struct {
	ProjectId string          `json:"projectId"`
	Payload   json.RawMessage `json:"payload"`
}

// Data of JWT token
type JWTClaim struct {
	ProjectId string `json:"projectId"`
	jwt.StandardClaims
}
