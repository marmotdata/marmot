package webhook

import (
	"encoding/json"
	"time"
)

// GenericProvider formats messages as a simple JSON payload for custom integrations.
type GenericProvider struct{}

type genericPayload struct {
	Type      string                 `json:"type"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

func (p *GenericProvider) FormatMessage(notification WebhookNotification) ([]byte, error) {
	payload := genericPayload{
		Type:      notification.Type,
		Title:     notification.Title,
		Message:   notification.Message,
		Data:      notification.Data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	return json.Marshal(payload)
}

func (p *GenericProvider) ContentType() string {
	return "application/json"
}
