package azureblob

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

const (
	testTenantID = "11111111-2222-3333-4444-555555555555"
	testClientID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
)

// Tenant and client ids with an account name federate; no key is involved.
func TestValidateWithEntraIdentityFederates(t *testing.T) {
	out, err := (&Source{}).Validate(pluginsdk.RawConfig{"account_name": "lake", "tenant_id": testTenantID, "client_id": testClientID})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got := pluginsdk.Audience(out); got != "api://AzureADTokenExchange" {
		t.Errorf("audience %q", got)
	}
}

func TestValidateRefusals(t *testing.T) {
	cases := map[string]pluginsdk.RawConfig{
		"key next to identity":               {"account_name": "lake", "account_key": "k", "tenant_id": testTenantID, "client_id": testClientID},
		"connection string next to identity": {"connection_string": "DefaultEndpointsProtocol=https;AccountName=lake;AccountKey=k", "tenant_id": testTenantID, "client_id": testClientID},
		"tenant without client":              {"account_name": "lake", "tenant_id": testTenantID},
		"account without any credential":     {"account_name": "lake"},
	}
	for name, raw := range cases {
		if _, err := (&Source{}).Validate(raw); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestValidateWithKeyDoesNotFederate(t *testing.T) {
	out, err := (&Source{}).Validate(pluginsdk.RawConfig{"account_name": "lake", "account_key": "k"})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if pluginsdk.Federated(out) {
		t.Errorf("config federates without an identity: %v", out)
	}
	if _, err := (&Source{}).Validate(pluginsdk.RawConfig{"connection_string": "DefaultEndpointsProtocol=https;AccountName=lake;AccountKey=k"}); err != nil {
		t.Fatalf("connection string: %v", err)
	}
}
