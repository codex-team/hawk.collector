package collector

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/codex-team/hawk.collector/pkg/accounts"
	log "github.com/codex-team/hawk.collector/pkg/logger"

	"github.com/caarlos0/env/v6"
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/codex-team/hawk.collector/pkg/hawk"
	"github.com/codex-team/hawk.collector/pkg/metrics"
	"github.com/codex-team/hawk.collector/pkg/periodic"
	"github.com/codex-team/hawk.collector/pkg/redis"
	"github.com/codex-team/hawk.collector/pkg/server"
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
	envLoadErr := godotenv.Load()
	var err error

	// setup logging as early as possible so all logs go to stdout + OTEL
	otelShutdown := log.SetupFromEnv(context.Background())
	defer func() {
		if shutdownErr := otelShutdown(context.Background()); shutdownErr != nil {
			slog.Warn("failed to shutdown OTLP logger", "error", shutdownErr)
		}
	}()

	if envLoadErr != nil {
		slog.Info("File .env not found, reading configuration from ENV")
	}

	// load config from .env
	var cfg cmd.Config
	if err := env.Parse(&cfg); err != nil {
		slog.Error("Failed to parse ENV", "error", err)
		return err
	}
	slog.InfoContext(context.Background(), fmt.Sprintf("collector started on %s", cfg.Listen), "event", "startup")

	// Initialize Hawk Catcher
	if cfg.HawkEnabled {
		err = hawk.Init()
		if err != nil {
			slog.Error("✗ Cannot initialize Hawk Catcher", "error", err)
		} else {
			go hawk.HawkCatcher.Run()
			defer hawk.HawkCatcher.Stop()
			slog.Info("✓ Hawk Catcher initialized", "url", hawk.HawkCatcher.GetURL())
		}
	}

	// connect to AMQP broker with retries
	slog.Info("Connecting to RabbitMQ", "url", cfg.BrokerURL)
	brokerObj := broker.New(cfg.BrokerURL, cfg.Exchange)
	brokerObj.Init()
	slog.Info("✓ Broker initialized", "url", cfg.BrokerURL)

	// connect to Redis
	slog.Info("Connecting to Redis", "url", cfg.RedisURL)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	redisClient := redis.New(ctx,
		cfg.RedisURL,
		cfg.RedisPassword,
		cfg.RedisDisabledProjectsSet,
		cfg.RedisBlacklistIPsSet,
		cfg.RedisAllIPsMap,
		cfg.RedisCurrentPeriodMap,
	)

	err = redisClient.LoadBlockedIDs()
	if err != nil {
		slog.Error("failed to load blocked IDs from Redis", "error", err)
	}

	// connect to accounts MongoDB
	doneAccountsContext := make(chan struct{})
	accountsClient := accounts.New(cfg.AccountsMongoDBURI)

	err = accountsClient.UpdateTokenCache()
	if err != nil {
		slog.Error("failed to update token cache", "error", err)
	}

	err = accountsClient.UpdateProjectsLimitsCache()
	if err != nil {
		slog.Error("failed to update projects limits cache", "error", err)
	}

	go periodic.RunPeriodically(accountsClient.UpdateTokenCache, cfg.TokenUpdatePeriod, doneAccountsContext)
	go periodic.RunPeriodically(accountsClient.UpdateProjectsLimitsCache, cfg.ProjectsLimitsUpdatePeriod, doneAccountsContext)
	defer close(doneAccountsContext)

	// start HTTP and websocket server
	serverObj := server.New(cfg, brokerObj, redisClient, accountsClient, cfg.BlacklistThreshold, cfg.NotifyURL)

	done := make(chan struct{})
	go periodic.RunPeriodically(redisClient.LoadBlockedIDs, cfg.BlockedIDsLoad, done)
	go periodic.RunPeriodically(serverObj.UpdateBlacklist, cfg.BlacklistUpdatePeriod, done)
	defer close(done)
	slog.Info("✓ Redis client initialized")

	// listen and serve prometheus metrics
	go metrics.RunServer(cfg.MetricsListen)

	serverObj.Run()

	return nil
}
