// WebSocket manipulation primitives

package server

import (
	"log"

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

// catcherWebsocketsHandler upgrades HTTP request to WebSocket connection
func catcherWebsocketsHandler(ctx *fasthttp.RequestCtx) {
	err := upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read message error:", err)
				break
			}

			log.Printf("recv: %s", message)

			answerBuffer := []byte(processMessage(message).Message)
			err = conn.WriteMessage(mt, answerBuffer)

			if err != nil {
				log.Println("write message error:", err)
				break
			}
		}
	})

	if err != nil {
		log.Println(err)
	}
}
