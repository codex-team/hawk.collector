package main

import (
	"encoding/json"
	"fmt"
	"github.com/valyala/fasthttp"
)

type Sender struct {
	Ip string `json:"ip"`
}

type Request struct {
	Token string `json:"token"`
	Payload json.RawMessage `json:"payload"`
	CatcherType string `json:"catcher_type"`
	Sender Sender `json:"sender"`
}

type Response struct {
	Error bool `json:"error"`
	Message string `json:"message"`
}

func SendAnswer(ctx *fasthttp.RequestCtx, r Response, status int) {
	ctx.Response.SetStatusCode(status)

	response, err := json.Marshal(r)
	failOnError(err, "Cannot marshall response")

	n, err := ctx.Write(response)
	failOnError(err, fmt.Sprintf("Cannot write an answer: %d", n))
}

func (r *Request) Validate () (bool, string) {
	if r.Token == "" {
		return false, "Token is empty"
	}
	if r.Payload == nil {
		return false, "Payload is empty"
	}
	if r.CatcherType == "" {
		return false, "CatcherType is empty"
	}
	if r.Sender.Ip == "" {
		return false, "Sender is empty"
	}
	return true, ""
}