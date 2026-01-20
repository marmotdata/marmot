package notification

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// assetChangeEvent represents a pending asset change notification.
type assetChangeEvent struct {
	assetID   string
	assetMRN  string
	assetName string
	owners    []Recipient
	queuedAt  time.Time
}

// assetChangeAggregator batches asset change notifications to reduce spam.
type assetChangeAggregator struct {
	svc      *Service
	window   time.Duration
	maxWait  time.Duration
	events   map[string]*assetChangeEvent
	mu       sync.Mutex
	timer    *time.Timer
	firstAt  time.Time
	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
}

func newAssetChangeAggregator(svc *Service, window, maxWait time.Duration) *assetChangeAggregator {
	return &assetChangeAggregator{
		svc:     svc,
		window:  window,
		maxWait: maxWait,
		events:  make(map[string]*assetChangeEvent),
	}
}

func (a *assetChangeAggregator) start(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)
}

func (a *assetChangeAggregator) stop() {
	a.stopOnce.Do(func() {
		if a.cancel != nil {
			a.cancel()
		}

		a.mu.Lock()
		if a.timer != nil {
			a.timer.Stop()
		}
		a.mu.Unlock()

		a.flush()
	})
}

func (a *assetChangeAggregator) queue(assetID, assetMRN, assetName string, owners []Recipient) {
	a.mu.Lock()
	defer a.mu.Unlock()

	select {
	case <-a.ctx.Done():
		return
	default:
	}

	now := time.Now()

	if _, exists := a.events[assetID]; !exists {
		a.events[assetID] = &assetChangeEvent{
			assetID:   assetID,
			assetMRN:  assetMRN,
			assetName: assetName,
			owners:    owners,
			queuedAt:  now,
		}
	}

	if a.firstAt.IsZero() {
		a.firstAt = now
	}

	if a.timer != nil {
		a.timer.Stop()
	}

	timeUntilMaxWait := a.maxWait - time.Since(a.firstAt)
	delay := a.window
	if timeUntilMaxWait < delay {
		delay = timeUntilMaxWait
	}
	if delay <= 0 {
		delay = time.Millisecond
	}

	a.timer = time.AfterFunc(delay, a.flush)
}

func (a *assetChangeAggregator) flush() {
	a.mu.Lock()
	events := a.events
	a.events = make(map[string]*assetChangeEvent)
	a.firstAt = time.Time{}
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}
	a.mu.Unlock()

	if len(events) == 0 {
		return
	}

	ownerAssets := make(map[string][]assetChangeEvent)
	for _, event := range events {
		for _, owner := range event.owners {
			key := fmt.Sprintf("%s:%s", owner.Type, owner.ID)
			ownerAssets[key] = append(ownerAssets[key], *event)
		}
	}

	for ownerKey, assets := range ownerAssets {
		a.sendAggregatedNotification(ownerKey, assets)
	}
}

func (a *assetChangeAggregator) sendAggregatedNotification(ownerKey string, assets []assetChangeEvent) {
	if len(assets) == 0 {
		return
	}

	var ownerType, ownerID string
	fmt.Sscanf(ownerKey, "%s:%s", &ownerType, &ownerID)

	for i, c := range ownerKey {
		if c == ':' {
			ownerType = ownerKey[:i]
			ownerID = ownerKey[i+1:]
			break
		}
	}

	var title, message string
	data := make(map[string]interface{})

	if len(assets) == 1 {
		asset := assets[0]
		title = "Asset Updated"
		message = fmt.Sprintf("Asset \"%s\" has been modified.", asset.assetName)
		data["asset_mrn"] = asset.assetMRN
		data["link"] = fmt.Sprintf("/discover/%s", asset.assetMRN)
	} else {
		title = "Assets Updated"
		message = fmt.Sprintf("%d assets you own have been modified.", len(assets))

		mrns := make([]string, 0, len(assets))
		for _, asset := range assets {
			mrns = append(mrns, asset.assetMRN)
		}
		data["asset_mrns"] = mrns
		data["count"] = len(assets)
	}

	input := CreateNotificationInput{
		Recipients: []Recipient{{Type: ownerType, ID: ownerID}},
		Type:       TypeAssetChange,
		Title:      title,
		Message:    message,
		Data:       data,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := a.svc.Create(ctx, input); err != nil {
		log.Warn().
			Err(err).
			Str("owner_type", ownerType).
			Str("owner_id", ownerID).
			Int("asset_count", len(assets)).
			Msg("Failed to send aggregated asset change notification")
	}
}
