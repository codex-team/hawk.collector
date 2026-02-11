package server

import (
	"testing"

	"github.com/valyala/fasthttp"
)

func TestHandleGenerateTestTimeSeries_MethodNotAllowed(t *testing.T) {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod(fasthttp.MethodGet)
	req.SetRequestURI("/test/generate-timeseries")

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)

	srv := &Server{}
	srv.HandleGenerateTestTimeSeries(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusMethodNotAllowed, ctx.Response.StatusCode())
	}
}

func TestHandleGenerateTestTimeSeries_InvalidJSON(t *testing.T) {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("/test/generate-timeseries")
	req.SetBodyString("{bad json")

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)

	srv := &Server{}
	srv.HandleGenerateTestTimeSeries(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusBadRequest, ctx.Response.StatusCode())
	}
}

func TestHandleGenerateTestTimeSeries_MissingProjectId(t *testing.T) {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("/test/generate-timeseries")
	req.SetBodyString(`{"projectId": ""}`)

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)

	srv := &Server{}
	srv.HandleGenerateTestTimeSeries(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusBadRequest, ctx.Response.StatusCode())
	}
}
