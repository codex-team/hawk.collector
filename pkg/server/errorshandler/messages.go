package errorshandler

import (
	"encoding/json"
	"github.com/dgrijalva/jwt-go"
)

type ResponseMessage struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

type CatcherMessage struct {
	Token       string          `json:"token"`
	Payload     json.RawMessage `json:"payload"`
	CatcherType string          `json:"catcherType"`
}

type BrokerMessage struct {
	ProjectId string          `json:"projectId"`
	Payload   json.RawMessage `json:"payload"`
}

// Data of JWT token
type JWTClaim struct {
	ProjectId string `json:"projectId"`
	jwt.StandardClaims
}
