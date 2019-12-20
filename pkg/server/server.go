package server

import (
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/codex-team/hawk.collector/pkg/server/errorshandler"
	"github.com/codex-team/hawk.collector/pkg/server/sourcemapshandler"
	"github.com/valyala/fasthttp"
)

// Server represents fasthttp server
type Server struct {
	Broker *broker.Broker

	// configuration from .env
	Config cmd.Config
}

// New creates new server and initiates it with link to the broker and copy of configuration parameters
func New(c cmd.Config, b *broker.Broker) *Server {
	return &Server{
		Broker: b,
		Config: c,
	}
}

// Run server
func (s *Server) Run() {
	fastHTTPServer := fasthttp.Server{
		// global handler
		Handler: s.handler,

		// limit HTTP body size
		MaxRequestBodySize: s.Config.MaxHTTPBodySize,
	}

	err := fastHTTPServer.ListenAndServe(s.Config.Listen)
	cmd.FailOnError(err, "Server run error")
}

// global fasthttp entrypoint
func (s *Server) handler(ctx *fasthttp.RequestCtx) {

	// handler of error messages via HTTP and websocket protocols
	errorsHandler := errorshandler.Handler{
		Broker:                     s.Broker,
		JwtSecret:                  s.Config.JwtSecret,
		MaxErrorCatcherMessageSize: s.Config.MaxErrorCatcherMessageSize,
	}

	// handler of sourcemap messages via HTTP
	sourcemapsHander := sourcemapshandler.Handler{
		SourcemapExchange:              s.Config.SourcemapExchange,
		Broker:                         s.Broker,
		JwtSecret:                      s.Config.JwtSecret,
		MaxSourcemapCatcherMessageSize: s.Config.MaxSourcemapCatcherMessageSize,
	}

	ctx.SetContentType("text/json; charset=utf8")

	switch string(ctx.Path()) {
	case "/":
		errorsHandler.HandleHTTP(ctx)
	case "/ws":
		errorsHandler.HandleWebsocket(ctx)
	case "/sourcemap":
		sourcemapsHander.HandleHTTP(ctx)
	default:
		ctx.Error("Not found", fasthttp.StatusNotFound)
	}
}
