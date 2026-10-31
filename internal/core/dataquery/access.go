package dataquery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/notification"
)

// ErrNotRunning is returned when cancelling a query that has already finished, or that runs on another replica, which holds its own cancel handle.
var ErrNotRunning = errors.New("query is not running here")

// Refusal is the structured account of a denied query, so a client can say what refused it, what would allow it and who the caller is acting as without parsing the message.
type Refusal struct {
	QueryID string `json:"query_id,omitempty"`
	Reason  string `json:"reason"`
	// Rule is the outcome reason: permission when the caller lacks a permission, policy when a policy refused the statement.
	Rule       string `json:"rule"`
	Permission string `json:"permission,omitempty"`
	Policy     string `json:"policy,omitempty"`
	// Needed says what would allow the query, such as "assets:query on warehouse".
	Needed    string    `json:"needed,omitempty"`
	Resources []string  `json:"resources,omitempty"`
	Roles     []string  `json:"roles"`
	Source    string    `json:"source"`
	DecidedAt time.Time `json:"decided_at"`
} // @name QueryRefusal

// DeniedError carries a refusal. It matches ErrDenied, and its message is the one callers have always seen.
type DeniedError struct {
	Refusal Refusal
}

func (e *DeniedError) Error() string {
	return fmt.Sprintf("%s: %s", ErrDenied, e.Refusal.Reason)
}

func (e *DeniedError) Unwrap() error { return ErrDenied }

// AccessRequestInput asks the people who can grant access for what a refused query needed. QueryID names the refused query; without it Source says what access is wanted.
type AccessRequestInput struct {
	QueryID string `json:"query_id,omitempty"`
	Source  string `json:"source,omitempty"`
	Message string `json:"message,omitempty" validate:"max=2000"`
} // @name QueryAccessRequestInput

// AccessRequest is a recorded request for access.
type AccessRequest struct {
	ID            string    `json:"id"`
	PrincipalType string    `json:"principal_type"`
	PrincipalID   string    `json:"principal_id"`
	AuditSubject  string    `json:"audit_subject"`
	QueryID       *string   `json:"query_id,omitempty"`
	SourceName    string    `json:"source_name"`
	Permission    string    `json:"permission,omitempty"`
	Needed        string    `json:"needed,omitempty"`
	Message       string    `json:"message,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	// Notified counts the people told about this request; Repeat is set when an identical request made within the last day was returned instead, and nobody was told again.
	Notified int  `json:"notified"`
	Repeat   bool `json:"repeat"`
} // @name QueryAccessRequest

// Notifier delivers in-app notifications. It is satisfied by the notification service.
type Notifier interface {
	CreateSync(ctx context.Context, input notification.CreateNotificationInput) (int, error)
}

// Grantors lists who can give a principal of the given type the access it asked for. OSS asks the holders of users:manage or service_accounts:manage; the cloud distribution answers from its IAM.
type Grantors func(ctx context.Context, principalType string) ([]notification.Recipient, error)

// accessRequestRepeatWindow is how long an identical request is answered with the earlier one, so pressing the button twice does not notify everyone twice.
const accessRequestRepeatWindow = 24 * time.Hour

// RequestAccess records a request for the access a refused query needed and notifies whoever can grant it. A request names the caller's own refused query, or a source when there is none.
func (s *service) RequestAccess(ctx context.Context, principal auth.Principal, input AccessRequestInput) (*AccessRequest, error) {
	if principal == nil {
		return nil, ErrDenied
	}
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	req := &AccessRequest{
		PrincipalType: string(principal.Type()),
		PrincipalID:   principal.ID(),
		AuditSubject:  principal.AuditSubject(),
		Message:       strings.TrimSpace(input.Message),
	}
	if input.QueryID != "" {
		q, err := s.repo.GetQuery(ctx, input.QueryID)
		if err != nil {
			return nil, err
		}
		// Only the caller's own refused query can be the basis of a request; anyone else's reads as missing.
		if q.PrincipalType != req.PrincipalType || q.PrincipalID != req.PrincipalID {
			return nil, ErrQueryNotFound
		}
		if q.Decision != DecisionDenied {
			return nil, fmt.Errorf("%w: only a refused query needs access", ErrInvalidInput)
		}
		req.QueryID = &q.ID
		req.SourceName = q.SourceName
		req.Permission, _ = q.DecisionDetail["permission"].(string)
		req.Needed, _ = q.DecisionDetail["needed"].(string)
	} else {
		req.SourceName = strings.TrimSpace(input.Source)
		if req.SourceName == "" {
			return nil, fmt.Errorf("%w: a query_id or a source is required", ErrInvalidInput)
		}
	}
	if req.Permission == "" {
		req.Permission = queryPermission
	}
	if req.Needed == "" {
		req.Needed = queryPermission + " on " + req.SourceName
	}

	if earlier, err := s.repo.RecentAccessRequest(ctx, req.PrincipalType, req.PrincipalID, req.SourceName, req.Needed, time.Now().Add(-accessRequestRepeatWindow)); err != nil {
		return nil, err
	} else if earlier != nil {
		earlier.Repeat = true
		return earlier, nil
	}

	if err := s.repo.CreateAccessRequest(ctx, req); err != nil {
		return nil, err
	}
	req.Notified = s.notifyGrantors(ctx, principal, req)
	return req, nil
}

// notifyGrantors tells the people who can grant access about a request. The request is already recorded, so a notification failure is logged rather than failing it.
func (s *service) notifyGrantors(ctx context.Context, principal auth.Principal, req *AccessRequest) int {
	if s.opts.Notifier == nil || s.opts.Grantors == nil {
		return 0
	}
	recipients, err := s.opts.Grantors(ctx, req.PrincipalType)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list who can grant query access")
		return 0
	}
	// Someone who could grant themselves the access gains nothing from being told they asked.
	if u := principal.AsUser(); u != nil {
		kept := recipients[:0]
		for _, r := range recipients {
			if r.ID != u.ID {
				kept = append(kept, r)
			}
		}
		recipients = kept
	}
	if len(recipients) == 0 {
		return 0
	}

	who := principal.DisplayName()
	if who == "" {
		who = req.AuditSubject
	}
	message := fmt.Sprintf("%s asked for %s.", who, req.Needed)
	if req.Message != "" {
		message += " " + req.Message
	}
	link := "/query-audit?caller=" + req.PrincipalType + ":" + req.PrincipalID
	if req.QueryID != nil {
		link = "/query-audit?id=" + *req.QueryID
	}
	data := map[string]interface{}{
		"request_id":     req.ID,
		"requester":      who,
		"principal_type": req.PrincipalType,
		"source":         req.SourceName,
		"permission":     req.Permission,
		"needed":         req.Needed,
		"link":           link,
	}
	if req.QueryID != nil {
		data["query_id"] = *req.QueryID
	}
	n, err := s.opts.Notifier.CreateSync(context.WithoutCancel(ctx), notification.CreateNotificationInput{
		Recipients: recipients,
		Type:       notification.TypeAccessRequest,
		Title:      "Query access requested",
		Message:    message,
		Data:       data,
	})
	if err != nil {
		log.Warn().Err(err).Str("request_id", req.ID).Msg("Failed to notify about a query access request")
		return 0
	}
	return n
}

func (r *PostgresRepository) CreateAccessRequest(ctx context.Context, req *AccessRequest) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO data_query_access_requests (principal_type, principal_id, audit_subject, query_id, source_name, permission, needed, message)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''))
		RETURNING id, created_at`,
		req.PrincipalType, req.PrincipalID, req.AuditSubject, req.QueryID, req.SourceName, req.Permission, req.Needed, req.Message,
	).Scan(&req.ID, &req.CreatedAt)
	if err != nil {
		return fmt.Errorf("recording access request: %w", err)
	}
	return nil
}

func (r *PostgresRepository) RecentAccessRequest(ctx context.Context, principalType, principalID, sourceName, needed string, since time.Time) (*AccessRequest, error) {
	var req AccessRequest
	var queryID *string
	var permission, neededCol, message *string
	err := r.db.QueryRow(ctx, `
		SELECT id, principal_type, principal_id, audit_subject, query_id::text, source_name, permission, needed, message, created_at
		FROM data_query_access_requests
		WHERE principal_type = $1 AND principal_id = $2 AND source_name = $3 AND COALESCE(needed, '') = $4 AND created_at >= $5
		ORDER BY created_at DESC LIMIT 1`,
		principalType, principalID, sourceName, needed, since,
	).Scan(&req.ID, &req.PrincipalType, &req.PrincipalID, &req.AuditSubject, &queryID, &req.SourceName, &permission, &neededCol, &message, &req.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding earlier access request: %w", err)
	}
	req.QueryID = queryID
	if permission != nil {
		req.Permission = *permission
	}
	if neededCol != nil {
		req.Needed = *neededCol
	}
	if message != nil {
		req.Message = *message
	}
	return &req, nil
}
