// Package cdc consumes Debezium change events delivered through SNS -> SQS
// and routes them, strongly typed, to per-table handlers.
package cdc

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Op is the Debezium operation code.
type Op string

const (
	OpCreate   Op = "c"
	OpUpdate   Op = "u"
	OpDelete   Op = "d"
	OpRead     Op = "r" // snapshot read
	OpTruncate Op = "t"
)

// Source mirrors Debezium's io.debezium.connector.mysql.Source block.
type Source struct {
	Version   string  `json:"version"`
	Connector string  `json:"connector"`
	Name      string  `json:"name"`
	TsMs      int64   `json:"ts_ms"`
	Snapshot  string  `json:"snapshot"`
	DB        string  `json:"db"`
	Table     string  `json:"table"`
	ServerID  int64   `json:"server_id"`
	GTID      *string `json:"gtid"`
	File      string  `json:"file"`
	Pos       int64   `json:"pos"`
	Row       int     `json:"row"`
}

// Envelope is the Debezium envelope payload.
type Envelope[T any] struct {
	Before *T     `json:"before"`
	After  *T     `json:"after"`
	Source Source `json:"source"`
	Op     Op     `json:"op"`
	TsMs   int64  `json:"ts_ms"`
}

// RawEvent is a parsed-but-untyped change event.
type RawEvent struct {
	ID       string // SNS MessageId (empty with raw message delivery)
	TopicArn string
	Schema   json.RawMessage // Debezium schema block (nil if schemas.enable=false)
	Envelope[json.RawMessage]
	payload json.RawMessage
}

var ErrNotCDCEvent = errors.New("cdc: message is not a debezium change event")

type snsEnvelope struct {
	Type      string `json:"Type"`
	MessageID string `json:"MessageId"`
	TopicArn  string `json:"TopicArn"`
	Message   string `json:"Message"`
}

// ParseMessage accepts an SQS body, either an SNS notification wrapping the
// Debezium JSON (as in your sample) or the bare Debezium JSON (raw delivery).
// Both "schemas.enable=true" ({"schema":..,"payload":..}) and "false"
// (payload at top level) are supported. The schema block is ignored.
func ParseMessage(body string) (*RawEvent, error) {
	ev := &RawEvent{}
	inner := []byte(body)

	var sns snsEnvelope
	if err := json.Unmarshal(inner, &sns); err == nil && sns.Type == "Notification" && sns.Message != "" {
		ev.ID, ev.TopicArn = sns.MessageID, sns.TopicArn
		inner = []byte(sns.Message)
	}

	var wrapper struct {
		Schema  json.RawMessage `json:"schema"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(inner, &wrapper); err != nil {
		return nil, fmt.Errorf("cdc: decode envelope: %w", err)
	}
	raw := inner
	if len(wrapper.Payload) > 0 && string(wrapper.Payload) != "null" {
		raw = wrapper.Payload
		ev.Schema = wrapper.Schema
	}
	if err := json.Unmarshal(raw, &ev.Envelope); err != nil {
		return nil, fmt.Errorf("cdc: decode payload: %w", err)
	}
	ev.payload = raw
	if ev.Op == "" || ev.Source.Table == "" {
		return nil, ErrNotCDCEvent
	}
	return ev, nil
}
