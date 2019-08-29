package server

import (
	"fmt"
	"github.com/codex-team/hawk.collector/collector/lib"
	"github.com/valyala/fasthttp"
	"log"
	"net"
)

var jwtSecret string

type Server struct {
	config   *Config
	amqpConn *lib.Connection
}

func New(c *Config) (*Server, error) {
	jwtSecret = c.JwtSecret
	return &Server{
		config:   c,
		amqpConn: nil,
	}, nil
}

// Initialize connection to the AMQP server
func (s Server) Connect() error {
	conn := lib.Connection{}
	err := conn.Init(s.config.BrokerURL, s.config.Exchange)
	if err != nil {
		return err
	}
	s.amqpConn = &conn
	return nil
}

// RunWorkers - run background worker which will read message from the channel and process it.
// There may be several workers with separate connections to the RabbitMQ
func (s Server) RunWorkers() bool {
	go func(conn lib.Connection, ch <-chan lib.Message) {
		for msg := range ch {
			_ = conn.Publish(msg)
		}
	}(*s.amqpConn, messagesQueue)
	return true
}

// Run HTTP server and block execution
func (s Server) Serve() {
	if err := fasthttp.ListenAndServe(net.JoinHostPort(s.config.Host, fmt.Sprintf("%d", s.config.Port)), requestHandler); err != nil {
		log.Fatalf("Serve error: %s", err)
	}
}

// Load appropriate request handlers for different protocols
func requestHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("text/json; charset=utf8")

	switch string(ctx.Path()) {
	case "/":
		catcherHTTPHandler(ctx)
	case "/sourcemap":
		sourcemapUploadHandler(ctx)
	case "/ws", "/ws/":
		catcherWebsocketsHandler(ctx)
	default:
		SendAnswer(ctx, Response{true, "Invalid path", fasthttp.StatusBadRequest})
	}
}
