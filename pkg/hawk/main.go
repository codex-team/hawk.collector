package hawk

import (
	"errors"

	"github.com/caarlos0/env/v6"
	hawkGo "github.com/codex-team/hawk.go"
	log "github.com/sirupsen/logrus"
)

// Global catcher instance
var HawkCatcher *hawkGo.Catcher

// Initialize global Hawk Catcher
func Init() error {
	var cfg HawkCatcherConfig

	// load settings from ENV
	if err := env.Parse(&cfg); err != nil {
		return errors.New("Failed to parse ENV")
	}

	// Initialize Catcher with Websocket transport
	catcher, err := hawkGo.New(cfg.Token, hawkGo.NewWebsocketSender())
	if err != nil {
		return err
	}

	// Set URL for Websocket transport
	if err := catcher.SetURL(cfg.URL); err != nil {
		return err
	}

	// Set defaults from config
	catcher.MaxBulkSize = cfg.BulkSize
	catcher.SourceCodeEnabled = cfg.SourceCodeEnabled

	// Assign global variable
	HawkCatcher = catcher

	return nil
}

func Catch(incomingError error) {
	if incomingError == nil {
		return
	}
	if HawkCatcher == nil {
		return
	}

	err := HawkCatcher.Catch(incomingError)
	if err != nil {
		log.Errorf("Error during Hawk Catch: %s\n", err)
	}
}
