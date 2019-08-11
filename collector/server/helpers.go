package server

import "log"

// failOnError - throw fatal error and log it with the message provided by the msg argument if err is not nil
func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}
