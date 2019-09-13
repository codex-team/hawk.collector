package cmd

import (
	"log"
	"strings"
)

// FailOnError - throw fatal error and log it with the message provided by the msg argument if err is not nil
func FailOnError(err error, msgs ...string) {
	if err != nil {
		log.Fatalf("%s: %s", strings.Join(msgs, ". "), err)
	}
}

func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}
