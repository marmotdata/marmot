package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	validator "github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrRoleNotFound           = errors.New("role not found")
	ErrInvalidInput           = errors.New("invalid input")
	ErrAlreadyExists          = errors.New("user already exists")
	ErrReservedUsername       = errors.New("reserved username")
	ErrInvalidPassword        = errors.New("invalid password")
	ErrUnauthorized           = errors.New("unauthorized")
	ErrInvalidAPIKey          = errors.New("invalid API key")
	ErrPasswordRequired       = errors.New("password is required for non-OAuth users")
	ErrCannotDeleteSelf       = errors.New("user can't delete self")
	ErrCannotDeleteAdmin      = errors.New("can't delete admin user")
	ErrPasswordChangeRequired = errors.New("password change required")
)

type User struct {
	ID                 string                 `json:"id"`
	Username           string                 `json:"username"`
	Name               string                 `json:"name"`
	Active             bool                   `json:"active"`
	MustChangePassword bool                   `json:"must_change_password"`
	Preferences        map[string]interface{} `json:"preferences"`
	Roles              []Role                 `json:"roles"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions,omitempty"`
}

type Permission struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ResourceType string `json:"resource_type"`
	Action       string `json:"action"`
}

type CreateUserInput struct {
	Username  string   `json:"username" validate:"required,min=3,max=255"`
	Name      string   `json:"name" validate:"required"`
	Password  string   `json:"password" validate:"required_without=OAuthProvider,min=8"`
	RoleNames []string `json:"role_names" validate:"required,min=1"`

	OAuthProvider     string                 `json:"oauth_provider,omitempty"`
	OAuthProviderID   string                 `json:"oauth_provider_id,omitempty"`
	OAuthProviderData map[string]interface{} `json:"oauth_provider_data,omitempty"`
}

type UpdateUserInput struct {
	Email       *string                `json:"email,omitempty" validate:"omitempty,email"`
	Name        *string                `json:"name,omitempty"`
	Password    *string                `json:"password,omitempty" validate:"omitempty,min=8"`
	Active      *bool                  `json:"active,omitempty"`
	Preferences map[string]interface{} `json:"preferences,omitempty"`
	RoleNames   []string               `json:"role_names,omitempty" validate:"omitempty,min=1"`
}

type Filter struct {
	Query   string
	RoleIDs []string
	Active  *bool
	Limit   int
	Offset  int
}

type Service interface {
	Create(ctx context.Context, input CreateUserInput) (*User, error)
	Update(ctx context.Context, id string, input UpdateUserInput) (*User, error)
	Delete(ctx context.Context, currentUserId string, id string) error
	Get(ctx context.Context, id string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	List(ctx context.Context, filter Filter) ([]*User, int, error)

	// Authentication
	Authenticate(ctx context.Context, username, password string) (*User, error)
	ValidateAPIKey(ctx context.Context, apiKey string) (*User, error)
	HasPermission(ctx context.Context, userID string, resourceType string, action string) (bool, error)
	GetPermissionsByRoleName(ctx context.Context, roleName string) ([]Permission, error)

	// OAuth
	AuthenticateOAuth(ctx context.Context, provider string, providerUserID string, userInfo map[string]interface{}) (*User, error)
	LinkOAuthAccount(ctx context.Context, userID string, provider string, providerUserID string, userInfo map[string]interface{}) error
	UnlinkOAuthAccount(ctx context.Context, userID string, provider string) error

	// API Keys
	CreateAPIKey(ctx context.Context, userID string, name string, expiresIn *time.Duration) (*APIKey, error)
	DeleteAPIKey(ctx context.Context, userID string, keyID string) error
	ListAPIKeys(ctx context.Context, userID string) ([]*APIKey, error)

	UpdatePreferences(ctx context.Context, userID string, preferences map[string]interface{}) error
	UpdatePassword(ctx context.Context, userID string, newPassword string) (*User, error)
}

type service struct {
	repo      Repository
	validator *validator.Validate
}

type ServiceOption func(*service)

func NewService(repo Repository, opts ...ServiceOption) Service {
	s := &service{
		repo:      repo,
		validator: validator.New(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *service) GetPermissionsByRoleName(ctx context.Context, roleName string) ([]Permission, error) {
	return s.repo.GetPermissionsByRoleName(ctx, roleName)
}

func (s *service) UpdatePreferences(ctx context.Context, userID string, preferences map[string]interface{}) error {
	if err := s.repo.UpdatePreferences(ctx, userID, preferences); err != nil {
		return fmt.Errorf("updating user preferences: %w", err)
	}

	return nil
}

func (s *service) Create(ctx context.Context, input CreateUserInput) (*User, error) {
	if input.OAuthProvider == "" {
		if err := s.validator.Struct(input); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	} else {
		validate := validator.New()
		if err := validate.StructExcept(input, "Password"); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	}

	exists, err := s.repo.UsernameExists(ctx, input.Username)
	if err != nil {
		return nil, fmt.Errorf("checking username existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("%w: username already taken", ErrAlreadyExists)
	}

	if input.Username == "anonymous" {
		return nil, fmt.Errorf("%w: cannot create user with reserved username", ErrAlreadyExists)
	}

	var passwordHash string
	if input.OAuthProvider == "" {
		if input.Password == "" {
			return nil, ErrPasswordRequired
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hashing password: %w", err)
		}
		passwordHash = string(hash)
	}

	user := &User{
		Username:    input.Username,
		Name:        input.Name,
		Active:      true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Preferences: make(map[string]interface{}),
	}

	if err := s.repo.CreateUser(ctx, user, passwordHash); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	if err := s.repo.AssignRoles(ctx, user.ID, input.RoleNames); err != nil {
		_ = s.repo.DeleteUser(ctx, user.ID)
		return nil, fmt.Errorf("assigning roles: %w", err)
	}

	if input.OAuthProvider != "" {
		identity := &UserIdentity{
			UserID:         user.ID,
			Provider:       input.OAuthProvider,
			ProviderUserID: input.OAuthProviderID,
			ProviderEmail:  input.Username,
			ProviderData:   input.OAuthProviderData,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if err := s.repo.CreateUserIdentity(ctx, identity); err != nil {
			_ = s.repo.DeleteUser(ctx, user.ID)
			return nil, fmt.Errorf("creating user identity: %w", err)
		}
	}

	return s.Get(ctx, user.ID)
}

func (s *service) Update(ctx context.Context, id string, input UpdateUserInput) (*User, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	updates := make(map[string]interface{})

	if input.Email != nil {
		if *input.Email != "" {
			exists, err := s.repo.UsernameExists(ctx, *input.Email)
			if err != nil {
				return nil, fmt.Errorf("checking email existence: %w", err)
			}
			if exists {
				return nil, fmt.Errorf("%w: email already registered", ErrAlreadyExists)
			}
		}
		updates["email"] = *input.Email
	}

	if input.Name != nil {
		updates["name"] = *input.Name
	}

	if input.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hashing password: %w", err)
		}
		updates["password_hash"] = string(hash)
	}

	if input.Active != nil {
		updates["active"] = *input.Active
	}

	updates["updated_at"] = time.Now()

	if err := s.repo.UpdateUser(ctx, id, updates); err != nil {
		return nil, fmt.Errorf("updating user: %w", err)
	}

	if len(input.RoleNames) > 0 {
		if err := s.repo.UpdateRoles(ctx, id, input.RoleNames); err != nil {
			return nil, fmt.Errorf("updating roles: %w", err)
		}
	}

	return s.Get(ctx, id)
}

func (s *service) Delete(ctx context.Context, currentUserId string, id string) error {
	if currentUserId == id {
		return ErrCannotDeleteSelf
	}

	user, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return fmt.Errorf("getting user for deletion: %w", err)
	}

	if user.Username == "admin" {
		return ErrCannotDeleteAdmin
	}

	if err := s.repo.DeleteUser(ctx, id); err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	return nil
}

func (s *service) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *service) Get(ctx context.Context, id string) (*User, error) {
	user, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}
	return user, nil
}

func (s *service) List(ctx context.Context, filter Filter) ([]*User, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 50 // Default limit
	}

	users, total, err := s.repo.ListUsers(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("listing users: %w", err)
	}

	return users, total, nil
}

func (s *service) Authenticate(ctx context.Context, username, password string) (*User, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		if err == ErrUserNotFound {
			return nil, ErrInvalidPassword
		}
		return nil, fmt.Errorf("getting user: %w", err)
	}

	if !user.Active {
		return nil, ErrUnauthorized
	}

	if err := s.repo.ValidatePassword(ctx, user.ID, password); err != nil {
		return nil, ErrInvalidPassword
	}

	return user, nil
}

func (s *service) HasPermission(ctx context.Context, userID string, resourceType string, action string) (bool, error) {
	hasPermission, err := s.repo.HasPermission(ctx, userID, resourceType, action)
	if err != nil {
		return false, fmt.Errorf("checking permission: %w", err)
	}

	return hasPermission, nil
}

func (s *service) UpdatePassword(ctx context.Context, userID string, newPassword string) (*User, error) {
	if err := s.validator.Var(newPassword, "required,min=8"); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	updates := map[string]interface{}{
		"password_hash":        string(hash),
		"must_change_password": false,
		"updated_at":           time.Now(),
	}

	if err := s.repo.UpdateUser(ctx, userID, updates); err != nil {
		return nil, fmt.Errorf("updating user password: %w", err)
	}

	return s.Get(ctx, userID)
}
