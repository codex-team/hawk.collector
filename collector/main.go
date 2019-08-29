package main

import (
	"github.com/codex-team/hawk.collector/collector/cmd"
	"github.com/jessevdk/go-flags"
	"os"
)

// Command-line interface options
var opts struct {
	Run cmd.RunCommand `command:"run" description:"Run server"`
}

func main() {
	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(0)
	}
}
