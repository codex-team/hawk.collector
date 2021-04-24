package cmd

// Config is loaded via .env file
type Config struct {
	// AMQP full connection URI
	BrokerURL string `env:"BROKER_URL"`

	// Exchange name for error messages
	Exchange string `env:"EXCHANGE"`

	// Exchange name for release messages
	ReleaseExchange string `env:"RELEASE_EXCHANGE"`

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

	// Maximum POST body size in bytes for release messages
	MaxReleaseCatcherMessageSize int `env:"MAX_RELEASE_CATCHER_MESSAGE_SIZE"`

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
	RedisSet      string `env:"REDIS_DISABLED_PROJECT_SET"`
}
