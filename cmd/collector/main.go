package collector

import (
	"github.com/caarlos0/env"
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/codex-team/hawk.collector/pkg/server"
	"log"

	"github.com/joho/godotenv"
)

// RunCommand - Run server in the production mode
type RunCommand struct {
	BrokerURL string `short:"B" long:"broker" description:"Connection URL for broker" required:"false"`
	Host      string `short:"H" long:"host" description:"Server host" required:"false"`
	Port      int    `short:"P" long:"port" description:"Server port" required:"false"`
}

// Execute Run server - Load configuration file and start server
func (x *RunCommand) Execute(args []string) error {
	if err := godotenv.Load(); err != nil {
		log.Println("File .env not found, reading configuration from ENV")
	}

	// load config from .env
	var cfg cmd.Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatalln("Failed to parse ENV")
	}

	// connect to AMQP broker with retries
	brokerObj := broker.New(cfg.BrokerURL, cfg.Exchange)
	brokerObj.Init()

	// start HTTP and websocket server
	serverObj := server.New(cfg, brokerObj)
	serverObj.Run()

	return nil
}
