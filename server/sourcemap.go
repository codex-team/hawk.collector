package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/codex-team/hawk.collector/lib"
	"github.com/valyala/fasthttp"
	"io"
	"log"
	"mime/multipart"
)

// Route name where to send sourcemaps
const sourcemapRoute = "release/javascript"

// SourcemapFile represents file content and its name
type SourcemapFile struct {
	Name    string `json:"name"`
	Payload []byte `json:"payload"`
}

// SourcemapMessage represents message structure for sending to queue
type SourcemapMessage struct {
	ProjectId string          `json:"projectId"`
	Release   string          `json:"release"`
	Files     []SourcemapFile `json:"files"`
}

// sourcemapUploadHandler processes HTTP request for sourcemap uploading
func sourcemapUploadHandler(ctx *fasthttp.RequestCtx) {
	log.Printf("%s sourcemapUploadHandler request from %s", ctx.Method(), ctx.RemoteIP())

	token := ctx.Request.Header.Peek("Authorization")
	if len(token) < 8 {
		sendAnswer(ctx, Response{true, "Provide Authorization header", fasthttp.StatusBadRequest})
	}
	// cut "Bearer "
	token = token[7:]

	form, err := ctx.MultipartForm()
	if err != nil {
		log.Printf("Error: %s", err)
		sendAnswer(ctx, Response{true, "Multipart form is not provided", fasthttp.StatusBadRequest})
	} else {
		sendAnswer(ctx, uploadSourcemap(form, token))
	}
}

// uploadSourcemap - send sourcemaps to queue
func uploadSourcemap(form *multipart.Form, token []byte) Response {
	var files []SourcemapFile
	releaseValues, ok := form.Value["release"]
	if !ok {
		return Response{true, "Provide `release` form value", fasthttp.StatusInternalServerError}
	}
	if len(releaseValues) != 1 {
		return Response{true, "Provide single `release` form value", fasthttp.StatusInternalServerError}
	}

	// Validate JWT
	projectId, err := DecodeJWT(string(token))
	if err != nil {
		return Response{true, fmt.Sprintf("%v", err), fasthttp.StatusBadRequest}
	}

	release := releaseValues[0]
	for _, v := range form.File {
		for _, header := range v {
			f, _ := header.Open()
			defer f.Close()
			buf := bytes.NewBuffer(nil)
			_, err := io.Copy(buf, f)
			if err != nil {
				break
			}
			files = append(files, SourcemapFile{Name: header.Filename, Payload: buf.Bytes()})
		}
	}
	messageToSend := SourcemapMessage{ProjectId: projectId, Files: files, Release: release}

	// Marshal JSON to string to send to queue
	minifiedMessage, err := json.Marshal(messageToSend)
	if err != nil {
		log.Printf("JSON compression error: %v", err)
		return Response{true, "Server error", fasthttp.StatusInternalServerError}
	}

	messagesQueue <- lib.Message{Payload: minifiedMessage, Route: sourcemapRoute}
	return Response{false, "OK", fasthttp.StatusOK}
}
