package accessexpiry

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marmotdata/marmot/internal/core/notification"
)

const (
	KindAPIKey            = "api_key"
	KindServiceAccountKey = "service_account_key"
)

// Holders lists who answers for something when its owner cannot be reached.
type Holders func(ctx context.Context) ([]notification.Recipient, error)

// RoleHolders lists the active users whose roles carry the permission, the same test HasPermission makes.
func RoleHolders(db *pgxpool.Pool, resourceType, action string) Holders {
	return func(ctx context.Context) ([]notification.Recipient, error) {
		rows, err := db.Query(ctx, `
			SELECT DISTINCT u.id
			  FROM users u
			  JOIN user_roles ur ON ur.user_id = u.id
			  JOIN roles r ON r.id = ur.role_id AND r.deleted_at IS NULL
			  JOIN role_permissions rp ON rp.role_id = r.id
			  JOIN permissions p ON p.id = rp.permission_id
			 WHERE u.active
			   AND p.resource_type = $1
			   AND p.action = $2`, resourceType, action)
		if err != nil {
			return nil, fmt.Errorf("listing %s:%s holders: %w", resourceType, action, err)
		}
		defer rows.Close()
		var out []notification.Recipient
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			out = append(out, notification.Recipient{Type: notification.RecipientTypeUser, ID: id})
		}
		return out, rows.Err()
	}
}

// KeySource announces personal and service account API keys that are about to expire or have expired. A personal key's notice goes to its owner; a service account key's goes to whoever created the account, or to the managers when that person has gone.
type KeySource struct {
	db       *pgxpool.Pool
	managers Holders
}

// NewKeySource returns a source over both kinds of API key.
func NewKeySource(db *pgxpool.Pool, managers Holders) *KeySource {
	return &KeySource{db: db, managers: managers}
}

// keyRow is one API key due a notice; AccountID is empty for a personal key.
type keyRow struct {
	ID          string
	Name        string
	ExpiresAt   time.Time
	CreatedAt   time.Time
	OwnerID     string
	AccountID   string
	AccountName string
}

// Notices implements Source.
func (s *KeySource) Notices(ctx context.Context, now time.Time) ([]Notice, error) {
	rows, err := s.db.Query(ctx, `
		SELECT k.id, k.name, k.expires_at, k.created_at, k.user_id::text, '', ''
		  FROM api_keys k
		  JOIN users u ON u.id = k.user_id
		 WHERE u.active
		   AND k.expires_at IS NOT NULL AND k.expires_at <= $1 AND k.expires_at > $2
		UNION ALL
		SELECT k.id, k.name, k.expires_at, k.created_at, COALESCE(owner.id::text, ''), sa.id::text, sa.name
		  FROM service_account_api_keys k
		  JOIN service_accounts sa ON sa.id = k.service_account_id
		  LEFT JOIN users owner ON owner.id = sa.created_by AND owner.active
		 WHERE sa.deleted_at IS NULL AND sa.active
		   AND k.expires_at IS NOT NULL AND k.expires_at <= $1 AND k.expires_at > $2`,
		now.Add(Window), now.Add(-Lookback))
	if err != nil {
		return nil, fmt.Errorf("listing expiring API keys: %w", err)
	}
	defer rows.Close()

	var keys []keyRow
	for rows.Next() {
		var k keyRow
		if err := rows.Scan(&k.ID, &k.Name, &k.ExpiresAt, &k.CreatedAt, &k.OwnerID, &k.AccountID, &k.AccountName); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var managers []notification.Recipient
	managersLoaded := false
	var notices []Notice
	for _, k := range keys {
		n, ok := keyNotice(k, now)
		if !ok {
			continue
		}
		if k.OwnerID != "" {
			n.Input.Recipients = []notification.Recipient{{Type: notification.RecipientTypeUser, ID: k.OwnerID}}
		} else if k.AccountID != "" && s.managers != nil {
			if !managersLoaded {
				if managers, err = s.managers(ctx); err != nil {
					return nil, err
				}
				managersLoaded = true
			}
			n.Input.Recipients = managers
		}
		notices = append(notices, n)
	}
	return notices, nil
}

// keyNotice builds the notice for one key, without recipients. A key made to last no longer than the warning window gets no warning, since whoever made it chose that, but it is still announced once it has expired.
func keyNotice(k keyRow, now time.Time) (Notice, bool) {
	state := State(k.ExpiresAt, now)
	if state == StateExpiring && k.ExpiresAt.Sub(k.CreatedAt) <= Window {
		return Notice{}, false
	}

	data := map[string]interface{}{
		"state":      state,
		"key_id":     k.ID,
		"key_name":   k.Name,
		"expires_at": k.ExpiresAt.UTC().Format(time.RFC3339),
	}
	var kind, title, message, prefix string
	if k.AccountID == "" {
		kind, prefix = KindAPIKey, "api-key"
		data["link"] = "/account/api-keys"
		if state == StateExpiring {
			title = "API key expiring"
			message = fmt.Sprintf("Your API key \"%s\" expires on %s.", k.Name, FormatDate(k.ExpiresAt))
		} else {
			title = "API key expired"
			message = fmt.Sprintf("Your API key \"%s\" has expired, so anything using it can no longer sign in.", k.Name)
		}
	} else {
		kind, prefix = KindServiceAccountKey, "service-account-key"
		data["account_id"] = k.AccountID
		data["account_name"] = k.AccountName
		data["link"] = "/settings/service-accounts/" + k.AccountID
		if state == StateExpiring {
			title = "Service account key expiring"
			message = fmt.Sprintf("Key \"%s\" on service account \"%s\" expires on %s.", k.Name, k.AccountName, FormatDate(k.ExpiresAt))
		} else {
			title = "Service account key expired"
			message = fmt.Sprintf("Key \"%s\" on service account \"%s\" has expired, so anything using it can no longer sign in.", k.Name, k.AccountName)
		}
	}
	data["kind"] = kind

	return Notice{
		Key: fmt.Sprintf("%s:%s:%d:%s", prefix, k.ID, k.ExpiresAt.Unix(), state),
		Input: notification.CreateNotificationInput{
			Type:    notification.TypeAccessExpiry,
			Title:   title,
			Message: message,
			Data:    data,
		},
	}, true
}
