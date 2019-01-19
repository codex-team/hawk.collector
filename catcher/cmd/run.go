package cmd

import (
	"fmt"
	"github.com/codex-team/hawk.catcher/catcher/server"
	"log"
	"net"
	"time"

	"github.com/codex-team/hawk.catcher/catcher/configuration"

	"github.com/valyala/fasthttp"
)

// Execute Run server - Load configuration file and start server
func (x *RunCommand) Execute(args []string) error {
	config := &configuration.Configuration{}
	err := config.Load(x.ConfigurationFilename)
	if err != nil {
		log.Fatalf("Configuration file with name %s not found", x.ConfigurationFilename)
	}

	// Try to connect to the Queue server several times until success or out of RetryNumber
	retry := config.RetryNumber
	connection, err := server.Connect(*config)
	for (err != nil) && (retry > 0) {
		time.Sleep(time.Second * time.Duration(config.RetryInterval))
		connection, err = server.Connect(*config)
		retry--
	}
	if err != nil {
		log.Fatalf("Could not connect to the queue server")
	}

	// Run background workers
	server.RunWorkers(connection, *config)

	// Run HTTP server and block execution
	if err := fasthttp.ListenAndServe(net.JoinHostPort(config.Host, fmt.Sprintf("%d", config.Port)), server.RequestHandler); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}

	return nil
}
