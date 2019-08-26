package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"

	"github.com/codex-team/hawk.collector/collector/lib"
	"github.com/valyala/fasthttp"
)

const sourcemapQueue = "release/javascript"

type SourcemapFile struct {
	Name    string `json:"name"`
	Payload []byte `json:"payload"`
}

type SourcemapMessage struct {
	Token   string          `json:"token"`
	Release string          `json:"release"`
	Files   []SourcemapFile `json:"files"`
}

func UploadSourcemap(form *multipart.Form, token []byte) Response {
	var files []SourcemapFile
	releaseValues, ok := form.Value["release"]
	if !ok {
		return Response{true, "Provide `release` form value", fasthttp.StatusInternalServerError}
	}
	if len(releaseValues) != 1 {
		return Response{true, "Provide single `release` form value", fasthttp.StatusInternalServerError}
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
	messageToSend := SourcemapMessage{Token: string(token), Files: files, Release: release}

	// Marshal JSON to string to send to queue
	minifiedMessage, err := json.Marshal(messageToSend)
	if err != nil {
		log.Printf("JSON compression error: %v", err)
		return Response{true, "Server error", fasthttp.StatusInternalServerError}
	}

	messagesQueue <- lib.Message{Payload: minifiedMessage, Route: sourcemapQueue}
	return Response{false, "OK", fasthttp.StatusOK}
}
