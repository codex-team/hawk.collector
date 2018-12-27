package cmd

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/codex-team/hawk.catcher/catcher/server"

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

	retry := config.RetryNumber
	res := server.RunWorkers(*config)
	for (res == false) && (retry > 0) {
		time.Sleep(time.Second * time.Duration(config.RetryInterval))
		res = server.RunWorkers(*config)
		retry--
	}
	if !res {
		log.Fatalf("Could not connect to rabbitmq")
	}

	if err := fasthttp.ListenAndServe(net.JoinHostPort(config.Host, fmt.Sprintf("%d", config.Port)), server.RequestHandler); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}

	return nil
}
