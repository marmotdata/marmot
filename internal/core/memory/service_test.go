package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestContentIsOneShortLine(t *testing.T) {
	got, err := validateContent("  grain is\n\none row   per order line \t")
	if err != nil || got != "grain is one row per order line" {
		t.Errorf("whitespace should collapse to one line: %q, %v", got, err)
	}
	if _, err := validateContent(" \n\t "); !errors.Is(err, ErrInvalid) {
		t.Errorf("blank content: %v", err)
	}
	if _, err := validateContent(strings.Repeat("é", MaxContentLength)); err != nil {
		t.Errorf("content at the limit, counted in characters: %v", err)
	}
	if _, err := validateContent(strings.Repeat("x", MaxContentLength+1)); !errors.Is(err, ErrInvalid) {
		t.Errorf("content over the limit: %v", err)
	}
}

type recordingObserver struct{ changed []string }

func (o *recordingObserver) OnEntityChanged(_ context.Context, entityType, entityID string) {
	o.changed = append(o.changed, entityType+":"+entityID)
}

// failingService fails every write after the first.
type failingService struct {
	Service
	calls int
}

func (f *failingService) Remember(context.Context, Entity, RememberInput) (*Memory, error) {
	f.calls++
	if f.calls > 1 {
		return nil, ErrInvalid
	}
	return &Memory{}, nil
}

func (f *failingService) Update(context.Context, Entity, string, UpdateInput) (*Memory, error) {
	return &Memory{}, nil
}

func (f *failingService) Forget(context.Context, Entity, string) error { return ErrNotFound }

func TestNotifySearchReportsSuccessfulChanges(t *testing.T) {
	obs := &recordingObserver{}
	svc := NotifySearch(&failingService{}, obs)
	e := Entity{Type: EntityAsset, ID: "a1"}
	ctx := context.Background()

	_, _ = svc.Remember(ctx, e, RememberInput{})
	_, _ = svc.Remember(ctx, e, RememberInput{})
	_, _ = svc.Update(ctx, e, "m1", UpdateInput{})
	_ = svc.Forget(ctx, e, "m1")

	if len(obs.changed) != 2 || obs.changed[0] != "asset:a1" {
		t.Errorf("want one notice per successful change, got %v", obs.changed)
	}
}
