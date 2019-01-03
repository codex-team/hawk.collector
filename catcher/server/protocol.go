package server

import (
	"encoding/json"
	"fmt"

	"github.com/codex-team/hawk.catcher/catcher/configuration"

	"github.com/codex-team/hawk.catcher/catcher/lib"

	"github.com/valyala/fasthttp"
)

// global messages processing queue
var messagesQueue = make(chan lib.Message)

// SendAnswer – send HTTP response to the client
//
// ctx – HTTP context
// r – Response structure that will be serialized and send as HTTP body
// status – HTTP status code
func SendAnswer(ctx *fasthttp.RequestCtx, r Response, status int) {
	ctx.Response.SetStatusCode(status)

	response, err := json.Marshal(r)
	failOnError(err, "Cannot marshall response")

	n, err := ctx.Write(response)
	failOnError(err, fmt.Sprintf("Cannot write an answer: %d", n))
}

// Validate – check if request structure has valid format
//
// Return:
// - is the request structure valid (bool)
// - cause of the error (string). Empty if the request is valid
func (r *Request) Validate() (bool, string) {
	if r.Token == "" {
		return false, "Token is empty"
	}
	if r.Payload == nil {
		return false, "Payload is empty"
	}
	if r.CatcherType == "" {
		return false, "CatcherType is empty"
	}
	if r.Sender.IP == "" {
		return false, "Sender is empty"
	}
	return true, ""
}

// Connect - initialize connection to the queue server
func Connect(config configuration.Configuration) (lib.Connection, error) {
	connection := lib.Connection{}
	err := connection.Init(config.BrokerURL, config.Exchange)
	if err != nil {
		return connection, err
	}

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
