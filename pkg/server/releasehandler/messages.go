package releasehandler

import (
	"encoding/json"

	"github.com/dgrijalva/jwt-go"
)

// ResponseMessage represents response message to a client
type ResponseMessage struct {
	Code    int    `json:"code"`
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

// Data of JWT token
type JWTClaim struct {
	ProjectId string `json:"projectId"`
	jwt.StandardClaims
}

// ReleaseFile represents file content and its name
type ReleaseFile struct {
	Name    string `json:"name"`
	Payload []byte `json:"payload"`
}

// ReleaseMessagePayload represents payload structure of the message for sending to queue
type ReleaseMessagePayload struct {
	Release string          `json:"release"`
	Commits json.RawMessage `json:"commits"`
	Files   []ReleaseFile   `json:"files"`
}

// ReleaseMessage represents message structure for sending to queue
type ReleaseMessage struct {
	ProjectId string                `json:"projectId"`
	Type      string                `json:"type"`
	Payload   ReleaseMessagePayload `json:"payload"`
}
