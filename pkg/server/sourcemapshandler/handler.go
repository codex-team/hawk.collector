package sourcemapshandler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/codex-team/hawk.collector/pkg/redis"
	"github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
)

type Handler struct {
	SourcemapExchange              string
	Broker                         *broker.Broker
	MaxSourcemapCatcherMessageSize int
	JwtSecret                      string
	RedisClient                    *redis.RedisClient
}

func (handler *Handler) process(form *multipart.Form, token string) ResponseMessage {
	releaseValues, ok := form.Value["release"]
	if !ok {
		return ResponseMessage{400, true, "Provide `release` form value"}
	}

	log.Debugf("[sourcemaps] Got releaseValues: %s", releaseValues)

	if len(releaseValues) != 1 {
		return ResponseMessage{400, true, "Provide single `release` form value"}
	}

	// Validate JWT token
	projectId, err := handler.DecodeJWT(token)
	if err != nil {
		return ResponseMessage{400, true, fmt.Sprintf("%s", err)}
	}

	if handler.RedisClient.IsBlocked(projectId) {
		return ResponseMessage{402, true, "Project has exceeded the event limit"}
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
	if err != nil {
		log.Errorf("Message marshalling error: %v", err)
		return ResponseMessage{400, true, "Cannot encode message to JSON"}
	}

	// send serialized message to a broker
	handler.Broker.Chan <- broker.Message{Payload: rawMessage, Route: handler.SourcemapExchange}
	return ResponseMessage{200, false, "OK"}
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

	log.Debugf("Token data: %v", tokenData)
	if tokenData.ProjectId == "" {
		return "", errors.New("empty projectId")
	}

	return tokenData.ProjectId, nil
}
