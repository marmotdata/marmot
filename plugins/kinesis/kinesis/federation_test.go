package kinesis

import (
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

func federatedConfig(extra map[string]any) pluginsdk.RawConfig {
	creds := map[string]any{"role_arn": "arn:aws:iam::123456789012:role/marmot", "region": "eu-west-1"}
	for k, v := range extra {
		creds[k] = v
	}
	return pluginsdk.RawConfig{"credentials": creds}
}

// A role names the audience the host mints for; the config is otherwise
// untouched and no key is needed.
func TestValidateWithRoleARNFederates(t *testing.T) {
	out, err := (&Source{}).Validate(federatedConfig(nil))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got := pluginsdk.Audience(out); got != "sts.amazonaws.com" {
		t.Errorf("audience %q, want sts.amazonaws.com", got)
	}
}

func TestValidateRefusesRoleARNWithKeys(t *testing.T) {
	_, err := (&Source{}).Validate(federatedConfig(map[string]any{"id": "AKIA", "secret": "s"}))
	if err == nil || !strings.Contains(err.Error(), "role_arn excludes") {
		t.Fatalf("got %v, want a role_arn refusal", err)
	}
}

func TestValidateWithoutRoleDoesNotFederate(t *testing.T) {
	out, err := (&Source{}).Validate(pluginsdk.RawConfig{"credentials": map[string]any{"region": "eu-west-1"}})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if pluginsdk.Federated(out) {
		t.Errorf("config federates without a role: %v", out)
	}
}
