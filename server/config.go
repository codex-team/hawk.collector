package server

type Config struct {
	BrokerURL     string `env:"BROKER_URL"`
	Exchange      string `env:"EXCHANGE"`
	Host          string `env:"HOST"`
	Port          int    `env:"PORT"`
	RetryNumber   int    `env:"RETRY_NUMBER"`
	RetryInterval uint   `env:"RETRY_INTERVAL"`
	JwtSecret     string `env:"JWT_SECRET"`
}
