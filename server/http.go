package server

import (
	"log"

	"github.com/valyala/fasthttp"
)

// Response represents JSON answer from the HTTP server
type Response struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// catcherHTTPHandler processes HTTP requests with JSON body
func catcherHTTPHandler(ctx *fasthttp.RequestCtx) {
	log.Printf("%s catcherHTTPHandler request from %s", ctx.Method(), ctx.RemoteIP())
	sendAnswer(ctx, processMessage(ctx.PostBody()))
}
