package errorshandler

import (
	"encoding/json"
	"github.com/codex-team/hawk.collector/cmd"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

// HandleHTTP processes HTTP requests with JSON body
func (handler *Handler) HandleHTTP(ctx *fasthttp.RequestCtx) {
	if ctx.Request.Header.ContentLength() > handler.MaxErrorCatcherMessageSize {
		log.Warnf("Incoming request with size %d", ctx.Request.Header.ContentLength())
		sendAnswerHTTP(ctx, ResponseMessage{
			Error:   true,
			Message: "Request is too large",
		}, 400)
		return
	}

	// process raw body via unified message handler
	body := ctx.PostBody()
	log.Debugf("Headers: %s\nBody: %s", ctx.Request.Header.String(), body)

	response := handler.process(body)
	log.Debugf("Response: %s", response)

	if response.Error {
		sendAnswerHTTP(ctx, response, 400)
		return
	} else {
		sendAnswerHTTP(ctx, response, 200)
		return
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
