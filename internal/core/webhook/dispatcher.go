package webhook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/marmotdata/marmot/internal/worker"
	"github.com/rs/zerolog/log"
)

// HTTPClient interface for testability.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// DispatcherConfig configures the webhook dispatcher.
type DispatcherConfig struct {
	MaxWorkers int
	QueueSize  int
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

// Dispatcher handles async delivery of webhook notifications.
type Dispatcher struct {
	repo       Repository
	registry   *ProviderRegistry
	httpClient HTTPClient
	workerPool *worker.Pool
	config     DispatcherConfig
}

// NewDispatcher creates a new webhook dispatcher.
func NewDispatcher(repo Repository, registry *ProviderRegistry, config DispatcherConfig) *Dispatcher {
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 5
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 100
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = time.Second
	}

	d := &Dispatcher{
		repo:     repo,
		registry: registry,
		config:   config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}

	d.workerPool = worker.NewPool(worker.PoolConfig{
		Name:       "webhook-dispatcher",
		MaxWorkers: config.MaxWorkers,
		QueueSize:  config.QueueSize,
		OnJobComplete: func(job worker.Job, err error, duration time.Duration) {
			if err != nil {
				log.Error().
					Str("job_id", job.ID()).
					Err(err).
					Dur("duration", duration).
					Msg("Webhook delivery job failed")
			}
		},
	})

	return d
}

// Start begins the dispatcher worker pool.
func (d *Dispatcher) Start(ctx context.Context) {
	d.workerPool.Start(ctx)
	log.Info().Msg("Webhook dispatcher started")
}

// Stop gracefully shuts down the dispatcher.
func (d *Dispatcher) Stop() {
	d.workerPool.Stop()
	log.Info().Msg("Webhook dispatcher stopped")
}

// Dispatch queues a webhook delivery job.
func (d *Dispatcher) Dispatch(webhook *Webhook, notification WebhookNotification) {
	job := &deliveryJob{
		dispatcher:   d,
		webhook:      webhook,
		notification: notification,
	}

	if !d.workerPool.Submit(job) {
		log.Warn().
			Str("webhook_id", webhook.ID).
			Str("webhook_name", webhook.Name).
			Msg("Webhook dispatch queue full, dropping notification")
	}
}

// nonRetryableError wraps errors that should not be retried.
type nonRetryableError struct {
	err error
}

func (e *nonRetryableError) Error() string { return e.err.Error() }
func (e *nonRetryableError) Unwrap() error { return e.err }

// deliveryJob implements worker.Job for webhook delivery.
type deliveryJob struct {
	dispatcher   *Dispatcher
	webhook      *Webhook
	notification WebhookNotification
}

func (j *deliveryJob) ID() string {
	return fmt.Sprintf("webhook-delivery:%s:%s", j.webhook.ID, j.notification.Type)
}

func (j *deliveryJob) Execute(ctx context.Context) error {
	provider, ok := j.dispatcher.registry.Get(j.webhook.Provider)
	if !ok {
		errMsg := fmt.Sprintf("unknown provider: %s", j.webhook.Provider)
		if err := j.dispatcher.repo.UpdateLastTriggered(ctx, j.webhook.ID, &errMsg); err != nil {
			log.Warn().Err(err).Str("webhook_id", j.webhook.ID).Msg("Failed to update last triggered")
		}
		return errors.New(errMsg)
	}

	body, err := provider.FormatMessage(j.notification)
	if err != nil {
		errMsg := fmt.Sprintf("format error: %v", err)
		if updateErr := j.dispatcher.repo.UpdateLastTriggered(ctx, j.webhook.ID, &errMsg); updateErr != nil {
			log.Warn().Err(updateErr).Str("webhook_id", j.webhook.ID).Msg("Failed to update last triggered")
		}
		return fmt.Errorf("formatting message: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= j.dispatcher.config.MaxRetries; attempt++ {
		lastErr = j.doHTTPPost(ctx, provider.ContentType(), body)
		if lastErr == nil {
			if err := j.dispatcher.repo.UpdateLastTriggered(ctx, j.webhook.ID, nil); err != nil {
				log.Warn().Err(err).Str("webhook_id", j.webhook.ID).Msg("Failed to update last triggered")
			}
			log.Debug().
				Str("webhook_id", j.webhook.ID).
				Str("webhook_name", j.webhook.Name).
				Str("type", j.notification.Type).
				Msg("Webhook delivered successfully")
			return nil
		}

		// Don't retry non-retryable errors (4xx client errors except 429)
		var nre *nonRetryableError
		if errors.As(lastErr, &nre) {
			log.Warn().
				Err(lastErr).
				Str("webhook_id", j.webhook.ID).
				Msg("Webhook delivery failed with non-retryable error")
			errMsg := lastErr.Error()
			if err := j.dispatcher.repo.UpdateLastTriggered(ctx, j.webhook.ID, &errMsg); err != nil {
				log.Warn().Err(err).Str("webhook_id", j.webhook.ID).Msg("Failed to update last triggered")
			}
			return lastErr
		}

		log.Warn().
			Err(lastErr).
			Str("webhook_id", j.webhook.ID).
			Int("attempt", attempt).
			Int("max_retries", j.dispatcher.config.MaxRetries).
			Msg("Webhook delivery attempt failed, will retry")

		if attempt < j.dispatcher.config.MaxRetries {
			delay := j.dispatcher.config.RetryDelay * time.Duration(attempt*attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	errMsg := fmt.Sprintf("delivery failed after %d attempts: %v", j.dispatcher.config.MaxRetries, lastErr)
	if err := j.dispatcher.repo.UpdateLastTriggered(ctx, j.webhook.ID, &errMsg); err != nil {
		log.Warn().Err(err).Str("webhook_id", j.webhook.ID).Msg("Failed to update last triggered")
	}
	return errors.New(errMsg)
}

func (j *deliveryJob) doHTTPPost(ctx context.Context, contentType string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, j.webhook.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "Marmot-Webhook/1.0")

	resp, err := j.dispatcher.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		return fmt.Errorf("retryable error: HTTP %d", resp.StatusCode)
	}

	return &nonRetryableError{err: fmt.Errorf("HTTP %d: request rejected by endpoint", resp.StatusCode)}
}
