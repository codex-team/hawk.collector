package cmd

import (
	"fmt"
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
