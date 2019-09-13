package sourcemapshandler

import (
	"encoding/json"
	"github.com/codex-team/hawk.collector/cmd"
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

func (handler *Handler) HandleHTTP(ctx *fasthttp.RequestCtx) {
	if ctx.Request.Header.ContentLength() > handler.MaxSourcemapCatcherMessageSize {
		sendAnswerHTTP(ctx, ResponseMessage{
			Error:   true,
			Message: "Request is too large",
		}, 400)
	}

	token := ctx.Request.Header.Peek("Authorization")
	if len(token) < 8 {
		sendAnswerHTTP(ctx, ResponseMessage{true, "Provide Authorization header"}, 400)
		return
	}
	// cut "Bearer "
	token = token[7:]

	form, err := ctx.MultipartForm()
	if err != nil {
		sendAnswerHTTP(ctx, ResponseMessage{true, "Multipart form is not provided"}, 400)
		return
	}

	response := handler.process(form, string(token))
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
