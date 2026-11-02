package dataquery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/plugin"
	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// fakeLoadState stands in for startup plugin loading, which the real process finishes once by closing done; like plugin.LoadState it is ready once done is closed.
type fakeLoadState struct {
	done chan struct{}
}

func (f *fakeLoadState) Ready() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}
func (f *fakeLoadState) Done() <-chan struct{} { return f.done }

func shortPluginLoadWait(t *testing.T, d time.Duration) {
	t.Helper()
	prev := pluginLoadWait
	pluginLoadWait = d
	t.Cleanup(func() { pluginLoadWait = prev })
}

func TestLoadedPluginReturnsARegisteredPluginWhileStillLoading(t *testing.T) {
	s := &service{loadState: &fakeLoadState{done: make(chan struct{})}}
	entry, err := s.loadedPlugin(context.Background(), testPluginID)
	if err != nil || entry == nil {
		t.Fatalf("expected the registered plugin, got %v, %v", entry, err)
	}
}

func TestLoadedPluginWaitsForLoadingToFinish(t *testing.T) {
	state := &fakeLoadState{done: make(chan struct{})}
	s := &service{loadState: state}
	const id = "query-test-late"
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = plugin.GetRegistry().Register(pluginsdk.Meta{ID: id, Name: "Late", SupportsQuery: true}, nopSource{})
		close(state.done)
	}()
	entry, err := s.loadedPlugin(context.Background(), id)
	if err != nil || entry == nil {
		t.Fatalf("expected the plugin once loading finished, got %v, %v", entry, err)
	}
}

func TestLoadedPluginReportsLoadingWhenItTakesTooLong(t *testing.T) {
	shortPluginLoadWait(t, 20*time.Millisecond)
	s := &service{loadState: &fakeLoadState{done: make(chan struct{})}}
	_, err := s.loadedPlugin(context.Background(), "query-test-never")
	if !errors.Is(err, ErrPluginsLoading) {
		t.Fatalf("expected ErrPluginsLoading, got %v", err)
	}
}

func TestLoadedPluginReportsAMissingPluginOnceLoadingHasFinished(t *testing.T) {
	done := make(chan struct{})
	close(done)
	s := &service{loadState: &fakeLoadState{done: done}}
	_, err := s.loadedPlugin(context.Background(), "query-test-missing")
	if err == nil || errors.Is(err, ErrPluginsLoading) {
		t.Fatalf("expected a not-found error, got %v", err)
	}
}

func TestLoadedPluginStopsWaitingWhenTheCallerGivesUp(t *testing.T) {
	s := &service{loadState: &fakeLoadState{done: make(chan struct{})}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.loadedPlugin(ctx, "query-test-never")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
