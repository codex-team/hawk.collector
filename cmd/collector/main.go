package collector

import (
	"context"
	"os"
	"time"

	"github.com/caarlos0/env"
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/codex-team/hawk.collector/pkg/hawk"
	"github.com/codex-team/hawk.collector/pkg/metrics"
	"github.com/codex-team/hawk.collector/pkg/redis"
	"github.com/codex-team/hawk.collector/pkg/server"

	log "github.com/sirupsen/logrus"

	"github.com/joho/godotenv"
)

// loadInterval is time interval to load data from Redis
const loadInterval = 5 * time.Minute

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
		log.Fatalf("Failed to parse ENV")
	}

	// setup logging and set log level from config
	level, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = log.ErrorLevel
	}
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(level)
	log.Infof("✓ Log level set on %s", level)

	// Initialize Hawk Cather
	if cfg.HawkEnabled {
		err = hawk.Init()
		if err != nil {
			log.Errorf("✗ Cannot initialize Hawk Catcher: %s", err)
		} else {
			go hawk.HawkCatcher.Run()
			defer hawk.HawkCatcher.Stop()
			log.Infof("✓ Hawk Catcher initialized on %s", hawk.HawkCatcher.GetURL())
		}
	}

	// connect to AMQP broker with retries
	brokerObj := broker.New(cfg.BrokerURL, cfg.Exchange)
	brokerObj.Init()
	log.Infof("✓ Broker initialized on %s", cfg.BrokerURL)

	// connect to Redis
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	redisClient := redis.New(ctx, cfg.RedisURL, cfg.RedisPassword, cfg.RedisSet)
	err = redisClient.LoadSet()
	if err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(loadInterval)
		for {
			select {
			case <-ticker.C:
				err := redisClient.LoadSet()
				if err != nil {
					log.Errorf("failed to load data from Redis: %s", err.Error())
					return
				}
			case <-done:
				return
			}
		}
	}()
	defer close(done)
	log.Info("✓ Redis client initialized")

	// listen and serve prometheus metrics
	go metrics.RunServer(cfg.MetricsListen)

	// start HTTP and websocket server
	serverObj := server.New(cfg, brokerObj, redisClient)
	serverObj.Run()

	return nil
}
