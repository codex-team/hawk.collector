package main

import (
	"github.com/codex-team/hawk.collector/cmd/collector"
	"github.com/jessevdk/go-flags"
	"os"
)

// Command-line interface options
var opts struct {
	Run collector.RunCommand `command:"run" description:"Run server"` // nolint: unused
}

func main() {
	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(0)
	}
}
