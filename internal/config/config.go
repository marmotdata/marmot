package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type AnonymousAuthConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Role    string `mapstructure:"role"`
}

type RateLimitConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

type OAuthProviderConfig struct {
	Enabled      bool             `mapstructure:"enabled"`
	Type         string           `mapstructure:"type"`
	Name         string           `mapstructure:"name"`
	ClientID     string           `mapstructure:"client_id"`
	ClientSecret string           `mapstructure:"client_secret"`
	URL          string           `mapstructure:"url"`
	RedirectURL  string           `mapstructure:"redirect_url"`
	Scopes       []string         `mapstructure:"scopes"`
	AllowSignup  bool             `mapstructure:"allow_signup"`
	GroupMapping []GroupMapConfig `mapstructure:"group_mapping"`
	TeamSync     TeamSyncConfig   `mapstructure:"team_sync"`
}

type GroupMapConfig struct {
	GroupName string   `mapstructure:"group_name"`
	Roles     []string `mapstructure:"roles"`
}

type TeamSyncConfig struct {
	Enabled     bool            `mapstructure:"enabled"`
	StripPrefix string          `mapstructure:"strip_prefix"`
	Group       TeamGroupConfig `mapstructure:"group"`
}

type TeamGroupConfig struct {
	Claim  string          `mapstructure:"claim"`
	Filter TeamGroupFilter `mapstructure:"filter"`
}

type TeamGroupFilter struct {
	Mode    string `mapstructure:"mode"`
	Pattern string `mapstructure:"pattern"`
}

// Config holds all configuration for the application
type Config struct {
	Server struct {
		Port                  int               `mapstructure:"port"`
		Host                  string            `mapstructure:"host"`
		RootURL               string            `mapstructure:"root_url"`
		CustomResponseHeaders map[string]string `mapstructure:"customer_response_headers"`
		EncryptionKey         string            `mapstructure:"encryption_key"`
		AllowUnencrypted      bool              `mapstructure:"allow_unencrypted"`
	} `mapstructure:"server"`

	Metrics struct {
		Enabled             bool     `mapstructure:"enabled"`
		Port                int      `mapstructure:"port"`
		OwnerMetadataFields []string `mapstructure:"owner_metadata_fields"`
		Schemas             struct {
			ExcludedAssetTypes []string `mapstructure:"excluded_asset_types"`
			ExcludedProviders  []string `mapstructure:"excluded_providers"`
		} `mapstructure:"schemas"`
	} `mapstructure:"metrics"`

	Database struct {
		Host         string `mapstructure:"host"`
		Port         int    `mapstructure:"port"`
		User         string `mapstructure:"user"`
		Password     string `mapstructure:"password"`
		Name         string `mapstructure:"name"`
		SSLMode      string `mapstructure:"sslmode"`
		MaxConns     int    `mapstructure:"max_conns"`
		IdleConns    int    `mapstructure:"idle_conns"`
		ConnLifetime int    `mapstructure:"conn_lifetime"`
	} `mapstructure:"database"`

	Logging struct {
		Level  string `mapstructure:"level"`
		Format string `mapstructure:"format"`
	} `mapstructure:"logging"`

	Auth struct {
		Google    *OAuthProviderConfig `mapstructure:"google"`
		GitHub    *OAuthProviderConfig `mapstructure:"github"`
		GitLab    *OAuthProviderConfig `mapstructure:"gitlab"`
		Okta      *OAuthProviderConfig `mapstructure:"okta"`
		Slack     *OAuthProviderConfig `mapstructure:"slack"`
		Auth0     *OAuthProviderConfig `mapstructure:"auth0"`
		Anonymous AnonymousAuthConfig  `mapstructure:"anonymous"`
	} `mapstructure:"auth"`

	OpenLineage struct {
		Auth struct {
			Enabled bool `mapstructure:"enabled"`
		} `mapstructure:"auth"`
	} `mapstructure:"openlineage"`

	RateLimit RateLimitConfig `mapstructure:"rate_limit"`

	UI struct {
		Banner BannerConfig `mapstructure:"banner"`
	} `mapstructure:"ui"`

	Pipelines struct {
		MaxWorkers        int `mapstructure:"max_workers"`
		SchedulerInterval int `mapstructure:"scheduler_interval"`
		LeaseExpiry       int `mapstructure:"lease_expiry"`
		ClaimExpiry       int `mapstructure:"claim_expiry"`
	} `mapstructure:"pipelines"`
}

type BannerConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	Dismissible bool   `mapstructure:"dismissible"`
	Variant     string `mapstructure:"variant"`
	Message     string `mapstructure:"message"`
	ID          string `mapstructure:"id"`
}

var (
	config *Config
	once   sync.Once
)

// Load initializes and loads the config
func Load(configPath string) (*Config, error) {
	var err error
	once.Do(func() {
		err = loadConfig(configPath)
	})
	return config, err
}

// Get returns the current config, panics if config is not loaded
func Get() *Config {
	if config == nil {
		panic("config is not loaded")
	}
	return config
}

func loadConfig(configPath string) error {
	v := viper.New()

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.AddConfigPath(".")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	// Read config file first
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		fmt.Printf("No config file found, using defaults and environment variables\n")
	}

	// Set up environment variables
	v.SetEnvPrefix("MARMOT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicitly bind auth provider config env vars
	v.BindEnv("auth.okta.client_id")
	v.BindEnv("auth.okta.client_secret")
	v.BindEnv("auth.okta.url")
	v.BindEnv("auth.okta.redirect_url")
	v.BindEnv("auth.okta.enabled")
	v.BindEnv("auth.okta.type")
	v.BindEnv("auth.okta.name")
	v.BindEnv("auth.okta.allow_signup")
	v.BindEnv("auth.okta.team_sync.enabled")
	v.BindEnv("auth.okta.team_sync.group_claim")

	v.BindEnv("auth.google.client_id")
	v.BindEnv("auth.google.client_secret")
	v.BindEnv("auth.google.redirect_url")
	v.BindEnv("auth.google.enabled")
	v.BindEnv("auth.google.type")
	v.BindEnv("auth.google.name")
	v.BindEnv("auth.google.allow_signup")

	v.BindEnv("auth.github.client_id")
	v.BindEnv("auth.github.client_secret")
	v.BindEnv("auth.github.redirect_url")
	v.BindEnv("auth.github.enabled")
	v.BindEnv("auth.github.type")
	v.BindEnv("auth.github.name")
	v.BindEnv("auth.github.allow_signup")

	v.BindEnv("auth.gitlab.client_id")
	v.BindEnv("auth.gitlab.client_secret")
	v.BindEnv("auth.gitlab.url")
	v.BindEnv("auth.gitlab.redirect_url")
	v.BindEnv("auth.gitlab.enabled")
	v.BindEnv("auth.gitlab.type")
	v.BindEnv("auth.gitlab.name")
	v.BindEnv("auth.gitlab.allow_signup")

	v.BindEnv("auth.slack.client_id")
	v.BindEnv("auth.slack.client_secret")
	v.BindEnv("auth.slack.redirect_url")
	v.BindEnv("auth.slack.enabled")
	v.BindEnv("auth.slack.type")
	v.BindEnv("auth.slack.name")
	v.BindEnv("auth.slack.allow_signup")

	v.BindEnv("auth.auth0.client_id")
	v.BindEnv("auth.auth0.client_secret")
	v.BindEnv("auth.auth0.url")
	v.BindEnv("auth.auth0.redirect_url")
	v.BindEnv("auth.auth0.enabled")
	v.BindEnv("auth.auth0.type")
	v.BindEnv("auth.auth0.name")
	v.BindEnv("auth.auth0.allow_signup")
	v.BindEnv("auth.auth0.team_sync.enabled")
	v.BindEnv("auth.auth0.team_sync.group_claim")

	v.BindEnv("auth.anonymous.enabled")
	v.BindEnv("auth.anonymous.role")

	v.BindEnv("openlineage.auth.enabled")

	v.BindEnv("server.root_url")
	v.BindEnv("server.encryption_key")
	v.BindEnv("server.allow_unencrypted")

	// Rate limit env vars
	v.BindEnv("rate_limit.enabled")

	// UI banner env vars
	v.BindEnv("ui.banner.enabled")
	v.BindEnv("ui.banner.dismissible")
	v.BindEnv("ui.banner.variant")
	v.BindEnv("ui.banner.message")
	v.BindEnv("ui.banner.id")

	// Pipelines env vars
	v.BindEnv("pipelines.max_workers")
	v.BindEnv("pipelines.scheduler_interval")
	v.BindEnv("pipelines.lease_expiry")
	v.BindEnv("pipelines.claim_expiry")

	// Set defaults
	setDefaults(v)

	// Initialize config struct
	config = &Config{}
	if err := v.Unmarshal(config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return validate(config)
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.allow_unencrypted", false)

	// Metrics defaults
	v.SetDefault("metrics.enabled", false)
	v.SetDefault("metrics.port", 9090)
	v.SetDefault("metrics.owner_metadata_fields", []string{"owner", "ownedBy", "owningTeam"})
	v.SetDefault("metrics.schemas.excluded_asset_types", []string{"Service"})
	v.SetDefault("metrics.schemas.excluded_providers", []string{})

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.name", "marmot")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_conns", 25)
	v.SetDefault("database.idle_conns", 5)
	v.SetDefault("database.conn_lifetime", 5) // minutes

	v.SetDefault("auth.anonymous.role", "user")

	// OpenLineage defaults
	v.SetDefault("openlineage.auth.enabled", true)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")

	// Auth defaults
	v.SetDefault("auth.okta.type", "okta")
	v.SetDefault("auth.okta.name", "Okta")
	v.SetDefault("auth.okta.allow_signup", true)
	v.SetDefault("auth.okta.scopes", []string{"openid", "profile", "email", "groups", "offline_access"})
	v.SetDefault("auth.okta.team_sync.enabled", false)
	v.SetDefault("auth.okta.team_sync.group_claim", "groups")

	v.SetDefault("auth.google.type", "google")
	v.SetDefault("auth.google.name", "Google")
	v.SetDefault("auth.google.allow_signup", true)
	v.SetDefault("auth.google.scopes", []string{"openid", "profile", "email"})

	v.SetDefault("auth.github.type", "github")
	v.SetDefault("auth.github.name", "GitHub")
	v.SetDefault("auth.github.allow_signup", true)
	v.SetDefault("auth.github.scopes", []string{"user:email"})

	v.SetDefault("auth.gitlab.type", "gitlab")
	v.SetDefault("auth.gitlab.name", "GitLab")
	v.SetDefault("auth.gitlab.allow_signup", true)
	v.SetDefault("auth.gitlab.url", "https://gitlab.com")
	v.SetDefault("auth.gitlab.scopes", []string{"openid", "profile", "email"})

	v.SetDefault("auth.slack.type", "slack")
	v.SetDefault("auth.slack.name", "Slack")
	v.SetDefault("auth.slack.allow_signup", true)
	v.SetDefault("auth.slack.scopes", []string{"openid", "profile", "email"})

	v.SetDefault("auth.auth0.type", "auth0")
	v.SetDefault("auth.auth0.name", "Auth0")
	v.SetDefault("auth.auth0.allow_signup", true)
	v.SetDefault("auth.auth0.scopes", []string{"openid", "profile", "email"})
	v.SetDefault("auth.auth0.team_sync.enabled", false)
	v.SetDefault("auth.auth0.team_sync.group_claim", "groups")

	// Rate limit defaults
	v.SetDefault("rate_limit.enabled", false)

	// UI defaults
	v.SetDefault("ui.banner.enabled", false)
	v.SetDefault("ui.banner.dismissible", true)
	v.SetDefault("ui.banner.variant", "info")
	v.SetDefault("ui.banner.message", "")
	v.SetDefault("ui.banner.id", "banner-1")

	// Pipelines defaults
	v.SetDefault("pipelines.max_workers", 10)
	v.SetDefault("pipelines.scheduler_interval", 60)
	v.SetDefault("pipelines.lease_expiry", 300)
	v.SetDefault("pipelines.claim_expiry", 30)
}

// BuildDSN builds a PostgreSQL connection string from config
func (c *Config) BuildDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Name,
		c.Database.SSLMode,
	)
}

func validate(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.Database.Port < 1 || cfg.Database.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", cfg.Database.Port)
	}

	validLevels := map[string]bool{
		"trace": true,
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
		"panic": true,
	}
	if !validLevels[strings.ToLower(cfg.Logging.Level)] {
		return fmt.Errorf("invalid logging level: %s", cfg.Logging.Level)
	}

	validFormats := map[string]bool{
		"json":    true,
		"console": true,
	}
	if !validFormats[strings.ToLower(cfg.Logging.Format)] {
		return fmt.Errorf("invalid logging format: %s", cfg.Logging.Format)
	}

	validVariants := map[string]bool{
		"info":    true,
		"warning": true,
		"error":   true,
		"success": true,
	}
	if cfg.UI.Banner.Enabled && !validVariants[strings.ToLower(cfg.UI.Banner.Variant)] {
		return fmt.Errorf("invalid banner variant: %s", cfg.UI.Banner.Variant)
	}

	if cfg.Pipelines.MaxWorkers < 1 {
		return fmt.Errorf("invalid pipelines.max_workers: must be at least 1")
	}
	if cfg.Pipelines.SchedulerInterval < 1 {
		return fmt.Errorf("invalid pipelines.scheduler_interval: must be at least 1 second")
	}
	if cfg.Pipelines.LeaseExpiry < 1 {
		return fmt.Errorf("invalid pipelines.lease_expiry: must be at least 1 second")
	}
	if cfg.Pipelines.ClaimExpiry < 1 {
		return fmt.Errorf("invalid pipelines.claim_expiry: must be at least 1 second")
	}

	return nil
}
