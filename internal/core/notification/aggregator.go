package notification

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// assetChangeEvent represents a pending asset change notification.
type assetChangeEvent struct {
	assetID    string
	assetMRN   string
	assetName  string
	changeType string
	owners     []Recipient
	queuedAt   time.Time
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

func (a *assetChangeAggregator) queue(assetID, assetMRN, assetName, changeType string, owners []Recipient) {
	a.mu.Lock()
	defer a.mu.Unlock()

	select {
	case <-a.ctx.Done():
		return
	default:
	}

	now := time.Now()

	// Key includes change type to keep schema and asset changes separate
	eventKey := assetID + ":" + changeType
	if _, exists := a.events[eventKey]; !exists {
		a.events[eventKey] = &assetChangeEvent{
			assetID:    assetID,
			assetMRN:   assetMRN,
			assetName:  assetName,
			changeType: changeType,
			owners:     owners,
			queuedAt:   now,
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

	// Group by owner and change type
	ownerTypeAssets := make(map[string][]assetChangeEvent)
	for _, event := range events {
		for _, owner := range event.owners {
			key := fmt.Sprintf("%s:%s:%s", owner.Type, owner.ID, event.changeType)
			ownerTypeAssets[key] = append(ownerTypeAssets[key], *event)
		}
	}

	for ownerKey, assets := range ownerTypeAssets {
		a.sendAggregatedNotification(ownerKey, assets)
	}
}

func (a *assetChangeAggregator) sendAggregatedNotification(ownerKey string, assets []assetChangeEvent) {
	if len(assets) == 0 {
		return
	}

	// Parse ownerKey: "type:id:changeType"
	parts := strings.SplitN(ownerKey, ":", 3)
	if len(parts) < 3 {
		log.Warn().Str("owner_key", ownerKey).Msg("Invalid owner key format")
		return
	}
	ownerType, ownerID, changeType := parts[0], parts[1], parts[2]

	var title, message string
	data := make(map[string]interface{})

	isSchemaRelated := changeType == TypeSchemaChange || changeType == TypeUpstreamSchemaChange || changeType == TypeDownstreamSchemaChange
	if len(assets) == 1 {
		asset := assets[0]
		switch changeType {
		case TypeSchemaChange:
			title = "Schema Updated"
			message = fmt.Sprintf("Schema for \"%s\" has been modified.", asset.assetName)
		case TypeUpstreamSchemaChange:
			title = "Upstream Schema Changed"
			message = fmt.Sprintf("An upstream asset \"%s\" has had its schema modified.", asset.assetName)
		case TypeDownstreamSchemaChange:
			title = "Downstream Schema Changed"
			message = fmt.Sprintf("A downstream asset \"%s\" has had its schema modified.", asset.assetName)
		case TypeLineageChange:
			title = "Lineage Changed"
			message = fmt.Sprintf("Lineage connections for \"%s\" have been modified.", asset.assetName)
		default:
			title = "Asset Updated"
			message = fmt.Sprintf("Asset \"%s\" has been modified.", asset.assetName)
		}
		data["asset_mrn"] = asset.assetMRN
		link := fmt.Sprintf("/discover/%s", strings.TrimPrefix(asset.assetMRN, "mrn://"))
		if isSchemaRelated {
			link += "?tab=schema"
		} else if changeType == TypeLineageChange {
			link += "?tab=lineage"
		}
		data["link"] = link
	} else {
		switch changeType {
		case TypeSchemaChange:
			title = "Schemas Updated"
			message = fmt.Sprintf("Schemas for %d assets you own have been modified.", len(assets))
		case TypeUpstreamSchemaChange:
			title = "Upstream Schemas Changed"
			message = fmt.Sprintf("Schemas for %d upstream assets have been modified.", len(assets))
		case TypeDownstreamSchemaChange:
			title = "Downstream Schemas Changed"
			message = fmt.Sprintf("Schemas for %d downstream assets have been modified.", len(assets))
		case TypeLineageChange:
			title = "Lineage Changed"
			message = fmt.Sprintf("Lineage connections for %d assets have been modified.", len(assets))
		default:
			title = "Assets Updated"
			message = fmt.Sprintf("%d assets you own have been modified.", len(assets))
		}

		mrns := make([]string, 0, len(assets))
		for _, asset := range assets {
			mrns = append(mrns, asset.assetMRN)
		}
		data["asset_mrns"] = mrns
		data["count"] = len(assets)
	}

	input := CreateNotificationInput{
		Recipients: []Recipient{{Type: ownerType, ID: ownerID}},
		Type:       changeType,
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
