package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/AlecAivazis/survey"
	"io/ioutil"
	"os"
)

type Configuration struct {
	BrokerURL     string `json:"broker_url"`
	Exchange      string `json:"exchange_name"`
	Host          string `json:"server_host"`
	Port          int    `json:"server_port"`
	RetryNumber   int    `json:"retry_number"`
	RetryInterval uint   `json:"retry_interval"`
}

var (
	DefaultBrokerURL      = "amqp://guest:guest@localhost:5672/"
	DefaultExchange       = "errors"
	DefaultHost           = "localhost"
	DefaultPort           = "3000"
	DefaultConfigFilename = "config.json"
)

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

func (x *InitCommand) Execute(args []string) error {
	var config = &Configuration{}
	var configFilename string

	prompt := &survey.Input{
		Message: "Input config filename",
		Default: DefaultConfigFilename,
	}

	err := survey.AskOne(prompt, &configFilename, nil)
	if err != nil {
		return err
	}

	if _, err := os.Stat(configFilename); !os.IsNotExist(err) {
		overwrite := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("File %s exists. Do you want to overwrite it", configFilename),
		}
		err = survey.AskOne(prompt, &overwrite, nil)
		if err != nil {
			return err
		}

		if !overwrite {
			return nil
		}
	}

	// fill the config
	err = survey.Ask(configurationWizard, config)
	if err != nil {
		return err
	}

	config.RetryNumber = x.RetryNumber
	config.RetryInterval = x.RetryInterval

	err = config.save(configFilename)
	if err != nil {
		return err
	}

	return nil
}

func (config *Configuration) load(filename string) error {
	plainText, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(plainText, config)
}

func (config *Configuration) check() error {
	return nil
}

func (config *Configuration) save(filename string) error {
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filename, configJSON, 0644)
	if err != nil {
		return err
	}

	return nil
}
