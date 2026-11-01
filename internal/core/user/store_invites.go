package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) SaveInvite(ctx context.Context, userID, email, tokenHash string, createdBy *string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_invites (user_id, token_hash, email, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		   SET token_hash = EXCLUDED.token_hash, created_by = EXCLUDED.created_by,
		       expires_at = EXCLUDED.expires_at, created_at = NOW()`,
		userID, tokenHash, email, createdBy, expiresAt)
	if err != nil {
		return fmt.Errorf("saving invitation: %w", err)
	}
	return nil
}

func scanInvite(row pgx.Row) (*Invite, error) {
	var inv Invite
	if err := row.Scan(&inv.UserID, &inv.Email, &inv.Name, &inv.ExpiresAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInviteNotFound
		}
		return nil, fmt.Errorf("reading invitation: %w", err)
	}
	return &inv, nil
}

const inviteColumns = `i.user_id, i.email, u.name, i.expires_at FROM user_invites i JOIN users u ON u.id = i.user_id`

func (r *PostgresRepository) GetInvite(ctx context.Context, userID string) (*Invite, error) {
	inv, err := scanInvite(r.db.QueryRow(ctx, `SELECT `+inviteColumns+` WHERE i.user_id = $1`, userID))
	if errors.Is(err, ErrInviteNotFound) {
		return nil, ErrNotInvited
	}
	return inv, err
}

func (r *PostgresRepository) GetInviteByTokenHash(ctx context.Context, tokenHash string) (*Invite, error) {
	return scanInvite(r.db.QueryRow(ctx, `SELECT `+inviteColumns+` WHERE i.token_hash = $1`, tokenHash))
}

// FindInviteByEmail ignores expiry on purpose: an administrator who invited an address has vouched for it, and single sign-on proves the person holds it, so a late arrival through their identity provider is still welcome.
func (r *PostgresRepository) FindInviteByEmail(ctx context.Context, email string) (*Invite, error) {
	return scanInvite(r.db.QueryRow(ctx, `SELECT `+inviteColumns+` WHERE lower(i.email) = lower($1) LIMIT 1`, email))
}

// AcceptInvite activates the invited member and spends the invitation in one transaction, so a link can never be used twice. An empty password hash leaves the password unset, for members who arrive through single sign-on.
func (r *PostgresRepository) AcceptInvite(ctx context.Context, userID, passwordHash, name string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `DELETE FROM user_invites WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("spending invitation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInviteNotFound
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users
		   SET active = true,
		       password_hash = COALESCE(NULLIF($2, ''), password_hash),
		       name = COALESCE(NULLIF($3, ''), name),
		       updated_at = NOW()
		 WHERE id = $1`,
		userID, passwordHash, name); err != nil {
		return fmt.Errorf("activating invited user: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) TouchLastSeen(ctx context.Context, userID string, minInterval time.Duration) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET last_seen_at = NOW()
		 WHERE id = $1 AND (last_seen_at IS NULL OR last_seen_at < NOW() - make_interval(secs => $2))`,
		userID, minInterval.Seconds())
	if err != nil {
		return fmt.Errorf("recording last seen: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetMemberStates(ctx context.Context, ids []string) (map[string]MemberState, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.last_seen_at, i.expires_at
		  FROM users u
		  LEFT JOIN user_invites i ON i.user_id = u.id
		 WHERE u.id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("querying member states: %w", err)
	}
	defer rows.Close()

	out := make(map[string]MemberState, len(ids))
	for rows.Next() {
		var id string
		var st MemberState
		if err := rows.Scan(&id, &st.LastSeenAt, &st.InviteExpiresAt); err != nil {
			return nil, fmt.Errorf("scanning member state: %w", err)
		}
		out[id] = st
	}
	return out, rows.Err()
}

func (r *PostgresRepository) CountIdentitiesByProvider(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.Query(ctx, `SELECT provider, COUNT(DISTINCT user_id) FROM user_identities GROUP BY provider`)
	if err != nil {
		return nil, fmt.Errorf("counting identities: %w", err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var provider string
		var n int
		if err := rows.Scan(&provider, &n); err != nil {
			return nil, fmt.Errorf("scanning identity count: %w", err)
		}
		out[provider] = n
	}
	return out, rows.Err()
}
