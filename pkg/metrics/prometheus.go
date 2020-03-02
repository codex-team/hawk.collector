package metrics

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func RunServer(listenAddress string) {
	if listenAddress == "" {
		log.Errorf("✗ Prometheus metrics listenAddress is not set")
		return
	}
	http.Handle("/metrics", promhttp.Handler())
	log.Infof("✓ Prometheus metrics initialized on %s/metrics", listenAddress)

	if err := http.ListenAndServe(listenAddress, nil); err != nil {
		log.Errorf("Prometheus metrics listen error: %s", err)
	}
}
