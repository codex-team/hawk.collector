package cmd

// RunCommand - Run server in the production mode
type RunCommand struct {
	BrokerURL string `short:"B" long:"broker" description:"Connection URL for broker" required:"false"`
	Host      string `short:"H" long:"host" description:"Server host" required:"false"`
	Port      int    `short:"P" long:"port" description:"Server port" required:"false"`
}
