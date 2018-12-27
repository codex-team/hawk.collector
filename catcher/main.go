package main

import (
	"os"

	"github.com/codex-team/hawk.catcher/catcher/cmd"

	"github.com/jessevdk/go-flags"
)

// Command-line interface options
var opts struct {
	Init cmd.InitCommand `command:"init" description:"Initialize server configuration"`
	Run  cmd.RunCommand  `command:"run" description:"Run server"`
}

func main() {
	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(0)
	}
}
