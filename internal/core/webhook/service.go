package webhook

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/crypto"
	"github.com/rs/zerolog/log"
)

const (
	ProviderSlack   = "slack"
	ProviderDiscord = "discord"
	ProviderGeneric = "generic"
)

var (
	ErrNotFound = errors.New("webhook not found")

	ValidProviders = map[string]bool{
		ProviderSlack:   true,
		ProviderDiscord: true,
		ProviderGeneric: true,
	}
)

// Webhook represents a team webhook configuration.
type Webhook struct {
	ID                string     `json:"id"`
	TeamID            string     `json:"team_id"`
	Name              string     `json:"name"`
	Provider          string     `json:"provider"`
	WebhookURL        string     `json:"webhook_url"`
	NotificationTypes []string   `json:"notification_types"`
	Enabled           bool       `json:"enabled"`
	LastTriggeredAt   *time.Time `json:"last_triggered_at,omitempty"`
	LastError         *string    `json:"last_error,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// CreateWebhookInput is the input for creating a webhook.
type CreateWebhookInput struct {
	TeamID            string   `json:"team_id"`
	Name              string   `json:"name"`
	Provider          string   `json:"provider"`
	WebhookURL        string   `json:"webhook_url"`
	NotificationTypes []string `json:"notification_types"`
	Enabled           *bool    `json:"enabled"`
}

// UpdateWebhookInput is the input for updating a webhook.
type UpdateWebhookInput struct {
	Name              *string  `json:"name,omitempty"`
	WebhookURL        *string  `json:"webhook_url,omitempty"`
	NotificationTypes []string `json:"notification_types,omitempty"`
	Enabled           *bool    `json:"enabled,omitempty"`
}

// Service handles webhook operations.
type Service struct {
	repo       Repository
	encryptor  *crypto.Encryptor
	dispatcher *Dispatcher
}

// NewService creates a new webhook service.
func NewService(repo Repository, encryptor *crypto.Encryptor, dispatcher *Dispatcher) *Service {
	return &Service{
		repo:       repo,
		encryptor:  encryptor,
		dispatcher: dispatcher,
	}
}

// Create creates a new webhook.
func (s *Service) Create(ctx context.Context, input CreateWebhookInput) (*Webhook, error) {
	if err := s.validateCreate(input); err != nil {
		return nil, err
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	webhookURL := input.WebhookURL
	if s.encryptor != nil {
		encrypted, err := s.encryptor.EncryptString(webhookURL)
		if err != nil {
			return nil, fmt.Errorf("encrypting webhook URL: %w", err)
		}
		webhookURL = encrypted
	}

	webhook := &Webhook{
		TeamID:            input.TeamID,
		Name:              input.Name,
		Provider:          input.Provider,
		WebhookURL:        webhookURL,
		NotificationTypes: input.NotificationTypes,
		Enabled:           enabled,
	}

	if err := s.repo.Create(ctx, webhook); err != nil {
		return nil, err
	}

	// Return with masked URL
	webhook.WebhookURL = maskURL(input.WebhookURL)
	return webhook, nil
}

// Get retrieves a webhook by ID with the URL decrypted.
func (s *Service) Get(ctx context.Context, id string) (*Webhook, error) {
	webhook, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.decryptURL(webhook)
	return webhook, nil
}

// GetMasked retrieves a webhook by ID with the URL masked.
func (s *Service) GetMasked(ctx context.Context, id string) (*Webhook, error) {
	webhook, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	webhook.WebhookURL = maskURL(webhook.WebhookURL)
	return webhook, nil
}

// Update updates a webhook.
func (s *Service) Update(ctx context.Context, id string, input UpdateWebhookInput) (*Webhook, error) {
	if err := s.validateUpdate(input); err != nil {
		return nil, err
	}

	// Encrypt new URL if provided
	if input.WebhookURL != nil && s.encryptor != nil {
		encrypted, err := s.encryptor.EncryptString(*input.WebhookURL)
		if err != nil {
			return nil, fmt.Errorf("encrypting webhook URL: %w", err)
		}
		input.WebhookURL = &encrypted
	}

	webhook, err := s.repo.Update(ctx, id, input)
	if err != nil {
		return nil, err
	}

	s.decryptURL(webhook)
	webhook.WebhookURL = maskURL(webhook.WebhookURL)
	return webhook, nil
}

// Delete deletes a webhook.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// ListByTeam lists all webhooks for a team with masked URLs.
func (s *Service) ListByTeam(ctx context.Context, teamID string) ([]*Webhook, error) {
	webhooks, err := s.repo.ListByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	for _, w := range webhooks {
		s.decryptURL(w)
		w.WebhookURL = maskURL(w.WebhookURL)
	}

	return webhooks, nil
}

// TestWebhook sends a test notification to the webhook.
func (s *Service) TestWebhook(ctx context.Context, id string) error {
	webhook, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	notification := WebhookNotification{
		Type:    "system",
		Title:   "Test Notification",
		Message: "This is a test notification from Marmot to verify your webhook configuration.",
		Data: map[string]interface{}{
			"test": true,
		},
	}

	s.dispatcher.Dispatch(webhook, notification)
	return nil
}

// DispatchToTeam finds matching webhooks for a team and dispatches the notification.
// This implements the ExternalNotifier interface for the notification service.
func (s *Service) DispatchToTeam(ctx context.Context, teamID, notificationType, title, message string, data map[string]interface{}) {
	webhooks, err := s.repo.GetEnabledForNotificationType(ctx, teamID, notificationType)
	if err != nil {
		log.Error().Err(err).
			Str("team_id", teamID).
			Str("type", notificationType).
			Msg("Failed to get webhooks for team")
		return
	}

	if len(webhooks) == 0 {
		return
	}

	notification := WebhookNotification{
		Type:    notificationType,
		Title:   title,
		Message: message,
		Data:    data,
	}

	for _, webhook := range webhooks {
		s.decryptURL(webhook)
		s.dispatcher.Dispatch(webhook, notification)
	}
}

func (s *Service) decryptURL(webhook *Webhook) {
	if s.encryptor == nil {
		return
	}
	decrypted, err := s.encryptor.DecryptString(webhook.WebhookURL)
	if err != nil {
		// URL might not be encrypted (pre-encryption data or AllowUnencrypted mode)
		log.Debug().Err(err).Str("webhook_id", webhook.ID).Msg("Could not decrypt webhook URL, using as-is")
		return
	}
	webhook.WebhookURL = decrypted
}

func (s *Service) validateCreate(input CreateWebhookInput) error {
	if input.TeamID == "" {
		return &ValidationError{Message: "team_id is required"}
	}
	if strings.TrimSpace(input.Name) == "" {
		return &ValidationError{Message: "name is required"}
	}
	if len(input.Name) > 255 {
		return &ValidationError{Message: "name must be 255 characters or less"}
	}
	if !ValidProviders[input.Provider] {
		return &ValidationError{Message: fmt.Sprintf("invalid provider: %q, must be one of: slack, discord, generic", input.Provider)}
	}
	if strings.TrimSpace(input.WebhookURL) == "" {
		return &ValidationError{Message: "webhook_url is required"}
	}
	if err := validateWebhookURL(input.WebhookURL); err != nil {
		return err
	}
	if len(input.NotificationTypes) == 0 {
		return &ValidationError{Message: "at least one notification type is required"}
	}
	return nil
}

func (s *Service) validateUpdate(input UpdateWebhookInput) error {
	if input.Name != nil {
		if strings.TrimSpace(*input.Name) == "" {
			return &ValidationError{Message: "name cannot be empty"}
		}
		if len(*input.Name) > 255 {
			return &ValidationError{Message: "name must be 255 characters or less"}
		}
	}
	if input.WebhookURL != nil {
		if strings.TrimSpace(*input.WebhookURL) == "" {
			return &ValidationError{Message: "webhook_url cannot be empty"}
		}
		if err := validateWebhookURL(*input.WebhookURL); err != nil {
			return err
		}
	}
	if input.NotificationTypes != nil && len(input.NotificationTypes) == 0 {
		return &ValidationError{Message: "at least one notification type is required"}
	}
	return nil
}

// validateWebhookURL validates a webhook URL for format and SSRF safety.
func validateWebhookURL(rawURL string) *ValidationError {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return &ValidationError{Message: "webhook_url must be a valid HTTP(S) URL"}
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return &ValidationError{Message: "webhook_url is not a valid URL"}
	}

	host := parsed.Hostname()
	if host == "" {
		return &ValidationError{Message: "webhook_url must have a host"}
	}

	// Block known private/local hostnames
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return &ValidationError{Message: "webhook_url cannot target localhost or loopback addresses"}
	}

	// Resolve and check for private IPs
	ips, err := net.LookupHost(host)
	if err == nil {
		for _, ipStr := range ips {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				return &ValidationError{Message: "webhook_url cannot target private or internal network addresses"}
			}
		}
	}

	return nil
}

// ValidationError represents a user-facing validation failure.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// IsValidationError reports whether err is a user-facing validation error.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// maskURL masks a webhook URL to show only the last 4 characters after the last slash.
func maskURL(url string) string {
	if len(url) <= 8 {
		return "****"
	}

	lastSlash := strings.LastIndex(url, "/")
	if lastSlash == -1 || lastSlash >= len(url)-1 {
		return url[:8] + "...****"
	}

	prefix := url[:lastSlash+1]
	suffix := url[lastSlash+1:]

	if len(suffix) <= 4 {
		return prefix + "****"
	}

	return prefix + "..." + suffix[len(suffix)-4:]
}
