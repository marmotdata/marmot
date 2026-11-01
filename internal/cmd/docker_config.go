package cmd

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// registryUsername is paired with the access token when a container tool
// authenticates to a Marmot registry. The token is the whole credential.
const registryUsername = "oauth2accesstoken"

// Docker runs docker-credential-<name> for a registry mapped to <name>.
const (
	credentialHelperName   = "marmot"
	credentialHelperBinary = "docker-credential-" + credentialHelperName
)

// registryHost reduces a Marmot URL, or any spelling a container tool uses
// for a registry, to host[:port].
func registryHost(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	return s
}

func dockerConfigPath() (string, error) {
	if dir := os.Getenv("DOCKER_CONFIG"); dir != "" {
		return filepath.Join(dir, "config.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".docker", "config.json"), nil
}

// dockerConfig is ~/.docker/config.json decoded loosely, so keys this code
// does not know about survive a round trip.
type dockerConfig map[string]any

func loadDockerConfig(path string) (dockerConfig, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return dockerConfig{}, nil
	}
	if err != nil {
		return nil, err
	}
	cfg := dockerConfig{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return cfg, nil
}

func saveDockerConfig(path string, cfg dockerConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// section returns the named object, creating it if missing.
func (c dockerConfig) section(name string) map[string]any {
	if m, ok := c[name].(map[string]any); ok {
		return m
	}
	m := map[string]any{}
	c[name] = m
	return m
}

// drop removes key from the named object and the object itself once empty.
func (c dockerConfig) drop(name, key string) {
	m, ok := c[name].(map[string]any)
	if !ok {
		return
	}
	delete(m, key)
	if len(m) == 0 {
		delete(c, name)
	}
}

func (c dockerConfig) helperFor(registry string) string {
	m, _ := c["credHelpers"].(map[string]any)
	s, _ := m[registry].(string)
	return s
}

// registryAuth says how the token was handed to container tools.
type registryAuth struct {
	Registry string
	Path     string
	// Helper means the registry was mapped to docker-credential-marmot, which
	// reads the current token from the marmot credential store on every use.
	// Otherwise the token was written into the config like docker login does.
	Helper bool
	// CredsStore is the user's global credential store, which shadows a
	// static entry.
	CredsStore string
}

func (r registryAuth) describe() string {
	if r.Helper {
		return fmt.Sprintf("Pushes to %s will authenticate through %s.\n", r.Registry, credentialHelperBinary)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Token written to %s for pushes to %s.\n", r.Path, r.Registry)
	if r.CredsStore != "" {
		fmt.Fprintf(&b, "Warning: credsStore %q takes precedence over that entry, so pushes will not use it until %s is on your PATH.\n",
			r.CredsStore, credentialHelperBinary)
	}
	fmt.Fprintf(&b, "Tip: symlink the marmot binary as %s on your PATH so pushes always use the current token.\n",
		credentialHelperBinary)
	return b.String()
}

// configureRegistryAuth lets docker, crane, oras and anything else reading
// the Docker credential store push to the registry on the Marmot host.
//
// The helper is preferred because it never goes stale and it wins over a
// credsStore. It is only mapped when the binary is on PATH: a mapping to a
// missing helper breaks every docker command.
func configureRegistryAuth(hostURL, token string) (registryAuth, error) {
	path, err := dockerConfigPath()
	if err != nil {
		return registryAuth{}, err
	}
	cfg, err := loadDockerConfig(path)
	if err != nil {
		return registryAuth{}, err
	}

	r := registryAuth{Registry: registryHost(hostURL), Path: path}
	_, lookErr := exec.LookPath(credentialHelperBinary)
	r.Helper = lookErr == nil
	r.CredsStore, _ = cfg["credsStore"].(string)

	if r.Helper {
		cfg.section("credHelpers")[r.Registry] = credentialHelperName
		cfg.drop("auths", r.Registry)
	} else {
		encoded := base64.StdEncoding.EncodeToString([]byte(registryUsername + ":" + token))
		cfg.section("auths")[r.Registry] = map[string]any{"auth": encoded}
		if cfg.helperFor(r.Registry) == credentialHelperName {
			cfg.drop("credHelpers", r.Registry)
		}
	}

	if err := saveDockerConfig(path, cfg); err != nil {
		return registryAuth{}, fmt.Errorf("writing %s: %w", path, err)
	}
	return r, nil
}

// removeRegistryAuth undoes configureRegistryAuth. Entries written by
// something else, such as a mapping to another helper, are left alone.
func removeRegistryAuth(hostURL string) error {
	path, err := dockerConfigPath()
	if err != nil {
		return err
	}
	cfg, err := loadDockerConfig(path)
	if err != nil {
		return err
	}
	if len(cfg) == 0 {
		return nil
	}

	registry := registryHost(hostURL)
	cfg.drop("auths", registry)
	if cfg.helperFor(registry) == credentialHelperName {
		cfg.drop("credHelpers", registry)
	}
	return saveDockerConfig(path, cfg)
}
