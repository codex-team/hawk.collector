package broker

import (
	"github.com/codex-team/hawk.collector/cmd"
)

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
