package server

import (
	"encoding/json"
	"log"

	"github.com/codex-team/hawk.collector/collector/lib"
	"github.com/valyala/fasthttp"
)

// Request represents JSON got from catchers
type Request struct {
	Token       string          `json:"token"`
	Payload     json.RawMessage `json:"payload"`
	CatcherType string          `json:"catcherType"`
}

// Message represents structure sent to message queue
type Message struct {
	Token   string          `json:"token"`
	Payload json.RawMessage `json:"payload"`
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

	// Compress JSON payload
	minifiedPayload, err := minifyJSON(message.Payload)
	if err != nil {
		log.Printf("JSON compression error: %v", err)
		return Response{true, "Server error", fasthttp.StatusInternalServerError}
	}

	// Create message instance
	messageToSend := Message{Token: message.Token, Payload: minifiedPayload}

	// Marshal JSON to string to send to queue
	minifiedMessage, err := json.Marshal(messageToSend)
	if err != nil {
		log.Printf("JSON compression error: %v", err)
		return Response{true, "Server error", fasthttp.StatusInternalServerError}
	}

	messagesQueue <- lib.Message{Payload: minifiedMessage, Route: message.CatcherType}
	return Response{false, "OK", fasthttp.StatusOK}
}
