package hawk

import (
	"errors"
	"github.com/caarlos0/env"
	hawkGo "github.com/codex-team/hawk.go"
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
