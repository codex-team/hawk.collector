package server

import (
	"encoding/json"
	"errors"
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

// DecodeJWT – check JWT and return projectId
func DecodeJWT(token string) (string, error) {
	var tokenData JWTClaim
	_, err := jwt.ParseWithClaims(token, &tokenData, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return "", errors.New("Invalid JWT signature")
	}

	if tokenData.ProjectId == "" {
		return "", errors.New("Empty projectId")
	}

	return tokenData.ProjectId, nil
}
