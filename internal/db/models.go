package db

import (
	"time"
)

type HL7Message struct {
	ID               string    `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	Direction        string    `json:"direction"` // "order" or "report"
	SourceAddr       string    `json:"source_addr"`
	DestinationAddr  string    `json:"destination_addr"`
	MessageType      string    `json:"message_type"`
	MessageControlID string    `json:"message_control_id"`
	PatientID        string    `json:"patient_id"`
	PatientName      string    `json:"patient_name"`
	RawMessage       []byte    `json:"raw_message"`
	Status           string    `json:"status"` // "pending", "forwarded", "failed"
	RetryCount       int       `json:"retry_count"`
	LastError        string    `json:"last_error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	ProcessedAt      *time.Time `json:"processed_at,omitempty"`
}

type StreamInfo struct {
	Name          string `json:"name"`
	Messages      uint64 `json:"messages"`
	Bytes         uint64 `json:"bytes"`
	FirstSequence uint64 `json:"first_sequence"`
	LastSequence  uint64 `json:"last_sequence"`
}

type ConsumerInfo struct {
	Stream         string `json:"stream"`
	Name           string `json:"name"`
	Pending        uint64 `json:"pending"`
	Delivered      uint64 `json:"delivered"`
	AckPending     uint64 `json:"ack_pending"`
	RedeliveryCount uint64 `json:"redelivery_count"`
}