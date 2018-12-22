package main

import (
	"github.com/codex-team/hawk.catcher/cmd"
	"github.com/jessevdk/go-flags"
	"os"
)

var opts struct {
	Init cmd.InitCommand `command:"init" description:"Initialize server"`
	Run  cmd.RunCommand  `command:"run" description:"Run server"`
}

func main() {
	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(0)
	}
}
