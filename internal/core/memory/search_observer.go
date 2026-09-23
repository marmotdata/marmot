package memory

import "context"

// SearchObserver keeps an external search index in step with an entity's
// memory.
type SearchObserver interface {
	OnEntityChanged(ctx context.Context, entityType, entityID string)
}

// NotifySearch wraps a Service so that every change to an entity's memory
// tells the observer which entity changed.
func NotifySearch(svc Service, observer SearchObserver) Service {
	return notifyingService{Service: svc, observer: observer}
}

type notifyingService struct {
	Service
	observer SearchObserver
}

func (s notifyingService) changed(ctx context.Context, e Entity) {
	s.observer.OnEntityChanged(ctx, string(e.Type), e.ID)
}

func (s notifyingService) Remember(ctx context.Context, e Entity, in RememberInput) (*Memory, error) {
	m, err := s.Service.Remember(ctx, e, in)
	if err == nil {
		s.changed(ctx, e)
	}
	return m, err
}

func (s notifyingService) Update(ctx context.Context, e Entity, id string, in UpdateInput) (*Memory, error) {
	m, err := s.Service.Update(ctx, e, id, in)
	if err == nil {
		s.changed(ctx, e)
	}
	return m, err
}

func (s notifyingService) Forget(ctx context.Context, e Entity, id string) error {
	err := s.Service.Forget(ctx, e, id)
	if err == nil {
		s.changed(ctx, e)
	}
	return err
}
