package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSONMinification(t *testing.T) {
	in := json.RawMessage(`{
		"token":"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9",
		"sender": {
		   "ip":"127.0.0.1"
		}, "test": "two words" ,
		"catcher_type":"errors/golang" }`)

	out := json.RawMessage(`{"token":"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9","sender":{"ip":"127.0.0.1"},"test":"two words","catcher_type":"errors/golang"}`)
	result, _ := minifyJSON(in)
	assert.Equal(t, result, out)
}