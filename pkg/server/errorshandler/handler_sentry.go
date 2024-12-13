package errorshandler

import (
	"encoding/json"
	"fmt"

	"github.com/codex-team/hawk.collector/pkg/broker"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

const SentryQueueName = "errors/sentry"
const CatcherType = "sentry"

// HandleHTTP processes HTTP requests with JSON body
func (handler *Handler) HandleSentry(ctx *fasthttp.RequestCtx) {
	if ctx.Request.Header.ContentLength() > handler.MaxErrorCatcherMessageSize {
		log.Warnf("Incoming request with size %d", ctx.Request.Header.ContentLength())
		sendAnswerHTTP(ctx, ResponseMessage{Code: 400, Error: true, Message: "Request is too large"})
		return
	}

	// check that X-Sentry-Auth header is available
	auth := ctx.Request.Header.Peek("X-Sentry-Auth")
	if auth == nil {
		log.Warnf("Incoming request without X-Sentry-Auth header")
		sendAnswerHTTP(ctx, ResponseMessage{Code: 400, Error: true, Message: "X-Sentry-Auth header is missing"})
		return
	}

	hawkToken, err := getSentryKeyFromAuth(string(auth))
	if err != nil {
		log.Warnf("Incoming request with invalid X-Sentry-Auth header: %s", err)
		sendAnswerHTTP(ctx, ResponseMessage{Code: 400, Error: true, Message: err.Error()})
		return
	}

	log.Debugf("Incoming request with hawk integration token: %s", hawkToken)

	body := ctx.PostBody()

	sentryEnvelopeBody, err := decompressGzipString(body)
	if err != nil {
		log.Warnf("Failed to decompress gzip body: %s", err)
		sendAnswerHTTP(ctx, ResponseMessage{Code: 400, Error: true, Message: "Failed to decompress gzip body"})
		return
	}
	log.Debugf("Decompressed body: %s", sentryEnvelopeBody)

	projectId, ok := handler.AccountsMongoDBClient.ValidTokens[hawkToken]
	if !ok {
		log.Debugf("Token %s is not in the accounts cache", hawkToken)
		sendAnswerHTTP(ctx, ResponseMessage{400, true, fmt.Sprintf("Integration token invalid: %s", hawkToken)})
		return
	}
	log.Debugf("Found project with ID %s for integration token %s", projectId, hawkToken)

	if handler.RedisClient.IsBlocked(projectId) {
		handler.ErrorsBlockedByLimit.Inc()
		sendAnswerHTTP(ctx, ResponseMessage{402, true, "Project has exceeded the events limit"})
		return
	}

	// convert message to JSON format
	rawMessage := RawSentryMessage{Envelope: sentryEnvelopeBody}
	jsonMessage, err := json.Marshal(rawMessage)
	if err != nil {
		log.Errorf("Message marshalling error: %v", err)
		sendAnswerHTTP(ctx, ResponseMessage{400, true, "Cannot serialize envelope"})
	}

	messageToSend := BrokerMessage{ProjectId: projectId, Payload: json.RawMessage(jsonMessage), CatcherType: CatcherType}
	payloadToSend, err := json.Marshal(messageToSend)
	if err != nil {
		log.Errorf("Message marshalling error: %v", err)
		sendAnswerHTTP(ctx, ResponseMessage{400, true, "Cannot serialize envelope"})
	}

	// send serialized message to a broker
	brokerMessage := broker.Message{Payload: payloadToSend, Route: SentryQueueName}
	log.Debugf("Send to queue: %s", brokerMessage)
	handler.Broker.Chan <- brokerMessage

	// increment processed errors counter
	handler.ErrorsProcessed.Inc()

	sendAnswerHTTP(ctx, ResponseMessage{200, false, "OK"})
}
