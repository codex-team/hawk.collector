package server

type Config struct {
	BrokerURL     string `yaml:"brokerUrl"`
	Exchange      string `yaml:"exchangeName"`
	Host          string `yaml:"serverHost"`
	Port          int    `yaml:"serverPort"`
	RetryNumber   int    `yaml:"retryNumber"`
	RetryInterval uint   `yaml:"retryInterval"`
	JwtSecret     string `yaml:"jwtSecret"`
}
