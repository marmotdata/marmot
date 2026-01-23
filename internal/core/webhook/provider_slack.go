package webhook

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SlackProvider formats messages for Slack incoming webhooks using Block Kit.
type SlackProvider struct{}

type slackPayload struct {
	Text   string       `json:"text"`
	Blocks []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type     string          `json:"type"`
	Text     *slackTextObj   `json:"text,omitempty"`
	Elements []slackTextObj  `json:"elements,omitempty"`
	Fields   []slackTextObj  `json:"fields,omitempty"`
}

type slackTextObj struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (p *SlackProvider) FormatMessage(notification WebhookNotification) ([]byte, error) {
	blocks := []slackBlock{
		{
			Type: "header",
			Text: &slackTextObj{
				Type: "plain_text",
				Text: truncate(notification.Title, 150),
			},
		},
		{
			Type: "section",
			Text: &slackTextObj{
				Type: "mrkdwn",
				Text: notification.Message,
			},
		},
	}

	// Add data fields if present
	if len(notification.Data) > 0 {
		fields := buildSlackFields(notification.Data)
		if len(fields) > 0 {
			blocks = append(blocks, slackBlock{
				Type:   "section",
				Fields: fields,
			})
		}
	}

	// Add context with notification type and timestamp
	blocks = append(blocks, slackBlock{
		Type: "context",
		Elements: []slackTextObj{
			{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*%s* | %s", formatNotificationType(notification.Type), time.Now().UTC().Format(time.RFC3339)),
			},
		},
	})

	payload := slackPayload{
		Text:   fmt.Sprintf("%s: %s", notification.Title, notification.Message),
		Blocks: blocks,
	}
	return json.Marshal(payload)
}

func (p *SlackProvider) ContentType() string {
	return "application/json"
}

func buildSlackFields(data map[string]interface{}) []slackTextObj {
	var fields []slackTextObj

	fieldKeys := []string{"asset_name", "asset_mrn", "pipeline_name", "status", "link"}
	for _, key := range fieldKeys {
		if val, ok := data[key]; ok {
			strVal := fmt.Sprintf("%v", val)
			if strVal == "" {
				continue
			}
			label := formatFieldLabel(key)
			if key == "link" {
				fields = append(fields, slackTextObj{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*%s:*\n<%s|View>", label, strVal),
				})
			} else {
				fields = append(fields, slackTextObj{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*%s:*\n%s", label, strVal),
				})
			}
		}
	}

	return fields
}

func formatFieldLabel(key string) string {
	switch key {
	case "asset_name":
		return "Asset"
	case "asset_mrn":
		return "MRN"
	case "pipeline_name":
		return "Pipeline"
	case "link":
		return "Link"
	default:
		return strings.ReplaceAll(key, "_", " ")
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
