package server

import (
	"encoding/json"
	"fmt"
	"github.com/codex-team/hawk.collector/collector/configuration"
	"github.com/codex-team/hawk.collector/collector/lib"

	"github.com/dgrijalva/jwt-go"
	"github.com/valyala/fasthttp"
)

// global messages processing queue
var messagesQueue = make(chan lib.Message)

// JWT signature secret
var jwtSecret string

// SendAnswer – send HTTP response to the client
//
// ctx – HTTP context
// r – Response structure that will be serialized and send as HTTP body
// status – HTTP status code
func SendAnswer(ctx *fasthttp.RequestCtx, r Response) {
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
func (r *Request) DecodeJWT() (bool, string, string) {
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

// Setup - initialize connection to the queue server and initialize configuration variables
func Setup(config configuration.Configuration) (lib.Connection, error) {
	connection := lib.Connection{}
	err := connection.Init(config.BrokerURL, config.Exchange)
	if err != nil {
		return connection, err
	}
	jwtSecret = config.JwtSecret

	return connection, nil
}

// RunWorkers - run background worker which will read message from the channel and process it.
// There may be several workers with separate connections to the RabbitMQ
func RunWorkers(connection lib.Connection, config configuration.Configuration) bool {
	go func(conn lib.Connection, ch <-chan lib.Message) {
		for msg := range ch {
			_ = conn.Publish(msg)
		}
	}(connection, messagesQueue)
	return true
}

// Data of JWT token
type JWTClaim struct {
	ProjectId string `json:"projectId"`
	jwt.StandardClaims
}
