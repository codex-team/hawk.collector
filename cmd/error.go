package cmd

import (
	"github.com/codex-team/hawk.collector/pkg/hawk"
	"log"
	"strings"
)

// FailOnError - throw fatal error and log it with the message provided by the msg argument if err is not nil
func FailOnError(err error, msgs ...string) {
	if err == nil {
		return
	}

	if hawkError := hawk.HawkCatcher.Catch(err); hawkError != nil {
		log.Printf("Error in HawkCatcher.Catch: %s", hawkError)
	}

	hawk.HawkCatcher.Stop()
	log.Fatalf("%s: %s", strings.Join(msgs, ". "), err)
}

// PanicOnError - throw a recoverable panic
func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}
