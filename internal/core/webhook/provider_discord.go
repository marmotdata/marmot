package webhook

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DiscordProvider formats messages for Discord webhooks using embeds.
type DiscordProvider struct{}

type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Color       int            `json:"color"`
	Fields      []discordField `json:"fields,omitempty"`
	Footer      *discordFooter `json:"footer,omitempty"`
	Timestamp   string         `json:"timestamp"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordFooter struct {
	Text string `json:"text"`
}

func (p *DiscordProvider) FormatMessage(notification WebhookNotification) ([]byte, error) {
	embed := discordEmbed{
		Title:       truncate(notification.Title, 256),
		Description: truncate(notification.Message, 4096),
		Color:       discordColorForType(notification.Type),
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Footer: &discordFooter{
			Text: fmt.Sprintf("Marmot • %s", formatNotificationType(notification.Type)),
		},
	}

	// Add data fields
	if len(notification.Data) > 0 {
		embed.Fields = buildDiscordFields(notification.Data)
	}

	payload := discordPayload{Embeds: []discordEmbed{embed}}
	return json.Marshal(payload)
}

func (p *DiscordProvider) ContentType() string {
	return "application/json"
}

func discordColorForType(notifType string) int {
	switch notifType {
	case "schema_change", "upstream_schema_change", "downstream_schema_change":
		return 0xE67E22 // Orange
	case "asset_change":
		return 0x3498DB // Blue
	case "job_complete":
		return 0x2ECC71 // Green
	case "lineage_change":
		return 0x9B59B6 // Purple
	case "mention":
		return 0xF1C40F // Yellow
	case "team_invite":
		return 0x1ABC9C // Teal
	default:
		return 0x95A5A6 // Grey
	}
}

func buildDiscordFields(data map[string]interface{}) []discordField {
	var fields []discordField

	fieldKeys := []string{"asset_name", "asset_mrn", "pipeline_name", "status", "link"}
	for _, key := range fieldKeys {
		if val, ok := data[key]; ok {
			strVal := fmt.Sprintf("%v", val)
			if strVal == "" {
				continue
			}
			label := formatFieldLabel(key)
			if key == "link" {
				fields = append(fields, discordField{
					Name:   label,
					Value:  fmt.Sprintf("[View](%s)", strVal),
					Inline: false,
				})
			} else {
				fields = append(fields, discordField{
					Name:   label,
					Value:  strVal,
					Inline: true,
				})
			}
		}
	}

	return fields
}

// formatNotificationType converts internal type to human-readable label.
func formatNotificationType(t string) string {
	switch t {
	case "system":
		return "System"
	case "schema_change":
		return "Schema Change"
	case "asset_change":
		return "Asset Change"
	case "team_invite":
		return "Team Invite"
	case "mention":
		return "Mention"
	case "job_complete":
		return "Job Complete"
	case "upstream_schema_change":
		return "Upstream Schema Change"
	case "downstream_schema_change":
		return "Downstream Schema Change"
	case "lineage_change":
		return "Lineage Change"
	default:
		return strings.ReplaceAll(t, "_", " ")
	}
}
