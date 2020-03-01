package sourcemapshandler

import (
	"encoding/json"
	"github.com/codex-team/hawk.collector/cmd"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

// SourcemapFile represents file content and its name
type SourcemapFile struct {
	Name    string `json:"name"`
	Payload []byte `json:"payload"`
}

// SourcemapMessage represents message structure for sending to queue
type SourcemapMessage struct {
	ProjectId string          `json:"projectId"`
	Release   string          `json:"release"`
	Files     []SourcemapFile `json:"files"`
}

// HandleHTTP processes HTTP requests with JSON body
func (handler *Handler) HandleHTTP(ctx *fasthttp.RequestCtx) {
	if ctx.Request.Header.ContentLength() > handler.MaxSourcemapCatcherMessageSize {
		log.Warnf("[sourcemaps] Incoming request with size %d", ctx.Request.Header.ContentLength())
		sendAnswerHTTP(ctx, ResponseMessage{
			Error:   true,
			Message: "Request is too large",
		}, 400)
		return
	}

	// collect JWT token from HTTP Authorization header
	token := ctx.Request.Header.Peek("Authorization")
	if len(token) < 8 {
		log.Warnf("[sourcemaps] Missing header (len=%d): %s", len(token), token)
		sendAnswerHTTP(ctx, ResponseMessage{true, "Provide Authorization header"}, 400)
		return
	}
	// cut "Bearer "
	token = token[7:]

	form, err := ctx.MultipartForm()
	if err != nil {
		log.Warnf("[sourcemaps] Multipart form is not provided for token: %s", token)
		sendAnswerHTTP(ctx, ResponseMessage{true, "Multipart form is not provided"}, 400)
		return
	}

	log.Debugf("[sourcemaps] Multipart form with token: %s", token)

	// process raw body via unified sourcemap handler
	response := handler.process(form, string(token))
	log.Debugf("[sourcemaps] Multipart form response: %s", response)

	if response.Error {
		sendAnswerHTTP(ctx, response, 400)
	} else {
		sendAnswerHTTP(ctx, response, 200)
	}
}

// Send ResponseMessage in JSON with statusCode set
func sendAnswerHTTP(ctx *fasthttp.RequestCtx, r ResponseMessage, code int) {
	ctx.Response.SetStatusCode(code)

	response, err := json.Marshal(r)
	cmd.PanicOnError(err)

	_, err = ctx.Write(response)
	cmd.PanicOnError(err)
}