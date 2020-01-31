package broker

import (
	"github.com/streadway/amqp"
	"log"
)

// Connection contains basic settings for AMQP connection
type Connection struct {
	// Exchange name
	Exchange string

	// Queue name
	Queue string

	// If message must be routable (http://www.rabbitmq.com/faq.html#mandatory-flat-routing)
	Mandatory bool

	// If message must be delivered only if there is a consumer (http://www.rabbitmq.com/faq.html#immediate-flat-routing)
	Immediate bool

	// Channel link
	channel *amqp.Channel
}

// Init – Initialize connection to RabbitMQ with URL and default exchange name
//
// It connects to RabbitMQ with credentials and address from URL string (ex: amqp://guest:guest@localhost:5672/)
// Then it opens channel and declare exchange with the name provided.
//
// url – connection URL with credentials (amqp://guest:guest@localhost:5672/)
// exchangeName – name of RabbitMQ exchange
//
// Returns error
func (connection *Connection) Init(url string, exchangeName string) error {
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Printf("Failed to connect to RabbitMQ (%s): %s", url, err)
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Printf("Failed to open a channel (%s): %s", url, err)
		return err
	}

	connection.channel = ch
	connection.Exchange = exchangeName

	err = connection.channel.ExchangeDeclare(
		exchangeName, // name
		"direct",
		true,  // durable queue
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)

	if err != nil {
		log.Printf("Failed to declare an exchange (%s): %s", url, err)
		return err
	}

	return nil
}

// Publish message to the Queue provided by connection
// msg – Message (structure with payload and route name)
//
// Return error
func (connection *Connection) Publish(msg Message) error {
	err := connection.channel.Publish(
		connection.Exchange,  // exchange
		msg.Route,            // routing key
		connection.Mandatory, // mandatory
		connection.Immediate, // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/plain",
			Body:         msg.Payload,
		})

	if err != nil {
		log.Fatalf("Failed to publish a message to a %s queue: %s", connection.Queue, err)
	}

	return err
}
