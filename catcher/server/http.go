package server

import (
	"github.com/valyala/fasthttp"
	"log"
)

// Response represents JSON answer from the HTTP server
type Response struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Status int  `json:"status"`
}

// RequestHandler - handle connections and send valid messages to the global queue
func RequestHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("text/json; charset=utf8")

	switch string(ctx.Path()) {
	case "/":
		catcherHTTPHandler(ctx)
	case "/ws", "/ws/":
		catcherWebsocketsHandler(ctx)
	default:
		SendAnswer(ctx, Response{true, "Invalid path", fasthttp.StatusBadRequest})
	}

}

func catcherHTTPHandler(ctx *fasthttp.RequestCtx) {
	log.Printf("%s request from %s", ctx.Method(), ctx.RemoteIP())
	SendAnswer(ctx, processMessage(ctx.PostBody()))
}
