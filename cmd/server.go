package cmd

import (
	"encoding/json"
	"github.com/codex-team/hawk.catcher/lib/amqp"
	"github.com/valyala/fasthttp"
	"log"
)

// global messages processing queue
var messagesQueue = make(chan amqp.Message)

// handle HTTP connections and send valid messages to the global queue
func requestHandler(ctx *fasthttp.RequestCtx) {
	if string(ctx.Path()) != "/catcher" {
		SendAnswer(ctx, Response{true, "Invalid path"}, fasthttp.StatusBadRequest)
		return
	}

	ctx.SetContentType("text/json; charset=utf8")
	log.Printf("%s request from %s", ctx.Method(), ctx.RemoteIP())

	message := &Request{}
	err := json.Unmarshal(ctx.PostBody(), message)
	if err != nil {
		SendAnswer(ctx, Response{true, "Invalid JSON format"}, fasthttp.StatusBadRequest)
		return
	}
	valid, cause := message.Validate()
	if !valid {
		SendAnswer(ctx, Response{true, cause}, fasthttp.StatusBadRequest)
		return
	}

	messagesQueue <- amqp.Message{minifyJSON(message.Payload), message.CatcherType}
}

// minifyJSON - Unmarshall JSON and marshall it to remove comments and whitespaces
func minifyJSON(input json.RawMessage) json.RawMessage {

	// Unmarshall raw JSON to Object
	inputObject := &json.RawMessage{}
	err := json.Unmarshal(input, inputObject)
	failOnError(err, "Invalid payload JSON")

	// Marshall object to minified raw JSON
	output, err := json.Marshal(inputObject)
	failOnError(err, "Invalid payload JSON")

	return output
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}
