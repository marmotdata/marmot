package notification

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/marmotdata/marmot/internal/worker"
	"github.com/rs/zerolog/log"
)

const (
	RecipientTypeUser = "user"
	RecipientTypeTeam = "team"
)

const (
	TypeSystem       = "system"
	TypeSchemaChange = "schema_change"
	TypeAssetChange  = "asset_change"
	TypeTeamInvite   = "team_invite"
	TypeMention      = "mention"
	TypeJobComplete  = "job_complete"
)

const (
	MaxBatchSize     = 500
	DefaultBatchSize = 100

	DefaultPruneAge         = 90 * 24 * time.Hour
	DefaultPruneInterval    = 24 * time.Hour
	DefaultAggregateWindow  = 2 * time.Minute
	DefaultAggregateMaxWait = 5 * time.Minute
)

var (
	ErrNotificationNotFound = errors.New("notification not found")
	ErrUnauthorized         = errors.New("unauthorized to access notification")
)

// Recipient represents a notification target.
type Recipient struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Notification represents a user notification.
type Notification struct {
	ID            string                 `json:"id"`
	UserID        string                 `json:"user_id"`
	RecipientType string                 `json:"recipient_type"`
	RecipientID   string                 `json:"recipient_id"`
	Type          string                 `json:"type"`
	Title         string                 `json:"title"`
	Message       string                 `json:"message"`
	Data          map[string]interface{} `json:"data,omitempty"`
	Read          bool                   `json:"read"`
	ReadAt        *time.Time             `json:"read_at,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// CreateNotificationInput is used by internal services to create notifications.
type CreateNotificationInput struct {
	Recipients []Recipient            `json:"recipients" validate:"required,min=1"`
	Type       string                 `json:"type" validate:"required"`
	Title      string                 `json:"title" validate:"required,max=255"`
	Message    string                 `json:"message" validate:"required"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

// NotificationFilter for listing notifications.
type NotificationFilter struct {
	UserID   string
	Type     string
	ReadOnly *bool
	Cursor   string
	Limit    int
	Offset   int
}

// NotificationSummary provides unread count for UI badge.
type NotificationSummary struct {
	UnreadCount int `json:"unread_count"`
	TotalCount  int `json:"total_count"`
}

// ListResult contains paginated notification results.
type ListResult struct {
	Notifications []*Notification `json:"notifications"`
	Total         int             `json:"total"`
	NextCursor    string          `json:"next_cursor,omitempty"`
}

// TeamMembershipProvider provides team membership lookup for notification fan-out.
type TeamMembershipProvider interface {
	GetTeamMemberUserIDs(ctx context.Context, teamID string) ([]string, error)
}

// UserPreferencesProvider provides user notification preferences.
type UserPreferencesProvider interface {
	GetNotificationPreferences(ctx context.Context, userID string) (map[string]bool, error)
	GetNotificationPreferencesBatch(ctx context.Context, userIDs []string) (map[string]map[string]bool, error)
}

// ServiceConfig configures the notification service.
type ServiceConfig struct {
	MaxWorkers         int
	QueueSize          int
	BatchSize          int
	PruneAge           time.Duration
	PruneInterval      time.Duration
	AggregateWindow    time.Duration
	AggregateMaxWait   time.Duration
	DisableAggregation bool
}

// Service handles notification operations.
type Service struct {
	repo              Repository
	teamProvider      TeamMembershipProvider
	userPrefsProvider UserPreferencesProvider
	config            *ServiceConfig

	workerPool *worker.Pool
	aggregator *assetChangeAggregator

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewService creates a new notification service.
func NewService(repo Repository, teamProvider TeamMembershipProvider, opts ...ServiceOption) *Service {
	config := &ServiceConfig{
		MaxWorkers:       5,
		QueueSize:        200,
		BatchSize:        DefaultBatchSize,
		PruneAge:         DefaultPruneAge,
		PruneInterval:    DefaultPruneInterval,
		AggregateWindow:  DefaultAggregateWindow,
		AggregateMaxWait: DefaultAggregateMaxWait,
	}

	s := &Service{
		repo:         repo,
		teamProvider: teamProvider,
		config:       config,
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.config.BatchSize <= 0 {
		s.config.BatchSize = DefaultBatchSize
	}
	if s.config.BatchSize > MaxBatchSize {
		s.config.BatchSize = MaxBatchSize
	}

	s.workerPool = worker.NewPool(worker.PoolConfig{
		Name:       "notification-fanout",
		MaxWorkers: s.config.MaxWorkers,
		QueueSize:  s.config.QueueSize,
		OnJobComplete: func(job worker.Job, err error, duration time.Duration) {
			if err != nil {
				log.Error().
					Str("job_id", job.ID()).
					Err(err).
					Dur("duration", duration).
					Msg("Notification fan-out job failed")
			}
		},
	})

	return s
}

// ServiceOption configures the notification service.
type ServiceOption func(*Service)

// WithConfig sets the service configuration.
func WithConfig(config *ServiceConfig) ServiceOption {
	return func(s *Service) {
		if config != nil {
			s.config = config
		}
	}
}

// WithUserPreferencesProvider sets the user preferences provider.
func WithUserPreferencesProvider(provider UserPreferencesProvider) ServiceOption {
	return func(s *Service) {
		s.userPrefsProvider = provider
	}
}

// Start begins background processing.
func (s *Service) Start(ctx context.Context) {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.workerPool.Start(ctx)

	if !s.config.DisableAggregation {
		s.aggregator = newAssetChangeAggregator(s, s.config.AggregateWindow, s.config.AggregateMaxWait)
		s.aggregator.start(s.ctx)
	}

	s.wg.Add(1)
	go s.pruneLoop()

	log.Info().Msg("Notification service started")
}

// Stop gracefully shuts down the service.
func (s *Service) Stop() {
	log.Info().Msg("Stopping notification service...")

	if s.cancel != nil {
		s.cancel()
	}

	if s.aggregator != nil {
		s.aggregator.stop()
	}

	s.workerPool.Stop()
	s.wg.Wait()

	log.Info().Msg("Notification service stopped")
}

func (s *Service) pruneLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.PruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.pruneOldNotifications()
		}
	}
}

func (s *Service) pruneOldNotifications() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cutoff := time.Now().Add(-s.config.PruneAge)
	deleted, err := s.repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		log.Error().Err(err).Msg("Failed to prune old notifications")
		return
	}

	if deleted > 0 {
		log.Info().
			Int64("deleted", deleted).
			Time("cutoff", cutoff).
			Msg("Pruned old notifications")
	}
}

// Create queues notifications for the specified recipients.
func (s *Service) Create(ctx context.Context, input CreateNotificationInput) error {
	if len(input.Recipients) == 0 {
		return errors.New("at least one recipient is required")
	}

	job := &fanoutJob{
		svc:   s,
		input: input,
	}

	if !s.workerPool.Submit(job) {
		log.Warn().Msg("Notification queue full, processing synchronously")
		return job.Execute(ctx)
	}

	return nil
}

// CreateSync creates notifications synchronously.
func (s *Service) CreateSync(ctx context.Context, input CreateNotificationInput) (int, error) {
	if len(input.Recipients) == 0 {
		return 0, errors.New("at least one recipient is required")
	}

	return s.doFanout(ctx, input)
}

// QueueAssetChange queues an asset change for aggregated notification.
// changeType should be TypeAssetChange or TypeSchemaChange.
func (s *Service) QueueAssetChange(assetID, assetMRN, assetName, changeType string, owners []Recipient) {
	if s.aggregator == nil {
		return
	}
	s.aggregator.queue(assetID, assetMRN, assetName, changeType, owners)
}

func (s *Service) doFanout(ctx context.Context, input CreateNotificationInput) (int, error) {
	userRecipients := make(map[string]Recipient)

	for _, r := range input.Recipients {
		switch r.Type {
		case RecipientTypeUser:
			if existing, exists := userRecipients[r.ID]; !exists || existing.Type == RecipientTypeTeam {
				userRecipients[r.ID] = r
			}
		case RecipientTypeTeam:
			memberIDs, err := s.teamProvider.GetTeamMemberUserIDs(ctx, r.ID)
			if err != nil {
				log.Error().Err(err).Str("team_id", r.ID).Msg("Failed to get team members for notification")
				continue
			}
			for _, userID := range memberIDs {
				if _, exists := userRecipients[userID]; !exists {
					userRecipients[userID] = r
				}
			}
		default:
			log.Warn().Str("type", r.Type).Msg("Unknown recipient type")
		}
	}

	if len(userRecipients) == 0 {
		return 0, nil
	}

	userRecipients = s.filterByPreferences(ctx, userRecipients, input.Type)
	if len(userRecipients) == 0 {
		return 0, nil
	}

	return s.repo.CreateBatch(ctx, userRecipients, input, s.config.BatchSize)
}

func (s *Service) filterByPreferences(ctx context.Context, recipients map[string]Recipient, notifType string) map[string]Recipient {
	if s.userPrefsProvider == nil {
		return recipients
	}

	userIDs := make([]string, 0, len(recipients))
	for userID := range recipients {
		userIDs = append(userIDs, userID)
	}

	allPrefs, err := s.userPrefsProvider.GetNotificationPreferencesBatch(ctx, userIDs)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to batch load notification preferences, defaulting to enabled")
		return recipients
	}

	filtered := make(map[string]Recipient)
	for userID, recipient := range recipients {
		prefs, exists := allPrefs[userID]
		if !exists {
			filtered[userID] = recipient
			continue
		}

		enabled, hasKey := prefs[notifType]
		if !hasKey || enabled {
			filtered[userID] = recipient
		}
	}

	return filtered
}

// Get retrieves a single notification by ID.
func (s *Service) Get(ctx context.Context, id string) (*Notification, error) {
	return s.repo.Get(ctx, id)
}

// List retrieves notifications for a user with filters.
func (s *Service) List(ctx context.Context, filter NotificationFilter) (*ListResult, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	if filter.Cursor != "" {
		return s.repo.ListWithCursor(ctx, filter)
	}

	notifications, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	result := &ListResult{
		Notifications: notifications,
		Total:         total,
	}

	if len(notifications) > 0 && filter.Offset+len(notifications) < total {
		lastNotification := notifications[len(notifications)-1]
		result.NextCursor = lastNotification.CreatedAt.Format(time.RFC3339Nano)
	}

	return result, nil
}

// GetSummary returns unread/total count for a user.
func (s *Service) GetSummary(ctx context.Context, userID string) (*NotificationSummary, error) {
	return s.repo.GetSummary(ctx, userID)
}

// MarkAsRead marks a single notification as read.
func (s *Service) MarkAsRead(ctx context.Context, id, userID string) error {
	notification, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if notification.UserID != userID {
		return ErrUnauthorized
	}
	return s.repo.MarkAsRead(ctx, id)
}

// MarkAllAsRead marks all notifications for a user as read.
func (s *Service) MarkAllAsRead(ctx context.Context, userID string) error {
	return s.repo.MarkAllAsReadChunked(ctx, userID, 1000)
}

// Delete deletes a notification.
func (s *Service) Delete(ctx context.Context, id, userID string) error {
	notification, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if notification.UserID != userID {
		return ErrUnauthorized
	}
	return s.repo.Delete(ctx, id)
}

// DeleteAllRead deletes all read notifications for a user.
func (s *Service) DeleteAllRead(ctx context.Context, userID string) error {
	return s.repo.DeleteAllRead(ctx, userID)
}

type fanoutJob struct {
	svc   *Service
	input CreateNotificationInput
}

func (j *fanoutJob) ID() string {
	return fmt.Sprintf("notification-fanout:%s:%s", j.input.Type, j.input.Title)
}

func (j *fanoutJob) Execute(ctx context.Context) error {
	count, err := j.svc.doFanout(ctx, j.input)
	if err != nil {
		return err
	}

	log.Debug().
		Int("count", count).
		Str("type", j.input.Type).
		Msg("Notification fan-out complete")

	return nil
}
