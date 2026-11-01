package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrInvalidToken is returned for a token that does not verify: a bad signature, an unexpected algorithm or an expired token.
	ErrInvalidToken = errors.New("invalid token")
	// ErrTokenRevoked is returned for a token that verifies but was signed out before it expired.
	ErrTokenRevoked = errors.New("token signed out")
	// ErrTokenCheck is returned when whether a token was signed out could not be established, so callers refuse it with a server error rather than a 401 that would send the client round a sign-in that cannot help.
	ErrTokenCheck = errors.New("checking token sign-out")
)

// RevocationStore remembers tokens signed out before they expire. It is keyed by a hash of the token's jti rather than the token itself, so the table holds nothing that could be replayed.
type RevocationStore interface {
	Revoke(ctx context.Context, key []byte, retainUntil time.Time) error
	Revoked(ctx context.Context, key []byte) (bool, error)
}

// revokedFallback is how long a token without exp is remembered. Marmot always sets one; this only keeps such a token from being forgotten at once.
const revokedFallback = 24 * time.Hour

// WithRevocationStore makes ValidateToken refuse tokens that were signed out and lets RevokeToken record them. Without it signing out only clears the token in the client, which is what tests and tools without a database get.
func WithRevocationStore(store RevocationStore) ServiceOption {
	return func(s *service) { s.revocations = store }
}

// revocationKey identifies a token in the store by its jti, which every token Marmot issues carries, falling back to the whole token for one that has none.
func revocationKey(claims *Claims, token string) []byte {
	id := claims.ID
	if id == "" {
		id = token
	}
	sum := sha256.Sum256([]byte(id))
	return sum[:]
}

// RevokeToken stops the token validating from now on. Only a token that verifies is remembered, so nobody can fill the table with strings they made up, and it is remembered until its exp, after which the signature check refuses it anyway. Revoking a token twice is not an error.
func (s *service) RevokeToken(ctx context.Context, tokenString string) error {
	claims, err := s.parse(ctx, tokenString)
	if err != nil {
		return err
	}
	if s.revocations == nil {
		return nil
	}
	until := time.Now().Add(revokedFallback)
	if claims.ExpiresAt != nil {
		until = claims.ExpiresAt.Time
	}
	if err := s.revocations.Revoke(ctx, revocationKey(claims, tokenString), until); err != nil {
		return fmt.Errorf("recording sign-out: %w", err)
	}
	return nil
}

// checkRevoked refuses a signed-out token, and refuses rather than accepts one whose state could not be read.
func (s *service) checkRevoked(ctx context.Context, claims *Claims, tokenString string) error {
	if s.revocations == nil {
		return nil
	}
	revoked, err := s.revocations.Revoked(ctx, revocationKey(claims, tokenString))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTokenCheck, err)
	}
	if revoked {
		return ErrTokenRevoked
	}
	return nil
}

// PostgresRevocationStore keeps signed-out tokens in revoked_tokens.
type PostgresRevocationStore struct {
	db *pgxpool.Pool
}

func NewPostgresRevocationStore(db *pgxpool.Pool) *PostgresRevocationStore {
	return &PostgresRevocationStore{db: db}
}

// Revoke inserts the key and prunes rows past their retention in the same statement, so nothing has to sweep this table.
func (s *PostgresRevocationStore) Revoke(ctx context.Context, key []byte, retainUntil time.Time) error {
	_, err := s.db.Exec(ctx, `
		WITH expired AS (
			DELETE FROM revoked_tokens WHERE retain_until < now()
		)
		INSERT INTO revoked_tokens (token_key, retain_until)
		VALUES ($1, $2)
		ON CONFLICT (token_key) DO NOTHING`,
		key, retainUntil)
	return err
}

// Revoked is a primary key lookup, made on every request that carries a JWT.
func (s *PostgresRevocationStore) Revoked(ctx context.Context, key []byte) (bool, error) {
	var revoked bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM revoked_tokens WHERE token_key = $1)`,
		key).Scan(&revoked)
	return revoked, err
}
