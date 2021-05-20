package errorshandler

import (
	"encoding/json"

	"github.com/codex-team/hawk.collector/pkg/hawk"
	"github.com/fasthttp/websocket"
	log "github.com/sirupsen/logrus"
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

			log.Debugf("Websocket message: %s", message)

			// process raw body via unified message handler
			response := handler.process(message)
			log.Debugf("Websocket response: %s", response.Message)

			if err = sendAnswerWebsocket(conn, messageType, response); err != nil {
				log.Errorf("Websocket response: %v", err)
				return
			}
		}
	})

	// log if connection is closed ungracefully
	if err != nil {
		// Do not catch WebSocket upgrade erros, since it's usually client malformed requests
		if _, ok := err.(websocket.HandshakeError); !ok {
			hawk.Catch(err)
		}
		log.Errorf("Websocket error: %v", err)
	}
}

// Send ResponseMessage in JSON
func sendAnswerWebsocket(conn *websocket.Conn, messageType int, r ResponseMessage) error {
	response, err := json.Marshal(r)
	if err != nil {
		hawk.Catch(err)
		return err
	}

	return conn.WriteMessage(messageType, response)
}
