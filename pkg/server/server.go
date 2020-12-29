package server

import (
	"errors"

	"github.com/codex-team/hawk.collector/cmd"
	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/codex-team/hawk.collector/pkg/hawk"
	"github.com/codex-team/hawk.collector/pkg/redis"
	"github.com/codex-team/hawk.collector/pkg/server/errorshandler"
	"github.com/codex-team/hawk.collector/pkg/server/sourcemapshandler"
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

	// handler for sourcemap processing
	SourcemapsHander sourcemapshandler.Handler
	RedisClient      *redis.RedisClient
}

// New creates new server and initiates it with link to the broker and copy of configuration parameters
func New(c cmd.Config, b *broker.Broker, r *redis.RedisClient) *Server {
	return &Server{
		Broker:      b,
		Config:      c,
		RedisClient: r,
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
		JwtSecret:                  s.Config.JwtSecret,
		MaxErrorCatcherMessageSize: s.Config.MaxErrorCatcherMessageSize,
		ErrorsProcessed:            promauto.NewCounter(prometheus.CounterOpts{Name: "collection_errors_processed_ops_total"}),
		RedisClient:                s.RedisClient,
	}

	// handler of sourcemap messages via HTTP
	s.SourcemapsHander = sourcemapshandler.Handler{
		SourcemapExchange:              s.Config.SourcemapExchange,
		Broker:                         s.Broker,
		JwtSecret:                      s.Config.JwtSecret,
		MaxSourcemapCatcherMessageSize: s.Config.MaxSourcemapCatcherMessageSize,
		RedisClient:                    s.RedisClient,
	}

	log.Infof("✓ collector starting on %s", s.Config.Listen)

	err := fastHTTPServer.ListenAndServe(s.Config.Listen)
	cmd.FailOnError(err, "Server run error")
}

// global fasthttp entrypoint
func (s *Server) handler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("text/json; charset=utf8")

	var err error
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
	case "/ws":
		s.ErrorsHandler.HandleWebsocket(ctx)
	case "/sourcemap":
		s.SourcemapsHander.HandleHTTP(ctx)
	default:
		ctx.Error("Not found", fasthttp.StatusNotFound)
	}

}
