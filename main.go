package main

import (
	"flag"
	"fmt"
	"github.com/streadway/amqp"
	"github.com/valyala/fasthttp"
	"log"
)

var (
	addr     = flag.String("addr", ":8083", "TCP address to listen to")
	compress = flag.Bool("compress", false, "Whether to enable transparent response compression")
)

type Request struct {
	Id string `json:"_id"`
	Type int `json:"type"`
	Uid string `json:"uid"`
	Jwt bool `json:"jwt"`
	Payload string `json:"payload"`
	Approved bool `json:"approved"`
}

var messages = make(chan []byte)

func main() {
	flag.Parse()

	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	_, err = ch.QueueDeclare(
		"hello", // name
		true,   // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	failOnError(err, "Failed to declare a queue")

	h := requestHandler
	//if *compress {
	//	h = fasthttp.CompressHandler(h)
	//}

	go publish(messages, ch)

	if err := fasthttp.ListenAndServe(*addr, h); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}

func requestHandler(ctx *fasthttp.RequestCtx) {
	if string(ctx.Path()) != "/catcher" {
		ctx.Response.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	fmt.Fprintf(ctx, "Hello, world!\n\n")

	//log.Printf("GOT: at %s\n", time.Now().Format("2006-01-02 15:04:05"))

	message := &Request{}
	err := message.UnmarshalJSON(ctx.PostBody())
	if err != nil {
		ctx.Response.SetStatusCode(fasthttp.StatusBadRequest)
		fmt.Fprintf(ctx, "Invalid JSON!\n\n")
		return
	}

	message.Approved = true

	message_bytes, err := message.MarshalJSON()
	if err != nil {
		log.Printf("Cannot marshal JSON: %v\n", err)
		ctx.Response.SetStatusCode(fasthttp.StatusBadRequest)
		fmt.Fprintf(ctx, "Unknown issue!\n\n")
		return
	}

	ctx.SetContentType("text/json; charset=utf8")
	messages <- message_bytes

}

func publish(c <- chan []byte, ch *amqp.Channel)  {
	for m := range c {
		err := ch.Publish(
			"",     // exchange
			"hello", // routing key
			false,  // mandatory
			false,  // immediate
			amqp.Publishing {
				DeliveryMode: amqp.Persistent,
				ContentType: "text/plain",
				Body:        m,
			})
		failOnError(err, "Failed to publish a message")
	}
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}