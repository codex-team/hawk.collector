package server

import "encoding/json"

// Request represents JSON got from catchers
type Request struct {
	Token       string          `json:"token"`
	Payload     json.RawMessage `json:"payload"`
	CatcherType string          `json:"catcher_type"`
}
