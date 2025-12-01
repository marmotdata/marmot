package websocket

import (
	"encoding/json"
	"time"
)

// EventType represents the type of event being broadcast
type EventType string

const (
	EventJobRunCreated   EventType = "job_run_created"
	EventJobRunUpdated   EventType = "job_run_updated"
	EventJobRunClaimed   EventType = "job_run_claimed"
	EventJobRunStarted   EventType = "job_run_started"
	EventJobRunProgress  EventType = "job_run_progress"
	EventJobRunCompleted EventType = "job_run_completed"
	EventJobRunCancelled EventType = "job_run_cancelled"
)

// Event represents a websocket event to broadcast
type Event struct {
	Type      EventType              `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
}

// MarshalJSON marshals the event to JSON
func (e *Event) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type      EventType              `json:"type"`
		Payload   map[string]interface{} `json:"payload"`
		Timestamp time.Time              `json:"timestamp"`
	}{
		Type:      e.Type,
		Payload:   e.Payload,
		Timestamp: e.Timestamp,
	})
}
