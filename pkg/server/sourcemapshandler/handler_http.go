package sourcemapshandler

import (
	"encoding/json"
	"errors"

	"github.com/codex-team/hawk.collector/pkg/hawk"
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
		})
		return
	}

	// collect JWT token from HTTP Authorization header
	token := ctx.Request.Header.Peek("Authorization")
	if len(token) < 8 {
		log.Warnf("[sourcemaps] Missing header (len=%d): %s", len(token), token)
		sendAnswerHTTP(ctx, ResponseMessage{400, true, "Provide Authorization header"})
		return
	}
	// cut "Bearer "
	token = token[7:]

	form, err := ctx.MultipartForm()
	if err != nil {
		log.Warnf("[sourcemaps] Multipart form is not provided for token: %s", token)
		sendAnswerHTTP(ctx, ResponseMessage{400, true, "Multipart form is not provided"})
		return
	}

	log.Debugf("[sourcemaps] Multipart form with token: %s", token)

	// process raw body via unified sourcemap handler
	response := handler.process(form, string(token))
	log.Debugf("[sourcemaps] Multipart form response: %s", response.Message)

	sendAnswerHTTP(ctx, response)
}

// Send ResponseMessage in JSON with statusCode set
func sendAnswerHTTP(ctx *fasthttp.RequestCtx, r ResponseMessage) {
	ctx.Response.SetStatusCode(r.Code)

	if r.Code != 200 {
		hawk.Catch(errors.New(r.Message))
	}

	response, err := json.Marshal(r)
	if err != nil {
		log.Errorf("Error during response marshalling: %v", err)
		hawk.Catch(err)
		ctx.Response.SetStatusCode(500)
		ctx.SetConnectionClose()
		return
	}

	_, err = ctx.Write(response)
	if err != nil {
		log.Errorf("Error during response write: %v", err)
		hawk.Catch(err)
		ctx.Response.SetStatusCode(500)
		ctx.SetConnectionClose()
		return
	}
}
