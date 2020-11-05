package hawk

// HawkCatcherConfig is loaded from environment (ex: .env file)
type HawkCatcherConfig struct {
	// Hawk token
	Token string `env:"HAWK_TOKEN"`

	// Cache size
	BulkSize int `env:"HAWK_BULK_SIZE" envDefault:"10"`

	// Whether enable source code reading
	SourceCodeEnabled bool `env:"HAWK_SOURCE_CODE_ENABLED"  envDefault:"false"`

	// Hawk collector URL
	URL string `env:"HAWK_URL"`
}
