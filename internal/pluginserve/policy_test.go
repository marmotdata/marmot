package pluginserve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

func writePolicy(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestLoadPolicyRefusesUnknownKeys(t *testing.T) {
	for name, body := range map[string]string{
		"unknown top-level key":   "sources: {}\nsourcse: {}\n",
		"unknown source key":      "sources:\n  warehouse:\n    max_row: 10\n",
		"negative max_rows":       "sources:\n  warehouse:\n    max_rows: -1\n",
		"empty actor pattern":     "sources:\n  warehouse:\n    actors: [\"\"]\n",
		"wrong type for query":    "sources:\n  warehouse:\n    query: sometimes\n",
		"not a mapping of rules":  "sources:\n  - warehouse\n",
		"empty origin pattern":    "sources:\n  warehouse:\n    origins: [\"\"]\n",
		"malformed manifest pin":  "sources:\n  warehouse:\n    manifest: sha256:abc\n",
		"unknown tool key":        "sources:\n  warehouse:\n    tools:\n      scale:\n        actor: [\"x\"]\n",
		"unknown param rule":      "sources:\n  warehouse:\n    tools:\n      scale:\n        params:\n          replicas: {maximum: 3}\n",
		"param rule with nothing": "sources:\n  warehouse:\n    tools:\n      scale:\n        params:\n          replicas: {}\n",
		"param min above max":     "sources:\n  warehouse:\n    tools:\n      scale:\n        params:\n          replicas: {min: 5, max: 1}\n",
		"param range and values":  "sources:\n  warehouse:\n    tools:\n      scale:\n        params:\n          replicas: {min: 1, values: [2]}\n",
		"param value not scalar":  "sources:\n  warehouse:\n    tools:\n      scale:\n        params:\n          namespace: {values: [[a]]}\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadPolicy(writePolicy(t, body))
			assert.Error(t, err)
		})
	}
}

func TestPolicyDecides(t *testing.T) {
	p, err := LoadPolicy(writePolicy(t, `
sources:
  warehouse:
    max_rows: 50
    actors: ["user:alice", "task:*"]
  archive:
    query: false
  open: {}
`))
	require.NoError(t, err)
	assert.Equal(t, "policy.yaml", p.Name())

	query := func(source, actor string, maxRows int64) *pb.Query {
		return &pb.Query{Source: source, Actor: actor, MaxRows: maxRows}
	}
	tests := []struct {
		name    string
		q       *pb.Query
		allowed bool
		maxRows int64
	}{
		{"listed actor", query("warehouse", "user:alice", 10), true, 10},
		{"actor glob", query("warehouse", "task:nightly/report", 10), true, 10},
		{"actor outside the allow-list", query("warehouse", "user:mallory", 10), false, 0},
		{"glob does not match a prefix of another kind", query("warehouse", "user:task:x", 10), false, 0},
		{"row limit above the ceiling is lowered", query("warehouse", "user:alice", 5001), true, 50},
		{"queries switched off", query("archive", "user:alice", 10), false, 0},
		{"empty rules allow any actor", query("open", "user:anyone", 10), true, 10},
		{"a source the policy omits is refused", query("unlisted", "user:alice", 10), false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := p.decide(tt.q)
			assert.Equal(t, tt.allowed, d.allowed, d.reason)
			if tt.allowed {
				assert.Equal(t, tt.maxRows, d.maxRows)
			} else {
				assert.NotEmpty(t, d.reason)
			}
		})
	}
}

func TestNoPolicyAllowsEverything(t *testing.T) {
	var p *Policy
	d := p.decide(&pb.Query{Source: "anything", Actor: "user:anyone", MaxRows: 7})
	assert.True(t, d.allowed)
	assert.Equal(t, int64(7), d.maxRows)
}

func TestEmptyPolicyDeniesEverything(t *testing.T) {
	p, err := LoadPolicy(writePolicy(t, ""))
	require.NoError(t, err)
	assert.False(t, p.decide(&pb.Query{Source: "warehouse"}).allowed)
}

func TestGlobMatch(t *testing.T) {
	for _, tt := range []struct {
		pattern, s string
		want       bool
	}{
		{"user:alice", "user:alice", true},
		{"user:alice", "user:alice2", false},
		{"task:*", "task:", true},
		{"task:*", "task:a/b", true},
		{"*", "", true},
		{"*:bot", "service_account:bot", true},
		{"a*b*c", "abbbc", true},
		{"a*b*c", "acb", false},
		{"ab*ba", "aba", false},
		{"user:[a]", "user:a", false},
	} {
		assert.Equal(t, tt.want, globMatch(tt.pattern, tt.s), "%q ~ %q", tt.pattern, tt.s)
	}
}

func TestPolicyCheckManifest(t *testing.T) {
	specs := testToolManifest()
	for name, tc := range map[string]struct {
		rules string
		err   string
	}{
		"fits":                    {rules: "scale:\n        params:\n          namespace: {values: [payments]}\n          replicas: {min: 1, max: 10}\n"},
		"unknown tool":            {rules: "delete_everything: {}\n", err: "not in the plugin's manifest"},
		"unknown parameter":       {rules: "scale:\n        params:\n          replica: {max: 3}\n", err: "has no parameter"},
		"range on a string":       {rules: "scale:\n        params:\n          namespace: {max: 3}\n", err: "apply to int parameters"},
		"string value for an int": {rules: "scale:\n        params:\n          replicas: {values: [three]}\n", err: "does not suit"},
	} {
		t.Run(name, func(t *testing.T) {
			p, err := LoadPolicy(writePolicy(t, "sources:\n  prod:\n    tools:\n      "+tc.rules))
			require.NoError(t, err)
			err = p.CheckManifest("prod", specs)
			if tc.err == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.err)
		})
	}
}

func TestToolPolicyNeedsTheFlagAndAPolicy(t *testing.T) {
	p, err := LoadPolicy(writePolicy(t, "sources:\n  prod:\n    tools:\n      scale: {}\n"))
	require.NoError(t, err)

	_, _, err = p.toolPolicy(false, "prod", "scale")
	assert.ErrorIs(t, err, errToolsOff, "a policy naming the tool is not enough without --allow-tools")
	_, _, err = (*Policy)(nil).toolPolicy(true, "prod", "scale")
	assert.ErrorIs(t, err, errToolsWithoutPolicy, "--allow-tools is not enough without a policy")
	_, _, err = p.toolPolicy(true, "prod", "rollout_restart")
	assert.ErrorContains(t, err, "is not allowed")
	_, _, err = p.toolPolicy(true, "staging", "scale")
	assert.ErrorContains(t, err, "is not in the policy")
	_, _, err = p.toolPolicy(true, "prod", "scale")
	assert.NoError(t, err)
}

func TestParamPolicyAllows(t *testing.T) {
	one, ten := int64(1), int64(10)
	replicas := ParamPolicy{Min: &one, Max: &ten}
	assert.NoError(t, replicas.allows("replicas", map[string]any{"replicas": float64(3)}))
	assert.Error(t, replicas.allows("replicas", map[string]any{"replicas": float64(11)}))
	assert.Error(t, replicas.allows("replicas", map[string]any{"replicas": 2.5}), "a fraction is not a whole number")
	assert.ErrorContains(t, replicas.allows("replicas", map[string]any{}), "must be given")

	namespace := ParamPolicy{Values: []any{"payments", "checkout"}}
	assert.NoError(t, namespace.allows("namespace", map[string]any{"namespace": "checkout"}))
	assert.Error(t, namespace.allows("namespace", map[string]any{"namespace": "kube-system"}))

	exact := ParamPolicy{Values: []any{3}}
	assert.NoError(t, exact.allows("replicas", map[string]any{"replicas": float64(3)}), "an int value matches after a JSON round trip")
	assert.Error(t, exact.allows("replicas", map[string]any{"replicas": "3"}), "a string never matches an int value")
}
