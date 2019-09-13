package server

import (
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/codex-team/hawk.collector/pkg/server/errorshandler"
	"github.com/codex-team/hawk.collector/pkg/server/sourcemapshandler"
	"github.com/valyala/fasthttp"
)

type Server struct {
	Broker *broker.Broker
	Config cmd.Config
}

func New(c cmd.Config, b *broker.Broker) *Server {
	return &Server{
		Broker: b,
		Config: c,
	}
}

func (s *Server) Run() {
	fastHTTPServer := fasthttp.Server{
		Handler:            s.handler,
		MaxRequestBodySize: s.Config.MaxHTTPBodySize,
	}

	err := fastHTTPServer.ListenAndServe(s.Config.Listen)
	cmd.FailOnError(err, "Server run error")
}

func (s *Server) handler(ctx *fasthttp.RequestCtx) {
	errorsHandler := errorshandler.Handler{
		Broker:                     s.Broker,
		JwtSecret:                  s.Config.JwtSecret,
		MaxErrorCatcherMessageSize: s.Config.MaxErrorCatcherMessageSize,
	}

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
