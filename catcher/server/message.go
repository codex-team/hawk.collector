package server

import (
	"encoding/json"
	"log"

	"github.com/codex-team/hawk.catcher/catcher/lib"
	"github.com/valyala/fasthttp"
)

// Request represents JSON got from catchers
type Request struct {
	Token       string          `json:"token"`
	Payload     json.RawMessage `json:"payload"`
	CatcherType string          `json:"catcher_type"`
}

// processMessage validates body, sends it to queue and return http response
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
