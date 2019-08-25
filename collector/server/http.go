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

// catcherHTTPHandler processes HTTP requests
func catcherHTTPHandler(ctx *fasthttp.RequestCtx) {
	log.Printf("%s catcherHTTPHandler request from %s", ctx.Method(), ctx.RemoteIP())
	SendAnswer(ctx, processMessage(ctx.PostBody()))
}

func sourcemapUploadHandler(ctx *fasthttp.RequestCtx) {
	log.Printf("%s sourcemapUploadHandler request from %s", ctx.Method(), ctx.RemoteIP())

	token := ctx.Request.Header.Peek("Authentication")
	if len(token) == 0 {
		SendAnswer(ctx, Response{true, "Provide Authentication header", fasthttp.StatusBadRequest})
	}

	form, err := ctx.MultipartForm()
	if err != nil {
		log.Printf("Error: %s", err)
		SendAnswer(ctx, Response{true, "Multipart form is not provided", fasthttp.StatusBadRequest})
	} else {
		SendAnswer(ctx, UploadSourcemap(form, token))
	}
}
