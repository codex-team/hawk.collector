package ws

import (
	"encoding/json"
	"strings"

	"github.com/codex-team/hawk.collector/pkg/hawk"
	"github.com/codex-team/hawk.collector/pkg/server/errorshandler"
	"github.com/codex-team/hawk.collector/pkg/server/performancehandler"
	"github.com/fasthttp/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type WSMessage struct {
	CatcherType string `json:"catcherType"`
}

type Handler struct {
	ErrorsHandler      *errorshandler.Handler
	PerformanceHandler *performancehandler.Handler
}

// WebSocket upgrader options
var upgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *fasthttp.RequestCtx) bool {
		return true
	},
}

func NewHandler(errorsHandler *errorshandler.Handler, performanceHandler *performancehandler.Handler) *Handler {
	return &Handler{
		ErrorsHandler:      errorsHandler,
		PerformanceHandler: performanceHandler,
	}
}

func (h *Handler) Handle(ctx *fasthttp.RequestCtx) {
	err := upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				log.Errorf("Error reading message: %v", err)
				return
			}

			var msg WSMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Errorf("Error parsing message: %v", err)
				return
			}

			if msg.CatcherType == "" {
				log.Errorf("CatcherType is empty")
				return
			}

			baseType := strings.Split(msg.CatcherType, "/")[0]

			switch baseType {
			case "performance":
				conn.SetReadLimit(int64(h.PerformanceHandler.MaxPerformanceCatcherMessageSize))
				response := h.PerformanceHandler.Process(message)
				if err = sendWSResponse(conn, messageType, response); err != nil {
					log.Errorf("Performance websocket response error: %v", err)
					return
				}
			case "errors":
				conn.SetReadLimit(int64(h.ErrorsHandler.MaxErrorCatcherMessageSize))
				response := h.ErrorsHandler.Process(message)
				if err = sendWSResponse(conn, messageType, response); err != nil {
					log.Errorf("Errors websocket response error: %v", err)
					return
				}
			default:
				log.Errorf("Unknown catcher type: %s", baseType)
				return
			}
		}
	})

	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			hawk.Catch(err)
		}
		log.Errorf("WebSocket error: %v", err)
	}
}

func sendWSResponse(conn *websocket.Conn, messageType int, r interface{}) error {
	response, err := json.Marshal(r)
	if err != nil {
		hawk.Catch(err)
		return err
	}
	return conn.WriteMessage(messageType, response)
}
