package errorshandler

import (
	"encoding/json"
	"fmt"

	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/codex-team/hawk.collector/pkg/sentry"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

// HandleSentry processes Sentry envelope HTTP requests.
// Parses the envelope, skips non-event items (no rate-limit / plot impact),
// transforms events to Hawk format, and publishes to errors/javascript or errors/default.
func (handler *Handler) HandleSentry(ctx *fasthttp.RequestCtx) {
	if ctx.Request.Header.ContentLength() > handler.MaxErrorCatcherMessageSize {
		handler.ErrorsRejectedMessageTooLarge.Inc()
		log.Warnf("Incoming request with size %d", ctx.Request.Header.ContentLength())
		sendAnswerHTTP(ctx, ResponseMessage{Code: 400, Error: true, Message: "Request is too large"})
		return
	}

	allowCORS(ctx)
	if string(ctx.Method()) == fasthttp.MethodOptions {
		ctx.SetStatusCode(fasthttp.StatusNoContent) // 204
		return
	}

	var hawkToken string
	var err error

	// parse incoming get request params
	sentryKey := ctx.QueryArgs().Peek("sentry_key")
	if sentryKey == nil {
		// check that X-Sentry-Auth header is available
		auth := ctx.Request.Header.Peek("X-Sentry-Auth")
		if auth == nil {
			log.Warnf("Incoming request without X-Sentry-Auth header")
			sendAnswerHTTP(ctx, ResponseMessage{Code: 400, Error: true, Message: "X-Sentry-Auth header is missing"})
			return
		}

		hawkToken, err = getSentryKeyFromAuth(string(auth))
		if err != nil {
			log.Warnf("Incoming request with invalid X-Sentry-Auth header=%s: %s", auth, err)
			sendAnswerHTTP(ctx, ResponseMessage{Code: 400, Error: true, Message: err.Error()})
			return
		}
	} else {
		hawkToken = string(sentryKey)
	}

	log.Debugf("Incoming request with hawk integration token: %s", hawkToken)

	sentryEnvelopeBody := ctx.PostBody()

	contentEncoding := string(ctx.Request.Header.Peek("Content-Encoding"))
	if contentEncoding == "gzip" {
		sentryEnvelopeBody, err = decompressGzipString(sentryEnvelopeBody)
		if err != nil {
			log.Warnf("Failed to decompress gzip body: %s", err)
			sendAnswerHTTP(ctx, ResponseMessage{Code: 400, Error: true, Message: "Failed to decompress gzip body"})
			return
		}
		log.Debugf("Decompressed gzip body: %s", sentryEnvelopeBody)
	} else if contentEncoding == "br" {
		sentryEnvelopeBody, err = decompressBrotliString(sentryEnvelopeBody)
		if err != nil {
			log.Warnf("Failed to decompress brotli body: %s", err)
			sendAnswerHTTP(ctx, ResponseMessage{Code: 400, Error: true, Message: "Failed to decompress brotli body"})
			return
		}
		log.Debugf("Decompressed brotli body: %s", sentryEnvelopeBody)
	} else {
		log.Debugf("Body: %s", sentryEnvelopeBody)
	}

	projectId, ok := handler.AccountsMongoDBClient.GetValidToken(hawkToken)
	if !ok {
		log.Warnf("Token %s is not in the accounts cache", hawkToken)
		sendAnswerHTTP(ctx, ResponseMessage{400, true, fmt.Sprintf("Integration token invalid: %s", hawkToken)})
		return
	}
	log.Debugf("Found project with ID %s for integration token %s", projectId, hawkToken)

	// Parse + transform before rate-limiting so non-event-only envelopes
	// do not affect rate limits or plots.
	messages, err := sentry.ProcessEnvelope(sentryEnvelopeBody, projectId)
	if err != nil {
		log.Errorf("Error handling Sentry envelope: %v", err)
		sendAnswerHTTP(ctx, ResponseMessage{400, true, "Failed to process Sentry envelope"})
		return
	}

	if len(messages) == 0 {
		// Non-event items only (or empty after filtering) — skip silently
		sendAnswerHTTP(ctx, ResponseMessage{200, false, "OK"})
		return
	}

	projectLimits, ok := handler.AccountsMongoDBClient.GetProjectLimits(projectId)
	if !ok {
		log.Warnf("Project %s is not in the projects limits cache", projectId)
	} else {
		log.Debugf("Project %s limits: %+v", projectId, projectLimits)
	}

	if handler.RedisClient.IsBlocked(projectId) {
		handler.ErrorsBlockedByLimit.Inc()
		handler.recordProjectMetrics(projectId, "events-rate-limited", false)
		sendAnswerHTTP(ctx, ResponseMessage{402, true, "Project has exceeded the events limit"})
		return
	}

	rateWithinLimit, err := handler.RedisClient.UpdateRateLimit(projectId, projectLimits.EventsLimit, projectLimits.EventsPeriod)
	if err != nil {
		log.Errorf("Failed to update rate limit: %s", err)
		sendAnswerHTTP(ctx, ResponseMessage{402, true, "Failed to update rate limit"})
		return
	}

	if !rateWithinLimit {
		handler.recordProjectMetrics(projectId, "events-rate-limited", false)
		sendAnswerHTTP(ctx, ResponseMessage{402, true, "Rate limit exceeded"})
		return
	}

	for _, msg := range messages {
		payloadBytes, err := json.Marshal(msg.Payload)
		if err != nil {
			log.Errorf("Message marshalling error: %v", err)
			sendAnswerHTTP(ctx, ResponseMessage{400, true, "Cannot serialize event payload"})
			return
		}

		messageToSend := BrokerMessage{
			Timestamp:   msg.Timestamp,
			ProjectId:   msg.ProjectID,
			Payload:     json.RawMessage(payloadBytes),
			CatcherType: msg.CatcherType,
		}
		payloadToSend, err := json.Marshal(messageToSend)
		if err != nil {
			log.Errorf("Message marshalling error: %v", err)
			sendAnswerHTTP(ctx, ResponseMessage{400, true, "Cannot serialize message"})
			return
		}

		route := handler.determineQueue(msg.CatcherType)
		brokerMessage := broker.Message{Payload: payloadToSend, Route: route}
		log.Debugf("Send to queue: %s", brokerMessage)
		handler.Broker.Chan <- brokerMessage
	}

	// One acceptance counter per HTTP request that yielded ≥1 event (same as before).
	handler.ErrorsProcessed.Inc()
	handler.recordProjectMetrics(projectId, "events-accepted", true)

	sendAnswerHTTP(ctx, ResponseMessage{200, false, "OK"})
}

// helper for CORS
func allowCORS(ctx *fasthttp.RequestCtx) {
	h := &ctx.Response.Header
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type, X-Sentry-Auth")
	h.Set("Access-Control-Max-Age", "86400")
}
