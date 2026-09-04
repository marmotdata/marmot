package oauth2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/ory/fosite"
	"github.com/rs/zerolog/log"
)

const (
	// authorizeCodeLifespan is both the lifespan fosite is configured with and
	// the TTL of the rows behind a code, so the two cannot drift apart.
	authorizeCodeLifespan = 10 * time.Minute

	// PendingAuthorizeTTL is how long a browser has to finish signing in. It is
	// also the MaxAge of the oauth_session cookie pointing at the request.
	PendingAuthorizeTTL = 10 * time.Minute

	// LoginHandoffTTL is the window a browser has to redeem the ticket the SSO
	// callback set, which is one redirect away. It is also the MaxAge of the
	// cookie carrying the ticket.
	LoginHandoffTTL = 60 * time.Second

	// registrationTTL is how long a freshly registered client lives before
	// anyone has signed in with it. Registration is unauthenticated, so this
	// is the only thing bounding what an anonymous caller can leave behind:
	// it has to be short. Going from register to authorize takes seconds.
	registrationTTL = 10 * time.Minute

	// clientTTL is the life a client gets once a sign-in has completed with
	// it, measured from that sign-in. An MCP client that keeps signing in
	// never expires; the row the CLI leaves behind is gone the next day.
	clientTTL = 24 * time.Hour
)

// builtinCLIClient is the static client used by older CLI versions that skip
// dynamic registration. It never expires, so it is answered without a query.
var builtinCLIClient = &fosite.DefaultClient{
	ID:            "marmot-cli",
	Public:        true,
	RedirectURIs:  []string{"http://localhost"},
	GrantTypes:    []string{"authorization_code"},
	ResponseTypes: []string{"code"},
	Scopes:        []string{"openid"},
}

// Store is the fosite storage backend. It keeps nothing itself: every lookup
// goes to the Repository so that any replica can continue a login another
// replica started.
type Store struct {
	repo Repository
}

func NewStore(repo Repository) *Store {
	return &Store{repo: repo}
}

func (s *Store) GetClient(ctx context.Context, id string) (fosite.Client, error) {
	if id == builtinCLIClient.ID {
		return builtinCLIClient, nil
	}
	rec, err := s.repo.GetClient(ctx, id)
	if errors.Is(err, ErrNoRecord) {
		return nil, fosite.ErrNotFound
	}
	if err != nil {
		// Fosite turns any error here into "The requested OAuth 2.0 Client
		// does not exist", which is also what a genuinely unknown client
		// looks like. Log it so a database problem is not mistaken for one.
		log.Error().Err(err).Str("client_id", id).Msg("Failed to look up OAuth client")
		return nil, err
	}
	return rec.toFosite(), nil
}

// RegisterClient stores a dynamically registered client on a short lease.
// KeepClientAlive extends it once someone actually signs in.
func (s *Store) RegisterClient(ctx context.Context, client *ClientRecord) error {
	return s.repo.PutClient(ctx, client, registrationTTL)
}

// KeepClientAlive extends a client's life after a sign-in completed with it.
// Only authenticated steps call this. Reading a client does not extend it,
// because reading happens on the unauthenticated authorize endpoint and an
// anonymous caller could otherwise keep any number of rows alive for ever.
func (s *Store) KeepClientAlive(ctx context.Context, clientID string) error {
	if clientID == builtinCLIClient.ID {
		return nil
	}
	return s.repo.TouchClient(ctx, clientID, clientTTL)
}

func (s *Store) ClientAssertionJWTValid(_ context.Context, _ string) error {
	return nil
}

func (s *Store) SetClientAssertionJWT(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (s *Store) CreateAuthorizeCodeSession(ctx context.Context, code string, req fosite.Requester) error {
	data, err := encodeRequest(req)
	if err != nil {
		return err
	}
	return s.repo.PutSession(ctx, KindAuthorizeCode, code, data, authorizeCodeLifespan)
}

// GetAuthorizeCodeSession hydrates the session it is handed as well as the one
// it returns. Fosite validates the code against the caller's session before it
// swaps in the stored one, so leaving it empty would skip the expiry check.
func (s *Store) GetAuthorizeCodeSession(ctx context.Context, code string, session fosite.Session) (fosite.Requester, error) {
	data, err := s.repo.GetSession(ctx, KindAuthorizeCode, code)
	if errors.Is(err, ErrNoRecord) {
		return nil, fosite.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	req, err := decodeRequest(ctx, data, s.GetClient)
	if err != nil {
		return nil, err
	}
	if stored, ok := req.GetSession().(*MarmotSession); ok {
		if into, ok := session.(*MarmotSession); ok && into != nil {
			stored.copyInto(into)
		}
	}
	return req, nil
}

// InvalidateAuthorizeCodeSession drops the code. Marmot mints its own JWT
// rather than calling fosite's NewAccessResponse, so fosite never reaches this;
// single use is enforced by GetPKCERequestSession consuming the PKCE row.
func (s *Store) InvalidateAuthorizeCodeSession(ctx context.Context, code string) error {
	return s.repo.DeleteSession(ctx, KindAuthorizeCode, code)
}

func (s *Store) CreatePKCERequestSession(ctx context.Context, signature string, req fosite.Requester) error {
	data, err := encodeRequest(req)
	if err != nil {
		return err
	}
	return s.repo.PutSession(ctx, KindPKCE, signature, data, authorizeCodeLifespan)
}

// GetPKCERequestSession consumes the row. This is what makes an authorization
// code single use: fosite reads the PKCE session once per token request and
// deletes it straight after, and doing both in one statement closes the window
// where two replicas redeem the same code at the same moment.
func (s *Store) GetPKCERequestSession(ctx context.Context, signature string, _ fosite.Session) (fosite.Requester, error) {
	data, err := s.repo.TakeSession(ctx, KindPKCE, signature)
	if errors.Is(err, ErrNoRecord) {
		return nil, fosite.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeRequest(ctx, data, s.GetClient)
}

// DeletePKCERequestSession is a no-op: GetPKCERequestSession already removed
// the row.
func (s *Store) DeletePKCERequestSession(_ context.Context, _ string) error {
	return nil
}

// PutPendingAuthorize stores the authorize request a browser is signing in for,
// keyed by the id in the oauth_session cookie.
func (s *Store) PutPendingAuthorize(ctx context.Context, sessionID string, req fosite.AuthorizeRequester) error {
	data, err := encodeRequest(req)
	if err != nil {
		return err
	}
	return s.repo.PutSession(ctx, KindPendingAuth, sessionID, data, PendingAuthorizeTTL)
}

// GetPendingAuthorize returns ErrNoRecord when there is nothing pending.
func (s *Store) GetPendingAuthorize(ctx context.Context, sessionID string) (fosite.AuthorizeRequester, error) {
	data, err := s.repo.GetSession(ctx, KindPendingAuth, sessionID)
	if err != nil {
		return nil, err
	}
	req, err := decodeRequest(ctx, data, s.GetClient)
	if err != nil {
		return nil, err
	}
	ar, ok := req.(fosite.AuthorizeRequester)
	if !ok {
		return nil, errors.New("stored pending authorize is not an authorize request")
	}
	return ar, nil
}

func (s *Store) DeletePendingAuthorize(ctx context.Context, sessionID string) error {
	return s.repo.DeleteSession(ctx, KindPendingAuth, sessionID)
}

// PutLoginHandoff parks the user an SSO callback just authenticated, so the
// page it redirects to can trade the ticket for a token. Only the user id is
// stored: the token is minted at redemption, so no credential sits in the
// database.
func (s *Store) PutLoginHandoff(ctx context.Context, ticket, userID string) error {
	data, err := encodeLoginHandoff(userID)
	if err != nil {
		return err
	}
	return s.repo.PutSession(ctx, KindLoginHandoff, hashTicket(ticket), data, LoginHandoffTTL)
}

// hashTicket keys a handoff by a digest rather than by the ticket itself. The
// ticket is what redeems a session, so a reader of the table should not be
// handed one. A fast hash is right here: the input is 32 bytes from
// crypto/rand, so there is no dictionary to attack.
func hashTicket(ticket string) string {
	sum := sha256.Sum256([]byte(ticket))
	return hex.EncodeToString(sum[:])
}

// TakeLoginHandoff redeems a ticket and returns the user id it was issued for.
// It returns ErrNoRecord when the ticket is unknown, expired or already spent.
func (s *Store) TakeLoginHandoff(ctx context.Context, ticket string) (string, error) {
	data, err := s.repo.TakeSession(ctx, KindLoginHandoff, hashTicket(ticket))
	if err != nil {
		return "", err
	}
	return decodeLoginHandoff(data)
}

// Marmot issues its own stateless JWTs rather than fosite access tokens, so
// nothing ever reads these back. They exist only to satisfy the interface.
func (s *Store) CreateAccessTokenSession(_ context.Context, _ string, _ fosite.Requester) error {
	return nil
}

func (s *Store) GetAccessTokenSession(_ context.Context, _ string, _ fosite.Session) (fosite.Requester, error) {
	return nil, fosite.ErrNotFound
}

func (s *Store) DeleteAccessTokenSession(_ context.Context, _ string) error {
	return nil
}

// Marmot does not issue refresh tokens; these are unused.
func (s *Store) CreateRefreshTokenSession(_ context.Context, _ string, _ string, _ fosite.Requester) error {
	return nil
}

func (s *Store) GetRefreshTokenSession(_ context.Context, _ string, _ fosite.Session) (fosite.Requester, error) {
	return nil, fosite.ErrNotFound
}

func (s *Store) DeleteRefreshTokenSession(_ context.Context, _ string) error {
	return nil
}

func (s *Store) RotateRefreshToken(_ context.Context, _ string, _ string) error {
	return nil
}

func (s *Store) RevokeRefreshToken(_ context.Context, _ string) error {
	return nil
}

func (s *Store) RevokeAccessToken(_ context.Context, _ string) error {
	return nil
}

func (s *Store) RevokeRefreshTokenMaybeGracePeriod(_ context.Context, _ string, _ string) error {
	return nil
}
