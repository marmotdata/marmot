package plugin

import "sync"

type LoadState struct {
	done chan struct{}
	once sync.Once
}

var globalLoadState = &LoadState{done: make(chan struct{})}

func GetLoadState() *LoadState {
	return globalLoadState
}

// Done returns a channel closed when loading finishes.
func (l *LoadState) Done() <-chan struct{} {
	return l.done
}

// Ready reports whether loading has finished, without blocking.
func (l *LoadState) Ready() bool {
	select {
	case <-l.done:
		return true
	default:
		return false
	}
}

// MarkReady signals that loading is complete. Idempotent.
func (l *LoadState) MarkReady() {
	l.once.Do(func() { close(l.done) })
}
