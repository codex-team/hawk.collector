package server

import (
	"bytes"
	"net"
	"testing"

	"github.com/codex-team/hawk.collector/cmd"
	"github.com/codex-team/hawk.collector/pkg/server/errorshandler"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func TestBodyLimitByPath(t *testing.T) {
	s := &Server{
		Config: cmd.Config{MaxRequestBodySize: 1024 * 1024, MaxErrorCatcherMessageSize: 1024},
		ErrorsHandler: errorshandler.Handler{
			ErrorsRejectedMessageTooLarge: prometheus.NewCounter(prometheus.CounterOpts{Name: "test_rejected"}),
		},
	}

	handled := 0
	ln := fasthttputil.NewInmemoryListener()
	defer ln.Close()
	go (&fasthttp.Server{
		Handler:            func(ctx *fasthttp.RequestCtx) { handled++ },
		MaxRequestBodySize: 1024 * 1024,
		HeaderReceived:     s.headerReceived,
		ErrorHandler:       s.errorHandler,
	}).Serve(ln) //nolint:errcheck

	client := &fasthttp.Client{Dial: func(string) (net.Conn, error) { return ln.Dial() }}
	post := func(uri string, size int) int {
		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseRequest(req)
		defer fasthttp.ReleaseResponse(resp)
		req.SetRequestURI("http://collector" + uri)
		req.Header.SetMethod(fasthttp.MethodPost)
		req.SetBody(bytes.Repeat([]byte("a"), size))
		require.NoError(t, client.Do(req, resp))
		return resp.StatusCode()
	}

	assert.Equal(t, fasthttp.StatusOK, post("/", 1024))
	assert.Equal(t, fasthttp.StatusOK, post("/release", 4096))
	assert.Equal(t, fasthttp.StatusRequestEntityTooLarge, post("/", 1025))
	assert.Equal(t, fasthttp.StatusRequestEntityTooLarge, post("/api/0/envelope/?sentry_key=x", 1025))
	assert.Equal(t, fasthttp.StatusOK, post("/release?x=1", 4096))
	assert.Equal(t, 3, handled)
}
