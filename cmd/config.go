package cmd

import "time"

// Config is loaded via .env file
type Config struct {
	// AMQP full connection URI
	BrokerURL string `env:"BROKER_URL"`

	// Exchange name for error messages
	Exchange string `env:"EXCHANGE"`

	// Exchange name for sourcemap messages
	SourcemapExchange string `env:"SOURCEMAP_EXCHANGE"`

	// Number of retries for connection to AMQP server during initialization
	RetryNumber int `env:"RETRY_NUMBER"`

	// Interval between retries for connection to AMQP server during initialization
	RetryInterval uint `env:"RETRY_INTERVAL"`

	// JWT secret
	JwtSecret string `env:"JWT_SECRET"`

	// Maximum HTTP body size (global)
	MaxRequestBodySize int `env:"MAX_REQUEST_BODY_SIZE"`

	// Maximum POST body size in bytes for error messages
	MaxErrorCatcherMessageSize int `env:"MAX_ERROR_CATCHER_MESSAGE_SIZE"`

	// Maximum POST body size in bytes for sourcemap messages
	MaxSourcemapCatcherMessageSize int `env:"MAX_SOURCEMAP_CATCHER_MESSAGE_SIZE"`

	// Listen HOST:PORT
	Listen string `env:"LISTEN"`

	// Log level
	LogLevel string `env:"LOG_LEVEL"`

	// Metrics listen host:port
	MetricsListen string `env:"METRICS_LISTEN"`

	// Whether to enable Hawk Catcher
	HawkEnabled bool `env:"HAWK_ENABLED" defaultEnv:"false"`

	RedisURL      string `env:"REDIS_URL"`
	RedisPassword string `env:"REDIS_PASSWORD"`

	RedisDisabledProjectsSet string `env:"REDIS_DISABLED_PROJECT_SET"`
	RedisBlacklistIPsSet     string `env:"REDIS_BLACKLIST_IP_SET"`
	RedisAllIPsMap           string `env:"REDIS_ALL_IPS_MAP"`
	RedisCurrentPeriodMap    string `env:"REDIS_CURRENT_PERIOD_MAP"`

	BlockedIDsLoad time.Duration `env:"BLOCKED_PROJECTS_UPDATE_PERIOD"`

	BlacklistUpdatePeriod time.Duration `env:"BLACKLIST_UPDATE_PERIOD"`
	BlacklistThreshold    int           `env:"BLACKLIST_THRESHOLD"`

	NotifyURL string `env:"NOTIFY_URL"`
}
