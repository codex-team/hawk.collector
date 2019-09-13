package errorshandler

import (
	"encoding/json"
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"
)

// WebSocket upgrader options
var upgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *fasthttp.RequestCtx) bool {
		return true
	},
}

// HandleWebsocket handles WebSocket connection
func (handler *Handler) HandleWebsocket(ctx *fasthttp.RequestCtx) {
	err := upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		conn.SetReadLimit(int64(handler.MaxErrorCatcherMessageSize))
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}

			sendAnswerWebsocket(conn, messageType, ResponseMessage{
				Error:   false,
				Message: handler.process(message).Message,
			})
		}
	})
	cmd.PanicOnError(err)
}

func sendAnswerWebsocket(conn *websocket.Conn, messageType int, r ResponseMessage) {
	response, err := json.Marshal(r)
	cmd.PanicOnError(err)

	err = conn.WriteMessage(messageType, response)
	cmd.PanicOnError(err)
}
