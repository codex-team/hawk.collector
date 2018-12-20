package main

import (
	"github.com/streadway/amqp"
	"log"
)

type Connection struct {
	Exchange string
	Queue string
	Mandatory bool
	Immediate bool
	channel *amqp.Channel
}

type Message struct {
	Payload []byte
	Route string
}

func (connection *Connection) Init (url string, exchangeName string) error {
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ (%s): %s", url, err)
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel (%s): %s", url, err)
		return err
	}

	connection.channel = ch
	connection.Exchange = exchangeName

	err = connection.channel.ExchangeDeclare(
		exchangeName, 		// name
		"direct",
		true,   	// durable
		false,   	// delete when unused
		false,   	// exclusive
		false,   	// no-wait
		nil,     		// arguments
	)

	if err != nil {
		log.Fatalf("Failed to declare an exchange (%s): %s", url, err)
		return err
	}

	return nil
}

func (connection *Connection) Publish(msg Message) error {
	err := connection.channel.Publish(
		connection.Exchange,     	// exchange
		msg.Route,		 			// routing key
		connection.Mandatory,  		// mandatory
		connection.Immediate,  		// immediate
		amqp.Publishing {
			DeliveryMode: amqp.Persistent,
			ContentType: "text/plain",
			Body: msg.Payload,
		})
	if err != nil {
		log.Fatalf("Failed to publish a message to a %s queue: %s", connection.Queue, err)
	}
	return err
}
