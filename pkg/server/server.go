package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/codex-team/hawk.collector/pkg/accounts"

	"github.com/codex-team/hawk.collector/cmd"
	"github.com/codex-team/hawk.collector/pkg/alerts"
	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/codex-team/hawk.collector/pkg/hawk"
	"github.com/codex-team/hawk.collector/pkg/redis"
	"github.com/codex-team/hawk.collector/pkg/server/errorshandler"
	"github.com/codex-team/hawk.collector/pkg/server/releasehandler"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

// Server represents fasthttp server
type Server struct {
	Broker *broker.Broker

	// configuration from .env
	Config cmd.Config

	// handler for errors processing
	ErrorsHandler errorshandler.Handler

	// handler for release processing
	ReleaseHandler        releasehandler.Handler
	RedisClient           *redis.RedisClient
	AccountsMongoDBClient *accounts.AccountsMongoDBClient

	BlacklistThreshold int
	NotifyURL          string
}

// New creates new server and initiates it with link to the broker and copy of configuration parameters
func New(configuration cmd.Config, brokerClient *broker.Broker, redisClient *redis.RedisClient, accountsMongoDBClient *accounts.AccountsMongoDBClient, threshold int, notifyURL string) *Server {
	return &Server{
		Broker:                brokerClient,
		Config:                configuration,
		RedisClient:           redisClient,
		AccountsMongoDBClient: accountsMongoDBClient,
		BlacklistThreshold:    threshold,
		NotifyURL:             notifyURL,
	}
}

// Run server
func (s *Server) Run() {
	fastHTTPServer := fasthttp.Server{
		// global handler
		Handler: s.handler,

		// limit HTTP body size
		MaxRequestBodySize: s.Config.MaxRequestBodySize,
	}

	// handler of error messages via HTTP and websocket protocols
	s.ErrorsHandler = errorshandler.Handler{
		Broker:                     s.Broker,
		MaxErrorCatcherMessageSize: s.Config.MaxErrorCatcherMessageSize,
		ErrorsBlockedByLimit:       promauto.NewCounter(prometheus.CounterOpts{Name: "collection_errors_blocked_by_limit_total"}),
		ErrorsProcessed:            promauto.NewCounter(prometheus.CounterOpts{Name: "collection_errors_processed_ops_total"}),
		RedisClient:                s.RedisClient,
		AccountsMongoDBClient:      s.AccountsMongoDBClient,
	}

	// handler of sourcemap messages via HTTP
	s.ReleaseHandler = releasehandler.Handler{
		ReleaseExchange:              s.Config.ReleaseExchange,
		Broker:                       s.Broker,
		MaxReleaseCatcherMessageSize: s.Config.MaxReleaseCatcherMessageSize,
		RedisClient:                  s.RedisClient,
		AccountsMongoDBClient:        s.AccountsMongoDBClient,
	}

	log.Infof("✓ collector starting on %s", s.Config.Listen)

	err := fastHTTPServer.ListenAndServe(s.Config.Listen)
	cmd.FailOnError(err, "Server run error")
}

// global fasthttp entrypoint
func (s *Server) handler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("text/json; charset=utf8")

	var err error
	remoteIP := string(ctx.Request.Header.Peek(http.CanonicalHeaderKey("X-Forwarded-For")))
	if len(remoteIP) > 0 {
		isBlocked := s.RedisClient.CheckBlacklist(remoteIP)
		if isBlocked {
			ctx.Error("Too Many Requests", fasthttp.StatusTooManyRequests)
			return
		}

		err = s.RedisClient.IncrementIP(remoteIP)
		if err != nil {
			log.Errorf("failed to increment IP in database: %s", err.Error())
		}
	} else {
		log.Errorf("failed to get remote IP")
	}

	defer func() {
		r := recover()
		if r != nil {
			switch t := r.(type) {
			case string:
				err = errors.New(t)
			case error:
				err = t
			default:
				err = errors.New("unknown error")
			}

			log.Errorf("Recovered after error: %s", err)
			hawk.Catch(err)
			ctx.Error("Bad request", fasthttp.StatusBadRequest)
		}
	}()

	switch string(ctx.Path()) {
	case "/":
		s.ErrorsHandler.HandleHTTP(ctx)
	case "/health":
		s.HandleHealth(ctx)
	case "/ws":
		s.ErrorsHandler.HandleWebsocket(ctx)
	case "/release":
		s.ReleaseHandler.HandleHTTP(ctx)
	default:
		ctx.Error("Not found", fasthttp.StatusNotFound)
	}
}

func (s *Server) UpdateBlacklist() error {
	ipAddrs, requests, err := s.RedisClient.LoadBlacklist()
	if err != nil {
		return err
	}

	if len(ipAddrs) == 0 || len(requests) == 0 {
		return nil
	}

	for i := 0; i < len(ipAddrs); i++ {
		requestsQty, _ := strconv.Atoi(requests[i])
		if requestsQty >= s.BlacklistThreshold {
			err = alerts.Notify(s.NotifyURL, fmt.Sprintf("Hawk Collector (production) ⚠️\n\nTo many messages from %s\n%s", ipAddrs[i], requests[i]))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
