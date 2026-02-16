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
	assetID       string
	assetMRN      string
	assetName     string
	changeType    string
	changedFields []string
	owners        []Recipient
	queuedAt      time.Time
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

func (a *assetChangeAggregator) queue(assetID, assetMRN, assetName, changeType string, owners []Recipient, changedFields []string) {
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
	if existing, exists := a.events[eventKey]; exists {
		// Merge changed fields for multiple updates to same asset within window
		existing.changedFields = mergeUniqueFields(existing.changedFields, changedFields)
		existing.queuedAt = now
	} else {
		a.events[eventKey] = &assetChangeEvent{
			assetID:       assetID,
			assetMRN:      assetMRN,
			assetName:     assetName,
			changeType:    changeType,
			changedFields: changedFields,
			owners:        owners,
			queuedAt:      now,
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

	const batchThreshold = 20

	type recipientKey struct {
		recipientType string
		recipientID   string
	}

	grouped := make(map[recipientKey][]*assetChangeEvent)

	for _, event := range events {
		for _, owner := range event.owners {
			key := recipientKey{
				recipientType: owner.Type,
				recipientID:   owner.ID,
			}
			grouped[key] = append(grouped[key], event)
		}
	}

	// Process each group based on threshold
	for key, eventList := range grouped {
		owner := Recipient{Type: key.recipientType, ID: key.recipientID}

		// Separate deletions, lineage updates, and other updates
		var deletions, lineageUpdates, otherUpdates []*assetChangeEvent
		for _, event := range eventList {
			switch event.changeType {
			case TypeAssetDeleted:
				deletions = append(deletions, event)
			case TypeLineageChange:
				lineageUpdates = append(lineageUpdates, event)
			default:
				otherUpdates = append(otherUpdates, event)
			}
		}

		// Apply batching threshold logic
		if len(deletions) > batchThreshold {
			a.sendBatchedNotification(owner, deletions, TypeAssetDeleted)
		} else {
			for _, event := range deletions {
				a.sendAssetNotification(owner, *event)
			}
		}

		if len(lineageUpdates) > batchThreshold {
			a.sendBatchedNotification(owner, lineageUpdates, TypeLineageChange)
		} else {
			for _, event := range lineageUpdates {
				a.sendAssetNotification(owner, *event)
			}
		}

		if len(otherUpdates) > batchThreshold {
			a.sendBatchedNotification(owner, otherUpdates, TypeAssetChange)
		} else {
			for _, event := range otherUpdates {
				a.sendAssetNotification(owner, *event)
			}
		}
	}
}

func (a *assetChangeAggregator) sendAssetNotification(owner Recipient, asset assetChangeEvent) {
	var title, message string
	data := make(map[string]interface{})

	isSchemaRelated := asset.changeType == TypeSchemaChange || asset.changeType == TypeUpstreamSchemaChange || asset.changeType == TypeDownstreamSchemaChange

	// Generate notification based on change type
	switch asset.changeType {
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
	case TypeAssetDeleted:
		title = "Asset Deleted"
		message = fmt.Sprintf("Asset \"%s\" has been deleted.", asset.assetName)
	default:
		// Asset update with field details
		title = "Asset Updated"
		fieldList := strings.Join(asset.changedFields, ", ")
		message = fmt.Sprintf("%s updated for \"%s\".", fieldList, asset.assetName)
	}

	data["asset_mrn"] = asset.assetMRN
	if len(asset.changedFields) > 0 {
		data["changed_fields"] = asset.changedFields
	}
	if asset.changeType != TypeAssetDeleted {
		link := fmt.Sprintf("/discover/%s", strings.TrimPrefix(asset.assetMRN, "mrn://"))
		if isSchemaRelated {
			link += "?tab=schema"
		} else if asset.changeType == TypeLineageChange {
			link += "?tab=lineage"
		}
		data["link"] = link
	}

	input := CreateNotificationInput{
		Recipients: []Recipient{owner},
		Type:       asset.changeType,
		Title:      title,
		Message:    message,
		Data:       data,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := a.svc.Create(ctx, input); err != nil {
		log.Warn().
			Err(err).
			Str("owner_type", owner.Type).
			Str("owner_id", owner.ID).
			Str("asset_id", asset.assetID).
			Msg("Failed to send asset change notification")
	}
}

func (a *assetChangeAggregator) sendBatchedNotification(owner Recipient, events []*assetChangeEvent, batchType string) {
	eventCount := len(events)

	// Count unique assets
	uniqueAssets := make(map[string]bool)
	for _, event := range events {
		uniqueAssets[event.assetID] = true
	}
	assetCount := len(uniqueAssets)

	var title, message string

	switch batchType {
	case TypeAssetDeleted:
		title = "Assets Deleted"
		message = fmt.Sprintf("%d assets have been deleted.", assetCount)
	case TypeLineageChange:
		title = "Lineage Updates"
		message = fmt.Sprintf("%d assets have lineage changes.", assetCount)
	default:
		title = "Asset Updates"
		message = fmt.Sprintf("%d changes made in %d assets.", eventCount, assetCount)
	}

	data := map[string]interface{}{
		"count":       eventCount,
		"asset_count": assetCount,
	}

	input := CreateNotificationInput{
		Recipients: []Recipient{owner},
		Type:       batchType,
		Title:      title,
		Message:    message,
		Data:       data,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := a.svc.Create(ctx, input); err != nil {
		log.Warn().
			Err(err).
			Str("owner_type", owner.Type).
			Str("owner_id", owner.ID).
			Int("event_count", eventCount).
			Int("asset_count", assetCount).
			Msg("Failed to send batched notification")
	}
}

// mergeUniqueFields merges two field slices, removing duplicates.
func mergeUniqueFields(a, b []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(a)+len(b))

	for _, field := range append(a, b...) {
		if !seen[field] {
			seen[field] = true
			result = append(result, field)
		}
	}
	return result
}
