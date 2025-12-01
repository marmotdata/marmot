package websocket

import (
	"encoding/json"

	"github.com/marmotdata/marmot/internal/core/runs"
)

// JobRunBroadcaster broadcasts job run events via websockets
type JobRunBroadcaster struct {
	hub *Hub
}

// NewJobRunBroadcaster creates a new job run broadcaster
func NewJobRunBroadcaster(hub *Hub) *JobRunBroadcaster {
	return &JobRunBroadcaster{hub: hub}
}

func (b *JobRunBroadcaster) BroadcastJobRunCreated(run *runs.JobRun) {
	payload := jobRunToMap(run)
	b.hub.Broadcast(EventJobRunCreated, payload)
}

func (b *JobRunBroadcaster) BroadcastJobRunClaimed(run *runs.JobRun) {
	payload := jobRunToMap(run)
	b.hub.Broadcast(EventJobRunClaimed, payload)
}

func (b *JobRunBroadcaster) BroadcastJobRunStarted(run *runs.JobRun) {
	payload := jobRunToMap(run)
	b.hub.Broadcast(EventJobRunStarted, payload)
}

func (b *JobRunBroadcaster) BroadcastJobRunProgress(run *runs.JobRun) {
	payload := jobRunToMap(run)
	b.hub.Broadcast(EventJobRunProgress, payload)
}

func (b *JobRunBroadcaster) BroadcastJobRunCompleted(run *runs.JobRun) {
	payload := jobRunToMap(run)
	b.hub.Broadcast(EventJobRunCompleted, payload)
}

func (b *JobRunBroadcaster) BroadcastJobRunCancelled(run *runs.JobRun) {
	payload := jobRunToMap(run)
	b.hub.Broadcast(EventJobRunCancelled, payload)
}

// jobRunToMap converts a JobRun to a map for JSON serialization
func jobRunToMap(run *runs.JobRun) map[string]interface{} {
	// Marshal and unmarshal to convert struct to map
	data, _ := json.Marshal(run)
	var payload map[string]interface{}
	json.Unmarshal(data, &payload)
	return payload
}
