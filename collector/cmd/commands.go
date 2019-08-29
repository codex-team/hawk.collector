package cmd

// Run server in the production mode
type RunCommand struct {
	ConfigurationFilename string `short:"C" long:"config" description:"Configuration filename" required:"true"`
	Broker                string `short:"B" long:"broker" description:"Connection URL for broker" required:"false"`
	Exchange              string `short:"E" long:"exchange" description:"Base exchange name" required:"false"`
	Host                  string `short:"H" long:"host" description:"Server host" required:"false"`
	Port                  int    `short:"P" long:"port" description:"Server port" required:"false"`
	RetryNumber           int    `long:"retry_number" description:"Number of connection retries" required:"false"`
	RetryInterval         uint   `long:"retry_interval" description:"Period between retries in seconds" required:"false"`
}
