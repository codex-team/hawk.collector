package server

import (
	"encoding/json"
	"fmt"
	"github.com/codex-team/hawk.collector/lib"

	"github.com/dgrijalva/jwt-go"
	"github.com/valyala/fasthttp"
)

// Data of JWT token
type JWTClaim struct {
	ProjectId string `json:"projectId"`
	jwt.StandardClaims
}

// global messages processing queue
var messagesQueue = make(chan lib.Message)

// SendAnswer – send HTTP response to the client
//
// ctx – HTTP context
// r – Response structure that will be serialized and send as HTTP body
// status – HTTP status code
func sendAnswer(ctx *fasthttp.RequestCtx, r Response) {
	ctx.Response.SetStatusCode(r.Status)

	response, err := json.Marshal(r)
	failOnError(err, "Cannot marshall response")

	n, err := ctx.Write(response)
	failOnError(err, fmt.Sprintf("Cannot write an answer: %d", n))
}

// DecodeJWT – check if request structure has valid format and return projectId from JWT
//
// Return:
// - is the request structure valid (bool)
// - cause of the error (string). Empty if the request is valid
func (r *Request) decodeJWT() (bool, string, string) {
	if r.Token == "" {
		return false, "", "Token is empty"
	}
	if r.CatcherType == "" {
		return false, "", "CatcherType is empty"
	}

	tokenString := r.Token
	var tokenData JWTClaim
	_, err := jwt.ParseWithClaims(tokenString, &tokenData, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return false, "", "Invalid JWT signature"
	}

	if tokenData.ProjectId == "" {
		return false, "", "Invalid JWT"
	}

	return true, tokenData.ProjectId, ""
}
