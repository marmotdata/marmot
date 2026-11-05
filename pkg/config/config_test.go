package config

import (
	"os"
	"testing"
)

// Load runs once per process, so this must stay the only test calling it.
func TestLoad_DCRAllowedRedirectHostsFromEnv(t *testing.T) {
	t.Setenv("MARMOT_AUTH_DCR_ALLOWED_REDIRECT_HOSTS", "claude.ai,example.com:8443")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	got := cfg.Auth.DCR.AllowedRedirectHosts
	if len(got) != 2 || got[0] != "claude.ai" || got[1] != "example.com:8443" {
		t.Fatalf("unexpected allowlist from env: %v", got)
	}
}

func validBaseConfig() *Config {
	cfg := &Config{}
	cfg.Server.Port = 8080
	cfg.Database.Port = 5432
	cfg.Logging.Level = "info"
	cfg.Logging.Format = "json"
	cfg.Pipelines.MaxWorkers = 1
	cfg.Pipelines.SchedulerInterval = 1
	cfg.Pipelines.LeaseExpiry = 1
	cfg.Pipelines.ClaimExpiry = 1
	return cfg
}

func TestValidate_DCRAllowedRedirectHosts(t *testing.T) {
	valid := [][]string{
		nil,
		{"claude.ai"},
		{"claude.ai:8443"},
		{"claude.ai", "example.com"},
	}
	for _, hosts := range valid {
		cfg := validBaseConfig()
		cfg.Auth.DCR.AllowedRedirectHosts = hosts
		if err := validate(cfg); err != nil {
			t.Fatalf("expected %v to be valid, got %v", hosts, err)
		}
	}

	invalid := [][]string{
		{""},
		{"   "},
		{"https://claude.ai"},
		{"claude.ai/path"},
		{"*.claude.ai"},
		{"claude.ai:"},
		{"claude.ai:x"},
		{"claude ai"},
		{"@claude.ai"},
		{"claude.ai", "https://example.com"},
	}
	for _, hosts := range invalid {
		cfg := validBaseConfig()
		cfg.Auth.DCR.AllowedRedirectHosts = hosts
		if err := validate(cfg); err == nil {
			t.Fatalf("expected %v to be rejected", hosts)
		}
	}
}

func TestValidate_DCRAllowedRedirectHostsNormalised(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Auth.DCR.AllowedRedirectHosts = []string{" Claude.AI ", "EXAMPLE.com:8443"}
	if err := validate(cfg); err != nil {
		t.Fatalf("validate: %v", err)
	}
	got := cfg.Auth.DCR.AllowedRedirectHosts
	if got[0] != "claude.ai" || got[1] != "example.com:8443" {
		t.Fatalf("expected trimmed lowercase entries, got %v", got)
	}
}

// Config loading is not strict, so files written before server.tls.insecure_skip_verify was removed still load.
func TestLoadConfig_IgnoresRemovedServerTLSSkipVerify(t *testing.T) {
	saved := config
	t.Cleanup(func() { config = saved })

	path := t.TempDir() + "/config.yaml"
	body := "server:\n  tls:\n    cert_path: /certs/tls.crt\n    key_path: /certs/tls.key\n    insecure_skip_verify: true\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := loadConfig(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	if config.Server.TLS == nil || config.Server.TLS.CertPath != "/certs/tls.crt" || config.Server.TLS.KeyPath != "/certs/tls.key" {
		t.Fatalf("unexpected server tls: %+v", config.Server.TLS)
	}
}
