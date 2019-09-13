package broker

import (
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/streadway/amqp"
)

// Connection contains basic settings for AMQP connection
type Connection struct {
	Exchange  string
	Queue     string
	Mandatory bool
	Immediate bool
	channel   *amqp.Channel
}

// Message represents message payload sent to the Queue and AMQP route
type Message struct {
	Payload []byte
	Route   string
}

type Broker struct {
	Chan       chan Message
	Connection Connection
}

func New(url, exchange string) *Broker {
	var broker Broker
	broker.Chan = make(chan Message)
	err := broker.Connection.Init(url, exchange)
	cmd.FailOnError(err, "broker initialization error")

	return &broker
}

func (broker *Broker) Init() {
	go func(conn Connection, ch <-chan Message) {
		for msg := range ch {
			_ = conn.Publish(msg)
		}
	}(broker.Connection, broker.Chan)
}
