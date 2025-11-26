package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type AuthConfig struct {
	Providers map[string]*OAuthProviderConfig `mapstructure:"providers"`
}

type AnonymousAuthConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Role    string `mapstructure:"role"`
}

type OpenLineageAuthConfig struct {
	Enabled bool `mapstructure:"enabled"`
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
		Providers   map[string]*OAuthProviderConfig `mapstructure:"providers"`
		Anonymous   AnonymousAuthConfig             `mapstructure:"anonymous"`
		OpenLineage OpenLineageAuthConfig           `mapstructure:"openlineage"`
	} `mapstructure:"auth"`

	RateLimit RateLimitConfig `mapstructure:"rate_limit"`

	UI struct {
		Banner BannerConfig `mapstructure:"banner"`
	} `mapstructure:"ui"`
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

	// Explicitly bind nested provider config env vars
	v.BindEnv("auth.providers.okta.client_id")
	v.BindEnv("auth.providers.okta.client_secret")
	v.BindEnv("auth.providers.okta.url")
	v.BindEnv("auth.providers.okta.redirect_url")
	v.BindEnv("auth.providers.okta.enabled")
	v.BindEnv("auth.providers.okta.type")
	v.BindEnv("auth.providers.okta.name")
	v.BindEnv("auth.providers.okta.allow_signup")
	v.BindEnv("auth.providers.okta.team_sync.enabled")
	v.BindEnv("auth.providers.okta.team_sync.group_claim")

	v.BindEnv("auth.anonymous.enabled")
	v.BindEnv("auth.anonymous.role")
	v.BindEnv("auth.openlineage.enabled")

	v.BindEnv("server.root_url")

	// Rate limit env vars
	v.BindEnv("rate_limit.enabled")

	// UI banner env vars
	v.BindEnv("ui.banner.enabled")
	v.BindEnv("ui.banner.dismissible")
	v.BindEnv("ui.banner.variant")
	v.BindEnv("ui.banner.message")
	v.BindEnv("ui.banner.id")

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
	v.SetDefault("auth.openlineage.enabled", true)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")

	// Auth defaults
	v.SetDefault("auth.providers.okta.type", "okta")
	v.SetDefault("auth.providers.okta.name", "Okta")
	v.SetDefault("auth.providers.okta.allow_signup", true)
	v.SetDefault("auth.providers.okta.scopes", []string{"openid", "profile", "email", "groups", "offline_access"})
	v.SetDefault("auth.providers.okta.team_sync.enabled", false)
	v.SetDefault("auth.providers.okta.team_sync.group_claim", "groups")

	// Rate limit defaults
	v.SetDefault("rate_limit.enabled", false)

	// UI defaults
	v.SetDefault("ui.banner.enabled", false)
	v.SetDefault("ui.banner.dismissible", true)
	v.SetDefault("ui.banner.variant", "info")
	v.SetDefault("ui.banner.message", "")
	v.SetDefault("ui.banner.id", "banner-1")
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

	return nil
}
