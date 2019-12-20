package cmd

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
	MaxHTTPBodySize int `env:"MAX_HTTP_BODY_SIZE"`

	// Maximum POST body size in bytes for error messages
	MaxErrorCatcherMessageSize int `env:"MAX_ERROR_CATCHER_MESSAGE_SIZE"`

	// Maximum POST body size in bytes for sourcemap messages
	MaxSourcemapCatcherMessageSize int `env:"MAX_SOURCEMAP_CATCHER_MESSAGE_SIZE"`

	// Listen HOST:PORT
	Listen string `env:"LISTEN"`
}
