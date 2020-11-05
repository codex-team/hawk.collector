package cmd

import (
	"github.com/codex-team/hawk.collector/pkg/hawk"
	"log"
	"strings"
)

// FailOnError - throw fatal error and log it with the message provided by the msg argument if err is not nil
func FailOnError(err error, msgs ...string) {
	hawk.HawkCatcher.Catch(err)
	hawk.HawkCatcher.Stop()
	if err != nil {
		log.Fatalf("%s: %s", strings.Join(msgs, ". "), err)
	}
}

// PanicOnError - throw a recoverable panic
func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}
