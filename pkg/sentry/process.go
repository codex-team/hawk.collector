package sentry

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

// ProcessResult is the outcome of processing one envelope item.
type ProcessResult string

const (
	ProcessResultProcessed ProcessResult = "processed"
	ProcessResultSkipped   ProcessResult = "skipped"
)

// ProcessEnvelope parses a raw Sentry envelope and transforms event items into Hawk messages.
// Non-event items are skipped (client_report is logged then skipped). Same behavior as the Node worker.
func ProcessEnvelope(raw []byte, projectID string) ([]*HawkBrokerPayload, error) {
	envelope, err := ParseEnvelope(raw)
	if err != nil {
		return nil, err
	}

	if len(envelope.Items) == 0 {
		log.Warn("Received envelope with no items")
		return nil, nil
	}

	messages := make([]*HawkBrokerPayload, 0)
	processedCount := 0
	skippedCount := 0

	for _, item := range envelope.Items {
		result, msg, err := handleEnvelopeItem(envelope.Headers, item, projectID)
		if err != nil {
			return nil, err
		}
		switch result {
		case ProcessResultProcessed:
			processedCount++
			messages = append(messages, msg)
		case ProcessResultSkipped:
			skippedCount++
		}
	}

	log.Debugf("Processed %d events, skipped %d non-event items from envelope", processedCount, skippedCount)
	return messages, nil
}

func handleEnvelopeItem(envelopeHeaders json.RawMessage, item EnvelopeItem, projectID string) (ProcessResult, *HawkBrokerPayload, error) {
	// Same as Node: missing / null payload is an error for any item type
	if len(item.Payload) == 0 || string(item.Payload) == "null" {
		return "", nil, fmt.Errorf("Item payload is missing")
	}

	itemType := item.ItemType()
	if itemType != "event" {
		if itemType == "client_report" {
			logClientReport(envelopeHeaders, item)
		}
		log.Infof("Skipping non-event item of type: %s", itemType)
		return ProcessResultSkipped, nil, nil
	}

	msg, err := TransformToHawkFormat(envelopeHeaders, item, projectID)
	if err != nil {
		return "", nil, err
	}
	return ProcessResultProcessed, msg, nil
}

func logClientReport(envelopeHeaders json.RawMessage, item EnvelopeItem) {
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("Failed to decode/log client_report item: %v", r)
			log.Infof("👇 Here is the raw client_report item: header=%v payload=%s", item.Header, string(item.Payload))
		}
	}()

	var decoded interface{}
	if err := json.Unmarshal(item.Payload, &decoded); err != nil {
		decoded = string(item.Payload)
	}

	log.Info("Received client_report item; logging internals:")
	log.Infof("envelopeHeaders=%s itemHeader=%v payload=%v", string(envelopeHeaders), item.Header, decoded)

	// Also surface structured fields when present (helps debugging)
	if gjson.GetBytes(item.Payload, "discarded_events").Exists() {
		log.Debugf("client_report discarded_events=%s", gjson.GetBytes(item.Payload, "discarded_events").Raw)
	}
}
