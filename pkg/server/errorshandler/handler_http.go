package errorshandler

import (
	"encoding/json"
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/valyala/fasthttp"
)

// HandleHTTP processes HTTP requests with JSON body
func (handler *Handler) HandleHTTP(ctx *fasthttp.RequestCtx) {
	if ctx.Request.Header.ContentLength() > handler.MaxErrorCatcherMessageSize {
		sendAnswerHTTP(ctx, ResponseMessage{
			Error:   true,
			Message: "Request is too large",
		}, 400)
	}

	body := ctx.PostBody()
	response := handler.process(body)
	if response.Error {
		sendAnswerHTTP(ctx, response, 400)
	} else {
		sendAnswerHTTP(ctx, response, 200)
	}
}

func sendAnswerHTTP(ctx *fasthttp.RequestCtx, r ResponseMessage, code int) {
	ctx.Response.SetStatusCode(code)

	response, err := json.Marshal(r)
	cmd.PanicOnError(err)

	_, err = ctx.Write(response)
	cmd.PanicOnError(err)
}