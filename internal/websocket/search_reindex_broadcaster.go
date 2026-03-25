package websocket

import (
	"encoding/json"
	"time"

	"github.com/rs/zerolog/log"
)

// SearchReindexBroadcaster broadcasts search reindex events via websockets.
type SearchReindexBroadcaster struct {
	hub *Hub
}

// NewSearchReindexBroadcaster creates a new search reindex broadcaster.
func NewSearchReindexBroadcaster(hub *Hub) *SearchReindexBroadcaster {
	return &SearchReindexBroadcaster{hub: hub}
}

func (b *SearchReindexBroadcaster) BroadcastStarted(total int) {
	b.publish(EventSearchReindexStarted, map[string]interface{}{
		"total": total,
	})
}

func (b *SearchReindexBroadcaster) BroadcastProgress(indexed, errors, total int) {
	b.publish(EventSearchReindexProgress, map[string]interface{}{
		"indexed": indexed,
		"errors":  errors,
		"total":   total,
	})
}

func (b *SearchReindexBroadcaster) BroadcastCompleted(indexed, errors, total int) {
	b.publish(EventSearchReindexCompleted, map[string]interface{}{
		"indexed": indexed,
		"errors":  errors,
		"total":   total,
	})
}

func (b *SearchReindexBroadcaster) BroadcastFailed(err error, indexed, errors, total int) {
	b.publish(EventSearchReindexFailed, map[string]interface{}{
		"error":   err.Error(),
		"indexed": indexed,
		"errors":  errors,
		"total":   total,
	})
}

func (b *SearchReindexBroadcaster) publish(eventType EventType, payload map[string]interface{}) {
	event := Event{
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal search reindex event")
		return
	}

	if err := b.hub.Publish("search_reindex", data); err != nil {
		log.Error().Err(err).Str("event_type", string(eventType)).Msg("Failed to publish search reindex event")
	}
}
