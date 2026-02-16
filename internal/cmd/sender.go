package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/minicodemonkey/chief/internal/uplink"
)

// messageSender is an interface for sending messages to the server.
// The uplink adapter satisfies this interface.
type messageSender interface {
	Send(msg interface{}) error
}

// uplinkSender adapts *uplink.Uplink to the messageSender interface.
// It JSON-marshals the message, extracts the "type" field for the batcher's
// priority tier classification, and enqueues via Uplink.Send().
type uplinkSender struct {
	uplink *uplink.Uplink
}

func newUplinkSender(u *uplink.Uplink) *uplinkSender {
	return &uplinkSender{uplink: u}
}

func (s *uplinkSender) Send(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	// Extract the "type" field for batcher tier classification.
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		log.Printf("uplinkSender: could not extract message type: %v", err)
		envelope.Type = "unknown"
	}

	s.uplink.Send(data, envelope.Type)
	return nil
}
