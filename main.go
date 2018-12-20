package main

import (
	"encoding/json"
	"flag"
	"github.com/valyala/fasthttp"
	"log"
)

var (
	addr = flag.String("addr", ":3000", "TCP address to listen to")
)

// global messages processing queue
var messagesQueue = make(chan Message)

func main() {
	flag.Parse()

	runWorkers()

	if err := fasthttp.ListenAndServe(*addr, requestHandler); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}

// runWorkers - initialize AMQP connections and run background workers
func runWorkers()  {
	connection := Connection{}
	err := connection.Init("amqp://guest:guest@localhost:5672/", "errors")
	if err != nil {
		return
	}

	go func(conn Connection, ch <- chan Message) {
		for msg := range ch {
			_ = conn.Publish(msg)
		}
	}(connection, messagesQueue)
}

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

	messagesQueue <- Message{minifyJSON(message.Payload), message.CatcherType}
}

// minifyJSON - Unmarshall JSON and marshall it to remove comments and whitespaces
func minifyJSON(input json.RawMessage) json.RawMessage {
	d := &json.RawMessage{}
	err := json.Unmarshal(input, d)
	failOnError(err, "Invalid payload JSON")
	output, err := json.Marshal(d)
	failOnError(err, "Invalid payload JSON")
	return output
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}