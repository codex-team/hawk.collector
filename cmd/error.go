package cmd

import (
	"log/slog"
	"os"
	"strings"

	"github.com/codex-team/hawk.collector/pkg/hawk"
)

// FailOnError - throw fatal error and log it with the message provided by the msg argument if err is not nil
func FailOnError(err error, msgs ...string) {
	if err == nil {
		return
	}

	if hawkError := hawk.HawkCatcher.Catch(err); hawkError != nil {
		slog.Error("Error in HawkCatcher.Catch", "error", hawkError)
	}

	hawk.HawkCatcher.Stop()
	slog.Error(strings.Join(msgs, ". "), "error", err)
	os.Exit(1)
}

// PanicOnError - throw a recoverable panic
func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}
