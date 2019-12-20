package broker

import (
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/streadway/amqp"
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

// Message represents message payload sent to the Queue and AMQP route
type Message struct {
	Payload []byte
	Route   string
}

// Broker represents connection to a message broker and messages channel
type Broker struct {
	Chan       chan Message
	Connection Connection
}

// New returns newly broker object
func New(url, exchange string) *Broker {
	var broker Broker
	broker.Chan = make(chan Message)
	err := broker.Connection.Init(url, exchange)
	cmd.FailOnError(err, "broker initialization error")

	return &broker
}

// Init creates background channel receiver for publishing messages to a broker via connection
func (broker *Broker) Init() {
	go func(conn Connection, ch <-chan Message) {
		for msg := range ch {
			_ = conn.Publish(msg)
		}
	}(broker.Connection, broker.Chan)
}
