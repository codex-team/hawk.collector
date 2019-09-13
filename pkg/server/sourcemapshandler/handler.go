package sourcemapshandler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/codex-team/hawk.collector/cmd"
	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/dgrijalva/jwt-go"
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
	var files []SourcemapFile
	releaseValues, ok := form.Value["release"]
	if !ok {
		return ResponseMessage{true, "Provide `release` form value"}
	}
	if len(releaseValues) != 1 {
		return ResponseMessage{true, "Provide single `release` form value"}
	}

	// Validate JWT
	projectId, err := handler.DecodeJWT(token)
	if err != nil {
		return ResponseMessage{true, fmt.Sprintf("%s", err)}
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
	rawMessage, err := json.Marshal(messageToSend)
	cmd.PanicOnError(err)

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

	if tokenData.ProjectId == "" {
		return "", errors.New("empty projectId")
	}

	return tokenData.ProjectId, nil
}
