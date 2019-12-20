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
		// limit read size of MaxErrorCatcherMessageSize bytes
		conn.SetReadLimit(int64(handler.MaxErrorCatcherMessageSize))
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}

			// process raw body via unified message handler
			sendAnswerWebsocket(conn, messageType, handler.process(message))
		}
	})

	// panic if connection is closed ungracefully
	cmd.PanicOnError(err)
}

// Send ResponseMessage in JSON
func sendAnswerWebsocket(conn *websocket.Conn, messageType int, r ResponseMessage) {
	response, err := json.Marshal(r)
	cmd.PanicOnError(err)

	err = conn.WriteMessage(messageType, response)
	cmd.PanicOnError(err)
}
