// Package sftp discovers directories and files from an SFTP server.
//
// A server is catalogued the way someone browsing it would describe it:
// a Folder holds Files and other Folders, and every asset is named by
// its path below the configured root. Delimited and JSON files also get
// their columns, so a drop folder of CSVs reads like a set of tables.
package sftp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the Marmot provider for everything this plugin discovers.
// It matches the OpenMetadata plugin's projection for an Sftp service,
// which is what lets a catalog imported from OpenMetadata and one read
// straight from the server land on the same assets.
const provider = "SFTP"

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "sftp",
		Name:        "SFTP",
		Description: "Discover directories and files from an SFTP server",
		Icon:        "sftp",
		Category:    "storage",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the SFTP plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	// Connection
	Host     string `json:"host" description:"SFTP server hostname or IP address" validate:"required"`
	Port     int    `json:"port" description:"SFTP server port" default:"22" validate:"min=1,max=65535"`
	Username string `json:"username" description:"User to log in as" validate:"required"`

	// Authentication. One of password or private_key is required.
	Password             string `json:"password,omitempty" description:"Password for the user" sensitive:"true"`
	PrivateKey           string `json:"private_key,omitempty" label:"Private Key" description:"PEM private key for the user, RSA, Ed25519 or ECDSA" sensitive:"true"`
	PrivateKeyPassphrase string `json:"private_key_passphrase,omitempty" label:"Private Key Passphrase" description:"Passphrase protecting the private key" sensitive:"true"`
	HostKey              string `json:"host_key,omitempty" label:"Host Key" description:"Server public key to trust, as written in known_hosts. Empty means any key is accepted"`

	// What to walk
	RootDirectories []string `json:"root_directories" description:"Directories to walk" default:"[\"/\"]"`
	MaxDepth        int      `json:"max_depth" description:"Levels below each root to walk" default:"10" validate:"min=1,max=100"`
	MaxFiles        int      `json:"max_files" description:"Stop after this many files" default:"20000" validate:"min=1"`
	FollowSymlinks  bool     `json:"follow_symlinks" description:"Walk into symlinks instead of skipping them" default:"false"`
	StructuredOnly  bool     `json:"structured_only" description:"Only catalogue csv, tsv, json, jsonl, parquet and avro files" default:"false"`

	// What to read from each file
	IncludeColumns    bool  `json:"include_columns" description:"Infer columns for delimited and JSON files" default:"true"`
	IncludeStatistics bool  `json:"include_statistics" description:"Emit size, row count and column count statistics" default:"true"`
	SampleRows        int   `json:"sample_rows" description:"Rows read from a file to infer its columns" default:"200" validate:"min=1,max=10000"`
	MaxReadBytes      int64 `json:"max_read_bytes" description:"Most bytes to read from a single file" default:"33554432" validate:"min=1"`
}

// Example configuration for the plugin
var _ = `
host: "sftp.company.com"
port: 22
username: "marmot"
private_key: "${SFTP_PRIVATE_KEY}"
root_directories:
  - "/data/incoming"
  - "/data/archive"
max_depth: 5
structured_only: true
tags:
  - "sftp"
`

// Source represents the SFTP plugin.
type Source struct {
	config *Config
}

// unverifiedHostKeyOnce keeps the warning about an unpinned host key to
// one line per process, however many times Validate runs.
var unverifiedHostKeyOnce sync.Once

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	config.Host = strings.TrimSpace(config.Host)
	// A host copied from a client often carries the scheme.
	config.Host = strings.TrimPrefix(strings.TrimPrefix(config.Host, "sftp://"), "ssh://")
	config.Username = strings.TrimSpace(config.Username)

	config.RootDirectories = normaliseRoots(config.RootDirectories)

	if config.Password == "" && strings.TrimSpace(config.PrivateKey) == "" {
		return nil, fmt.Errorf("either password or private_key is required")
	}

	// Parsing the key here rather than at connect time turns a typo or an
	// unsupported key type into an error the config form can show.
	if _, err := authMethods(config); err != nil {
		return nil, err
	}
	if _, err := hostKeyCallback(config); err != nil {
		return nil, err
	}
	if config.HostKey == "" {
		unverifiedHostKeyOnce.Do(func() {
			log.Warn().Str("host", config.Host).Msg("No host_key configured, the server's identity is not verified")
		})
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover walks the configured roots and catalogues what it finds.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	conn, err := connect(ctx, s.config)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", s.config.address(), err)
	}
	defer conn.Close()

	result := s.collect(ctx, conn)

	log.Info().
		Int("assets", len(result.Assets)).
		Int("lineages", len(result.Lineage)).
		Int("statistics", len(result.Statistics)).
		Msg("SFTP discovery completed")

	return result, nil
}

// collect turns a walk of the server into assets, lineage and
// statistics. It is separate from Discover so tests can run the whole
// pipeline against a directory on disk instead of a live server.
func (s *Source) collect(ctx context.Context, fsys fileSystem) *pluginsdk.DiscoveryResult {
	w := newWalker(fsys, s.config)
	for _, root := range s.config.RootDirectories {
		if err := w.walkRoot(ctx, root); err != nil {
			// One unreadable root should not lose the others.
			log.Warn().Err(err).Str("root", root).Msg("Failed to walk root directory")
		}
	}

	if w.symlinks > 0 && !s.config.FollowSymlinks {
		log.Info().Int("symlinks", w.symlinks).Msg("Skipped symlinks, set follow_symlinks to walk them")
	}

	c := newCollector(s.config, fsys)
	c.build(ctx, w.directories, w.files)

	return &pluginsdk.DiscoveryResult{
		Assets:     c.assets,
		Lineage:    c.lineage,
		Statistics: c.statistics,
	}
}

// address is the server as it is dialled, for logs and errors.
func (c *Config) address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// normaliseRoots cleans the configured roots so the same directory
// written two ways ("/data" and "/data/") walks once.
func normaliseRoots(roots []string) []string {
	cleaned := make([]string, 0, len(roots))
	seen := make(map[string]bool, len(roots))

	for _, root := range roots {
		root = cleanPath(strings.TrimSpace(root))
		if root == "" || seen[root] {
			continue
		}
		seen[root] = true
		cleaned = append(cleaned, root)
	}

	if len(cleaned) == 0 {
		return []string{"/"}
	}
	return cleaned
}

// assetMRN is the single place an SFTP MRN is built. The walk, the asset
// pass and the lineage pass all go through it, so they can never drift
// into addressing the same directory differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
