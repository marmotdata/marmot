package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/limits"
	"github.com/marmotdata/marmot/internal/core/settings"
	"golang.org/x/crypto/bcrypt"
)

// InviteTTL is how long an invitation link works. Seven days covers a week of leave without leaving a working link around for long.
const InviteTTL = 7 * 24 * time.Hour

const (
	StatusActive      = "active"
	StatusInvited     = "invited"
	StatusDeactivated = "deactivated"
)

var (
	ErrInviteNotFound         = errors.New("invitation not found")
	ErrInviteExpired          = errors.New("invitation has expired")
	ErrNotInvited             = errors.New("user has no pending invitation")
	ErrSignupDisabled         = errors.New("new members must be invited")
	ErrPasswordSignInDisabled = errors.New("password sign-in is turned off")
	ErrUnverifiedInvitedEmail = errors.New("identity provider did not verify the invited email address")
	errInviteTokenGeneration  = errors.New("generating invitation token")
)

// InviteInput is who to invite and what they start with.
type InviteInput struct {
	Email     string   `json:"email" validate:"required,email,max=255"`
	Name      string   `json:"name,omitempty" validate:"max=255"`
	RoleNames []string `json:"role_names" validate:"required,min=1"`
} // @name InviteUserInput

// Invitation is a pending member and the one-time token for their link. The token is only ever returned here, when the invitation is created or renewed.
type Invitation struct {
	User      *User     `json:"user"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
} // @name Invitation

// InviteInfo is what the person holding a link may learn about it before accepting.
type InviteInfo struct {
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	ExpiresAt time.Time `json:"expires_at"`
	// PasswordSignIn says whether accepting can set a password; when false the person signs in with single sign-on instead.
	PasswordSignIn bool `json:"password_sign_in"`
} // @name InviteInfo

// Invite is a pending invitation as stored.
type Invite struct {
	UserID    string
	Email     string
	Name      string
	ExpiresAt time.Time
}

// MemberState is what the member list shows beside the account itself.
type MemberState struct {
	LastSeenAt      *time.Time
	InviteExpiresAt *time.Time
}

// WithSettings installs the instance settings that decide how people may sign in. Without it the defaults apply.
func WithSettings(store settings.Store) ServiceOption {
	return func(s *service) {
		s.settings = store
	}
}

func (s *service) SignInSettings(ctx context.Context) settings.SignIn {
	out, err := settings.GetSignIn(ctx, s.settings)
	if err != nil {
		// The defaults are how Marmot behaved before the settings existed, so a failed read degrades to that rather than refusing every sign-in.
		return settings.DefaultSignIn()
	}
	return out
}

// newInviteToken returns a token for the link and the hash that is stored, so a leaked database row cannot be turned into a working link.
func newInviteToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", errInviteTokenGeneration
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	return token, hashInviteToken(token), nil
}

func hashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *service) Invite(ctx context.Context, input InviteInput, invitedBy string) (*Invitation, error) {
	input.Email = strings.TrimSpace(input.Email)
	input.Name = strings.TrimSpace(input.Name)
	if err := s.validator.Struct(input); err != nil {
		return nil, invalidInput(err)
	}
	if input.Name == "" {
		input.Name = strings.SplitN(input.Email, "@", 2)[0]
	}

	if err := s.guard.CheckCreate(ctx, limits.ResourceUsers); err != nil {
		return nil, err
	}

	// The email is the username, as it is for members created by single sign-on, so the invited person and their identity provider account meet on the same row.
	exists, err := s.repo.UsernameExists(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("checking username existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("%w: a member with this email already exists", ErrAlreadyExists)
	}

	now := time.Now()
	u := &User{
		Username:    input.Email,
		Name:        input.Name,
		Active:      false,
		CreatedAt:   now,
		UpdatedAt:   now,
		Preferences: make(map[string]interface{}),
	}
	if err := s.repo.CreateUser(ctx, u, ""); err != nil {
		return nil, fmt.Errorf("creating invited user: %w", err)
	}
	if err := s.repo.AssignRoles(ctx, u.ID, input.RoleNames); err != nil {
		_ = s.repo.DeleteUser(ctx, u.ID)
		return nil, fmt.Errorf("assigning roles: %w", err)
	}

	inv, err := s.issueInvite(ctx, u.ID, input.Email, invitedBy)
	if err != nil {
		_ = s.repo.DeleteUser(ctx, u.ID)
		return nil, err
	}
	return inv, nil
}

func (s *service) issueInvite(ctx context.Context, userID, email, invitedBy string) (*Invitation, error) {
	token, hash, err := newInviteToken()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(InviteTTL)
	var by *string
	if invitedBy != "" {
		by = &invitedBy
	}
	if err := s.repo.SaveInvite(ctx, userID, email, hash, by, expiresAt); err != nil {
		return nil, fmt.Errorf("saving invitation: %w", err)
	}
	u, err := s.GetMember(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &Invitation{User: u, Token: token, ExpiresAt: expiresAt}, nil
}

// RenewInvite replaces an invitation's link with a new one that works for another InviteTTL, so a lost or expired link can be sent again and the old one stops working.
func (s *service) RenewInvite(ctx context.Context, userID, invitedBy string) (*Invitation, error) {
	inv, err := s.repo.GetInvite(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.issueInvite(ctx, userID, inv.Email, invitedBy)
}

// RevokeInvite withdraws an invitation. The pending account only existed for the invitation, so it goes too.
func (s *service) RevokeInvite(ctx context.Context, userID string) error {
	if _, err := s.repo.GetInvite(ctx, userID); err != nil {
		return err
	}
	return s.repo.DeleteUser(ctx, userID)
}

func (s *service) lookupInvite(ctx context.Context, token string) (*Invite, error) {
	if token == "" {
		return nil, ErrInviteNotFound
	}
	inv, err := s.repo.GetInviteByTokenHash(ctx, hashInviteToken(token))
	if err != nil {
		return nil, err
	}
	if time.Now().After(inv.ExpiresAt) {
		return nil, ErrInviteExpired
	}
	return inv, nil
}

func (s *service) GetInvite(ctx context.Context, token string) (*InviteInfo, error) {
	inv, err := s.lookupInvite(ctx, token)
	if err != nil {
		return nil, err
	}
	return &InviteInfo{
		Email:          inv.Email,
		Name:           inv.Name,
		ExpiresAt:      inv.ExpiresAt,
		PasswordSignIn: s.SignInSettings(ctx).AllowPasswordSignIn,
	}, nil
}

// AcceptInvite sets the invited member's password and activates them. It refuses when password sign-in is off, since the password could not then be used; those members accept by signing in with single sign-on.
func (s *service) AcceptInvite(ctx context.Context, token, password, name string) (*User, error) {
	inv, err := s.lookupInvite(ctx, token)
	if err != nil {
		return nil, err
	}
	if !s.SignInSettings(ctx).AllowPasswordSignIn {
		return nil, ErrPasswordSignInDisabled
	}
	if err := s.validator.Var(password, "required,min=8"); err != nil {
		return nil, &InvalidInputError{Fields: []FieldError{{
			Field:   "password",
			Message: "is required and must be at least 8 characters",
		}}}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}
	if err := s.repo.AcceptInvite(ctx, inv.UserID, string(hash), strings.TrimSpace(name)); err != nil {
		return nil, err
	}
	return s.GetMember(ctx, inv.UserID)
}

// GetMember is Get plus the member's status and last seen time. Get stays without them because it runs on every authenticated request.
func (s *service) GetMember(ctx context.Context, id string) (*User, error) {
	u, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.attachMemberStates(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *service) attachMemberStates(ctx context.Context, users ...*User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	states, err := s.repo.GetMemberStates(ctx, ids)
	if err != nil {
		return fmt.Errorf("getting member states: %w", err)
	}
	for _, u := range users {
		st := states[u.ID]
		u.LastSeenAt = st.LastSeenAt
		u.InviteExpiresAt = st.InviteExpiresAt
		switch {
		case st.InviteExpiresAt != nil:
			u.Status = StatusInvited
		case u.Active:
			u.Status = StatusActive
		default:
			u.Status = StatusDeactivated
		}
	}
	return nil
}

// ProvisionSSO is what single sign-on does for someone it has no linked account for. An invited member whose email the identity provider vouches for is activated and linked; anyone else becomes a new member with the default role, unless the instance only admits invited people. The bool reports whether a new account was created.
func (s *service) ProvisionSSO(ctx context.Context, input CreateUserInput) (*User, bool, error) {
	if input.OAuthProvider == "" {
		return nil, false, fmt.Errorf("%w: provider is required", ErrInvalidInput)
	}

	inv, err := s.repo.FindInviteByEmail(ctx, input.Username)
	if err != nil && !errors.Is(err, ErrInviteNotFound) {
		return nil, false, fmt.Errorf("checking invitations: %w", err)
	}
	if inv != nil {
		// An identity provider that says it has not verified the address must not be able to claim the invitation for it.
		if verified, ok := input.OAuthProviderData["email_verified"].(bool); ok && !verified {
			return nil, false, ErrUnverifiedInvitedEmail
		}
		identity := &UserIdentity{
			UserID:         inv.UserID,
			Provider:       input.OAuthProvider,
			ProviderUserID: input.OAuthProviderID,
			ProviderEmail:  input.Username,
			ProviderData:   input.OAuthProviderData,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := s.repo.CreateUserIdentity(ctx, identity); err != nil {
			return nil, false, fmt.Errorf("linking invited user: %w", err)
		}
		if err := s.repo.AcceptInvite(ctx, inv.UserID, "", ""); err != nil {
			return nil, false, err
		}
		u, err := s.GetMember(ctx, inv.UserID)
		return u, false, err
	}

	cfg := s.SignInSettings(ctx)
	if !cfg.CreateOnFirstSSO {
		return nil, false, ErrSignupDisabled
	}
	if cfg.DefaultRole != "" {
		input.RoleNames = []string{cfg.DefaultRole}
	}
	if len(input.RoleNames) == 0 {
		input.RoleNames = []string{"user"}
	}
	u, err := s.Create(ctx, input)
	if err != nil {
		return nil, false, err
	}
	return u, true, nil
}

// SetRolesFromSSO replaces a member's roles with those their identity provider groups map to. Nothing changes when the names are already the member's roles, so most sign-ins write nothing.
func (s *service) SetRolesFromSSO(ctx context.Context, userID string, roleNames []string) error {
	if len(roleNames) == 0 {
		return nil
	}
	u, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	have := make(map[string]bool, len(u.Roles))
	for _, r := range u.Roles {
		have[r.Name] = true
	}
	want := make(map[string]bool, len(roleNames))
	for _, n := range roleNames {
		want[n] = true
	}
	if len(have) == len(want) {
		same := true
		for n := range want {
			if !have[n] {
				same = false
				break
			}
		}
		if same {
			return nil
		}
	}
	return s.repo.UpdateRoles(ctx, userID, roleNames)
}

func (s *service) CountIdentitiesByProvider(ctx context.Context) (map[string]int, error) {
	return s.repo.CountIdentitiesByProvider(ctx)
}

func isAdmin(u *User) bool {
	for _, r := range u.Roles {
		if r.Name == "admin" {
			return true
		}
	}
	return false
}
