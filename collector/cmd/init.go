package cmd

import (
	"fmt"
	"os"

	"github.com/codex-team/hawk.collector/collector/configuration"

	"github.com/AlecAivazis/survey"
)

// Default configuration parameters for the setup wizard
var (
	DefaultBrokerURL      = "amqp://guest:guest@localhost:5672/"
	DefaultExchange       = "errors"
	DefaultHost           = "localhost"
	DefaultPort           = "3000"
	DefaultConfigFilename = "config/config.json"
)

// Configuration wizard survey
var configurationWizard = []*survey.Question{
	{
		Name: "BrokerURL",
		Prompt: &survey.Input{
			Message: "Input broker URL",
			Default: DefaultBrokerURL,
		},
	},
	{
		Name: "Exchange",
		Prompt: &survey.Input{
			Message: "Input exchange name",
			Default: DefaultExchange,
		},
	},
	{
		Name: "Host",
		Prompt: &survey.Input{
			Message: "Input server Host",
			Default: DefaultHost,
		},
	},
	{
		Name: "Port",
		Prompt: &survey.Input{
			Message: "Input server Port",
			Default: DefaultPort,
		},
	},
}

// Execute initial setup
// It creates configuration file with the help of setup wizard
func (x *InitCommand) Execute(args []string) error {
	var config = &configuration.Configuration{}
	var configFilename string

	prompt := &survey.Input{
		Message: "Input config filename",
		Default: DefaultConfigFilename,
	}

	err := survey.AskOne(prompt, &configFilename, nil)
	if err != nil {
		return err
	}

	// if configuration file does not exist
	if _, err := os.Stat(configFilename); !os.IsNotExist(err) {
		overwrite := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("File %s exists. Do you want to overwrite it", configFilename),
		}

		err = survey.AskOne(prompt, &overwrite, nil)
		if err != nil {
			return err
		}

		// exit if user does not want to overwrite file
		if !overwrite {
			return nil
		}
	}

	// fill the config
	err = survey.Ask(configurationWizard, config)
	if err != nil {
		return err
	}

	// fill retry parameters from cli arguments
	config.RetryNumber = x.RetryNumber
	config.RetryInterval = x.RetryInterval

	// save configuration file
	err = config.Save(configFilename)
	if err != nil {
		return err
	}

	return nil
}
