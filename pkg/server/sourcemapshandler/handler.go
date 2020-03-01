package sourcemapshandler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"io"
	"mime/multipart"
)

type Handler struct {
	SourcemapExchange              string
	Broker                         *broker.Broker
	MaxSourcemapCatcherMessageSize int
	JwtSecret                      string
}

func (handler *Handler) process(form *multipart.Form, token string) ResponseMessage {
	releaseValues, ok := form.Value["release"]
	if !ok {
		return ResponseMessage{true, "Provide `release` form value"}
	}

	log.Debugf("[sourcemaps] Got releaseValues: %s", releaseValues)

	if len(releaseValues) != 1 {
		return ResponseMessage{true, "Provide single `release` form value"}
	}

	// Validate JWT token
	projectId, err := handler.DecodeJWT(token)
	if err != nil {
		return ResponseMessage{true, fmt.Sprintf("%s", err)}
	}

	// peek first release value
	release := releaseValues[0]

	var files []SourcemapFile

	for _, v := range form.File { // for each File part in multipart form
		for _, header := range v { // for each MIME-style header
			f, _ := header.Open()
			defer f.Close()

			// copy file bytes to a buffer
			buf := bytes.NewBuffer(nil)
			_, err := io.Copy(buf, f)
			if err != nil {
				break
			}

			// append file name and content to files array
			log.Debugf("[sourcemaps] Got filename: %s", header.Filename)
			files = append(files, SourcemapFile{Name: header.Filename, Payload: buf.Bytes()})
		}
	}

	// convert message to JSON format
	messageToSend := SourcemapMessage{ProjectId: projectId, Files: files, Release: release}
	rawMessage, err := json.Marshal(messageToSend)
	cmd.PanicOnError(err)

	// send serialized message to a broker
	handler.Broker.Chan <- broker.Message{Payload: rawMessage, Route: handler.SourcemapExchange}
	return ResponseMessage{false, "OK"}
}

// DecodeJWT – check JWT and return projectId
func (handler *Handler) DecodeJWT(token string) (string, error) {
	var tokenData JWTClaim
	_, err := jwt.ParseWithClaims(token, &tokenData, func(token *jwt.Token) (interface{}, error) {
		return []byte(handler.JwtSecret), nil
	})
	if err != nil {
		return "", errors.New("invalid JWT signature")
	}

	log.Debugf("Token data: %s", tokenData)
	if tokenData.ProjectId == "" {
		return "", errors.New("empty projectId")
	}

	return tokenData.ProjectId, nil
}
