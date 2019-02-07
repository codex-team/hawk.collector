package server

import (
	"encoding/json"
	"github.com/codex-team/hawk.catcher/catcher/lib"
	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"
	"log"
)

var upgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *fasthttp.RequestCtx) bool {
		return true
	},
}

// Response represents JSON answer from the HTTP server
type Response struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Status int  `json:"status"`
}

// RequestHandler - handle HTTP connections and send valid messages to the global queue
func RequestHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("text/json; charset=utf8")

	switch string(ctx.Path()) {
	case "/":
		catcherHTTPHandler(ctx)
	case "/ws", "/ws/":
		catcherWebsocketsHandler(ctx)
	default:
		SendAnswer(ctx, Response{true, "Invalid path", fasthttp.StatusBadRequest})
	}

}

func processMessage(body []byte) Response {
	// Check if the body is a valid JSON with the Message structure
	message := &Request{}
	err := json.Unmarshal(body, message)
	if err != nil {
		return Response{true, "Invalid JSON format", fasthttp.StatusBadRequest}
	}

	// Validate Message data
	valid, cause := message.Validate()
	if !valid {
		return Response{true, cause, fasthttp.StatusBadRequest}
	}

	// Compress JSON data and send to the messagesQueue
	minifiedJSON, err := minifyJSON(message.Payload)
	if err != nil {
		log.Printf("JSON compression error: %v", err)
		return Response{true, "Server error", fasthttp.StatusInternalServerError}
	}

	messagesQueue <- lib.Message{minifiedJSON, message.CatcherType}
	return Response{false, "OK", fasthttp.StatusOK}
}

func catcherHTTPHandler(ctx *fasthttp.RequestCtx) {
	log.Printf("%s request from %s", ctx.Method(), ctx.RemoteIP())
	SendAnswer(ctx, processMessage(ctx.PostBody()))
}

func catcherWebsocketsHandler(ctx *fasthttp.RequestCtx)  {
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