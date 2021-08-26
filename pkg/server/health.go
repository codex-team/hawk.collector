package server

import (
	"encoding/json"

	"github.com/codex-team/hawk.collector/pkg/hawk"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type HealthStatus struct {
	RedisStatus   bool `json:"redis_status"`
	MongoDBStatus bool `json:"mongo_db_status"`
}

func (hs *HealthStatus) isAvailable() bool {
	return hs.RedisStatus && hs.MongoDBStatus
}

// HandleHealth handles health statuses of redis, rabbitmq and mongodb
func (s *Server) HandleHealth(ctx *fasthttp.RequestCtx) {
	healthStatus := HealthStatus{
		RedisStatus:   s.RedisClient.CheckAvailability(),
		MongoDBStatus: s.AccountsMongoDBClient.CheckAvailability(),
	}
	if healthStatus.isAvailable() {
		ctx.Response.SetStatusCode(200)
	} else {
		ctx.Response.SetStatusCode(500)
	}

	response, err := json.Marshal(healthStatus)
	if err != nil {
		log.Errorf("Error during response marshalling: %v", err)
		hawk.Catch(err)
		ctx.Response.SetStatusCode(500)
		ctx.SetConnectionClose()
		return
	}

	_, err = ctx.Write(response)
	if err != nil {
		log.Errorf("Error during response write: %v", err)
		hawk.Catch(err)
		ctx.Response.SetStatusCode(500)
		ctx.SetConnectionClose()
		return
	}
}
