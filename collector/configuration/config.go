package configuration

import (
	"encoding/json"
	"io/ioutil"
)

// Configuration parameters to connect to the queue
type Configuration struct {
	BrokerURL     string `json:"brokerUrl"`
	Exchange      string `json:"exchangeName"`
	Host          string `json:"serverHost"`
	Port          int    `json:"serverPort"`
	RetryNumber   int    `json:"retryNumber"`
	RetryInterval uint   `json:"retryInterval"`
	JwtSecret     string `json:"jwtSecret"`
}

// Load - load configuration from the file specified
func (config *Configuration) Load(filename string) error {
	plainText, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(plainText, config)
}

// Check - check configuration
func (config *Configuration) Check() error {
	return nil
}

// Save - save configuration to the file specified
func (config *Configuration) Save(filename string) error {
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
