package main

import (
	"encoding/json"
	"fmt"

	"github.com/valyala/fasthttp"
)

// Sender represents information about message sender
type Sender struct {
	IP string `json:"ip"`
}

// Request represents JSON got from catchers
type Request struct {
	Token       string          `json:"token"`
	Payload     json.RawMessage `json:"payload"`
	CatcherType string          `json:"catcher_type"`
	Sender      Sender          `json:"sender"`
}

// Response represents JSON answer to the catcher
type Response struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

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
