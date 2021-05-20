package releasehandler

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
	"github.com/tidwall/gjson"
)

type Handler struct {
	ReleaseExchange              string
	Broker                       *broker.Broker
	MaxReleaseCatcherMessageSize int
	JwtSecret                    string
	RedisClient                  *redis.RedisClient
}

const AddReleaseType string = "add-release"

// getSingleFormValue - returns the only value of the form or generates error
func getSingleFormValue(form *multipart.Form, key string) (error, string) {
	values, ok := form.Value[key]
	if !ok {
		return errors.New(fmt.Sprintf("provide `%s` form value", key)), ""
	}

	log.Debugf("[release] Got releaseValues: %s", values)

	if len(values) != 1 {
		return errors.New(fmt.Sprintf("provide single `%s` form value", key)), ""
	}

	return nil, values[0]
}

func (handler *Handler) process(form *multipart.Form, token string) ResponseMessage {
	err, release := getSingleFormValue(form, "release")
	if err != nil {
		return ResponseMessage{400, true, fmt.Sprintf("%s", err)}
	}
	err, catcherType := getSingleFormValue(form, "catcherType")
	if err != nil {
		return ResponseMessage{400, true, fmt.Sprintf("%s", err)}
	}
	err, commits := getSingleFormValue(form, "commits")
	if err != nil {
		return ResponseMessage{400, true, fmt.Sprintf("%s", err)}
	}

	// Validate JWT token
	projectId, err := handler.DecodeJWT(token)
	if err != nil {
		return ResponseMessage{400, true, fmt.Sprintf("%s", err)}
	}

	if handler.RedisClient.IsBlocked(projectId) {
		return ResponseMessage{402, true, "Project has exceeded the events limit"}
	}

	var files []ReleaseFile

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
			log.Debugf("[release] Got filename: %s", header.Filename)
			files = append(files, ReleaseFile{Name: header.Filename, Payload: buf.Bytes()})
		}
	}

	// Validate if message is a valid JSON
	stringMessage := string(commits)
	if !gjson.Valid(stringMessage) {
		return ResponseMessage{400, true, "Invalid commits JSON format"}
	}

	// convert message to JSON format
	messageToSend := ReleaseMessage{ProjectId: projectId, Type: AddReleaseType, Payload: ReleaseMessagePayload{Files: files, Release: release, CatcherType: catcherType, Commits: []byte(commits)}}
	rawMessage, err := json.Marshal(messageToSend)
	if err != nil {
		log.Errorf("Message marshalling error: %v", err)
		return ResponseMessage{400, true, "Cannot encode message to JSON"}
	}

	// send serialized message to a broker
	handler.Broker.Chan <- broker.Message{Payload: rawMessage, Route: handler.ReleaseExchange}
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
