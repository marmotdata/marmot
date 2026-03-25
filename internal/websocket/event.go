package websocket

import (
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

	EventSearchReindexStarted   EventType = "search_reindex_started"
	EventSearchReindexProgress  EventType = "search_reindex_progress"
	EventSearchReindexCompleted EventType = "search_reindex_completed"
	EventSearchReindexFailed    EventType = "search_reindex_failed"
)

// Event represents a websocket event to broadcast
type Event struct {
	Type      EventType              `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
}

