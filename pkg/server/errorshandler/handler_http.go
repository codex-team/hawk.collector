package errorshandler

import (
	"encoding/json"
	"errors"

	"github.com/codex-team/hawk.collector/pkg/hawk"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

// HandleHTTP processes HTTP requests with JSON body
func (handler *Handler) HandleHTTP(ctx *fasthttp.RequestCtx) {
	if ctx.Request.Header.ContentLength() > handler.MaxErrorCatcherMessageSize {
		log.Warnf("Incoming request with size %d", ctx.Request.Header.ContentLength())
		sendAnswerHTTP(ctx, ResponseMessage{
			Code:    400,
			Error:   true,
			Message: "Request is too large",
		})
		return
	}

	// process raw body via unified message handler
	body := ctx.PostBody()
	log.Debugf("Headers: %s\nBody: %s", ctx.Request.Header.String(), body)

	response := handler.process(body)
	log.Debugf("Response: %s", response)

	sendAnswerHTTP(ctx, response)
}

// Send ResponseMessage in JSON with statusCode set
func sendAnswerHTTP(ctx *fasthttp.RequestCtx, r ResponseMessage) {
	if r.Message == "" {
		return
	}
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
