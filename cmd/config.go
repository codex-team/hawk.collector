package cmd

type Config struct {
	BrokerURL                      string `env:"BROKER_URL"`
	Exchange                       string `env:"EXCHANGE"`
	RetryNumber                    int    `env:"RETRY_NUMBER"`
	RetryInterval                  uint   `env:"RETRY_INTERVAL"`
	JwtSecret                      string `env:"JWT_SECRET"`
	MaxHTTPBodySize                int    `env:"MAX_HTTP_BODY_SIZE"`
	MaxErrorCatcherMessageSize     int    `env:"MAX_ERROR_CATCHER_MESSAGE_SIZE"`
	MaxSourcemapCatcherMessageSize int    `env:"MAX_SOURCEMAP_CATCHER_MESSAGE_SIZE"`
	Listen                         string `env:"LISTEN"`
	SourcemapExchange              string `env:"SOURCEMAP_EXCHANGE"`
}
