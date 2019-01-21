package server

import "encoding/json"

// Sender represents information about message sender
type Sender struct {
	IP string `json:"ip"`
}

// Request represents JSON got from catchers
type Request struct {
	Token       string          `json:"token"`
	Payload     json.RawMessage `json:"payload"`
	CatcherType string          `json:"catcher_type"`
}
