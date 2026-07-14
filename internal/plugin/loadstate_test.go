package plugin

import (
	"testing"
	"time"
)

func TestLoadState_InitiallyNotReady(t *testing.T) {
	ls := &LoadState{done: make(chan struct{})}
	if ls.Ready() {
		t.Fatal("expected new LoadState to not be ready")
	}
	select {
	case <-ls.Done():
		t.Fatal("expected Done channel to be open")
	default:
	}
}

func TestLoadState_MarkReadyFlipsState(t *testing.T) {
	ls := &LoadState{done: make(chan struct{})}
	ls.MarkReady()
	if !ls.Ready() {
		t.Fatal("expected Ready true after MarkReady")
	}
	select {
	case <-ls.Done():
	case <-time.After(time.Second):
		t.Fatal("Done channel should be closed")
	}
}

func TestLoadState_MarkReadyIdempotent(t *testing.T) {
	ls := &LoadState{done: make(chan struct{})}
	ls.MarkReady()
	ls.MarkReady() // must not panic on double-close
	if !ls.Ready() {
		t.Fatal("expected still ready after second MarkReady")
	}
}

func TestGetLoadState_ReturnsSingleton(t *testing.T) {
	if GetLoadState() != GetLoadState() {
		t.Fatal("GetLoadState should return the same instance")
	}
}
