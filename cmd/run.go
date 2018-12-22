package cmd

import (
	"fmt"
	"github.com/codex-team/hawk.catcher/lib/amqp"
	"github.com/valyala/fasthttp"
	"log"
	"net"
	"time"
)

func (x *RunCommand) Execute(args []string) error {

	config := &Configuration{}
	err := config.load(x.ConfigurationFilename)
	if err != nil {
		log.Fatalf("Configuration file with name %s not found", x.ConfigurationFilename)
	}

	retry := config.RetryNumber
	res := runWorkers(*config)
	for (res == false) && (retry > 0) {
		time.Sleep(time.Second * time.Duration(config.RetryInterval))
		res = runWorkers(*config)
		retry--
	}
	if !res {
		log.Fatalf("Could not connect to rabbitmq")
	}

	if err := fasthttp.ListenAndServe(net.JoinHostPort(config.Host, fmt.Sprintf("%d", config.Port)), requestHandler); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}

	return nil
}

// runWorkers - initialize AMQP connections and run background workers
func runWorkers(config Configuration) bool {
	connection := amqp.Connection{}
	err := connection.Init(config.BrokerURL, "errors")
	if err != nil {
		return false
	}

	go func(conn amqp.Connection, ch <-chan amqp.Message) {
		for msg := range ch {
			_ = conn.Publish(msg)
		}
	}(connection, messagesQueue)
	return true
}
