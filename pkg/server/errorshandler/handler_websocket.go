package errorshandler

import (
	"encoding/json"
	"time"

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

		// Set initial read deadline
		if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			log.Errorf("Failed to set read deadline: %v", err)
			return
		}

		// Setup pong handler to reset the read deadline
		conn.SetPongHandler(func(string) error {
			if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
				log.Errorf("Failed to set read deadline in pong handler: %v", err)
				return err
			}
			return nil
		})

		// Start a ticker to send pings
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		// Create a done channel to signal when to exit
		done := make(chan struct{})

		// Start goroutine for ping
		go func() {
			for {
				select {
				case <-ticker.C:
					if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
						log.Errorf("Ping error: %v", err)
						close(done)
						return
					}
				case <-done:
					return
				}
			}
		}()

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				log.Errorf("Websocket error in ReadMessage: %v", err)
				close(done)
				break
			}

			// Reset the read deadline on successful read
			if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
				log.Errorf("Failed to reset read deadline: %v", err)
				close(done)
				break
			}

			log.Debugf("Websocket message: %s", message)

			// process raw body via unified message handler
			response := handler.process(message)
			log.Debugf("Websocket response: %s", response.Message)

			if err = sendAnswerWebsocket(conn, messageType, response); err != nil {
				log.Errorf("Websocket response: %v", err)
				close(done)
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
