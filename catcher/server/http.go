package server

import (
	"encoding/json"
	"github.com/codex-team/hawk.catcher/catcher/lib"
	"github.com/valyala/fasthttp"
	"log"
	"github.com/fasthttp/websocket"
)

var upgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Response represents JSON answer from the HTTP server
type Response struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

// RequestHandler - handle HTTP connections and send valid messages to the global queue
func RequestHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("text/json; charset=utf8")

	// Process only valid HTTP requests to the '/catcher' URI
	//if string(ctx.Path()) != "/catcher" {
	//	SendAnswer(ctx, Response{true, "Invalid path"}, fasthttp.StatusBadRequest)
	//	return
	//}

	switch string(ctx.Path()) {
	case "/catcher", "/catcher/":
		catcherHTTPHandler(ctx)
	case "/ws", "/ws/":
		catcherWebsocketsHandler(ctx)
	default:
		SendAnswer(ctx, Response{true, "Invalid path"}, fasthttp.StatusBadRequest)
		return
	}

}

func catcherHTTPHandler(ctx *fasthttp.RequestCtx) {
	log.Printf("%s request from %s", ctx.Method(), ctx.RemoteIP())

	// Check if Request POST body is valid JSON with the Message structure
	message := &Request{}
	err := json.Unmarshal(ctx.PostBody(), message)
	if err != nil {
		SendAnswer(ctx, Response{true, "Invalid JSON format"}, fasthttp.StatusBadRequest)
		return
	}

	// Validate Message data
	valid, cause := message.Validate()
	if !valid {
		SendAnswer(ctx, Response{true, cause}, fasthttp.StatusBadRequest)
		return
	}

	// Compress JSON data and send to the messagesQueue
	minifiedJSON, err := minifyJSON(message.Payload)
	if err != nil {
		SendAnswer(ctx, Response{true, "Server error"}, fasthttp.StatusInternalServerError)
		log.Printf("JSON compression error: %v", err)
		return
	}
	messagesQueue <- lib.Message{minifiedJSON, message.CatcherType}
}

func catcherWebsocketsHandler(ctx *fasthttp.RequestCtx)  {
	err := upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return
		}
		log.Printf("recv: %s", message)
		err = conn.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			return
		}
	})

	if err != nil {
		log.Println(err)
	}
}