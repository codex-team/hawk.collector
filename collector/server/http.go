package server

import (
	"bytes"
	"fmt"
	"io"
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
	form, _ := ctx.MultipartForm()
	for _, v := range form.File {
		for _, header := range v {
			fasthttp.SaveMultipartFile(header, fmt.Sprintf("/tmp/%s", header.Filename))
			f, _ := header.Open()
			defer f.Close()
			buf := bytes.NewBuffer(nil)
			io.Copy(buf, f)

			fmt.Printf("%d", len(buf.Bytes()))
		}
	}
}
