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

// RequestHandler - handle connections and send valid messages to the global queue
func RequestHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("text/json; charset=utf8")

	switch string(ctx.Path()) {
	case "/":
		catcherHTTPHandler(ctx)
	case "/sourcemap":
		sourcemapUploadHandler(ctx)
	case "/ws", "/ws/":
		catcherWebsocketsHandler(ctx)
	default:
		SendAnswer(ctx, Response{true, "Invalid path", fasthttp.StatusBadRequest})
	}

}

// catcherHTTPHandler processes HTTP requests with JSON body
func catcherHTTPHandler(ctx *fasthttp.RequestCtx) {
	log.Printf("%s catcherHTTPHandler request from %s", ctx.Method(), ctx.RemoteIP())
	SendAnswer(ctx, processMessage(ctx.PostBody()))
}
