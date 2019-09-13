package sourcemapshandler

import "github.com/dgrijalva/jwt-go"

type ResponseMessage struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

// Data of JWT token
type JWTClaim struct {
	ProjectId string `json:"projectId"`
	jwt.StandardClaims
}
