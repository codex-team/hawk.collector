package releasehandler

import "github.com/dgrijalva/jwt-go"

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
