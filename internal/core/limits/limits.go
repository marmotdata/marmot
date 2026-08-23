package limits

import (
	"context"
	"errors"
	"fmt"
)

// Resource identifies a countable kind that a Guard gates on create.
type Resource string

const (
	ResourceUsers           Resource = "users"
	ResourceAssets          Resource = "assets"
	ResourceGlossaryTerms   Resource = "glossary_terms"
	ResourceServiceAccounts Resource = "service_accounts"
)

// Guard is consulted before a service persists a new resource. It must
// return an error that wraps *ErrLimitExceeded when the create would push
// the count past the configured cap.
type Guard interface {
	CheckCreate(ctx context.Context, resource Resource) error
}

// NoopGuard is the OSS default and never blocks a create.
type NoopGuard struct{}

// CheckCreate always returns nil.
func (NoopGuard) CheckCreate(_ context.Context, _ Resource) error { return nil }

// ErrLimitExceeded is returned by a Guard when a create is refused because
// the resource is at its cap. Handlers surface Current and Limit to callers
// so they see why the write was denied and, in enterprise builds, where to
// go to raise it.
type ErrLimitExceeded struct {
	Resource Resource
	Current  int64
	Limit    int64
	// Message is an optional operator- or plan-specific message (e.g. a
	// call to action to raise the limit). When empty a default is used.
	Message string
}

func (e *ErrLimitExceeded) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("You have reached the limit of %d %s for this Marmot instance.", e.Limit, e.Resource)
}

// AsLimitExceeded returns the wrapped *ErrLimitExceeded and true if err
// (or any error it wraps) is one; otherwise nil, false.
func AsLimitExceeded(err error) (*ErrLimitExceeded, bool) {
	var target *ErrLimitExceeded
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
