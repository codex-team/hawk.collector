package periodic

import (
	"time"

	log "github.com/sirupsen/logrus"
)

type PeriodicFunc func() error

func RunPeriodically(task PeriodicFunc, interval time.Duration, done chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		for {
			select {
			case <-ticker.C:
				err := task()
				if err != nil {
					log.Errorf("failed to run task: %s", err.Error())
				}
			case <-done:
				return
			}
		}
	}()
	<-done
}
