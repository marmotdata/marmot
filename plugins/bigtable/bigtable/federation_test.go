package bigtable

import (
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

const testProvider = "projects/123456789/locations/global/workloadIdentityPools/marmot/providers/marmot"

// A provider under credentials names the audience the host mints for.
func TestValidateWithProviderFederates(t *testing.T) {
	out, err := (&Source{}).Validate(pluginsdk.RawConfig{"project_id": "p", "credentials": map[string]any{"workload_identity_provider": testProvider}})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got := pluginsdk.Audience(out); got != "https://iam.googleapis.com/"+testProvider {
		t.Errorf("audience %q", got)
	}
}

func TestValidateRefusesProviderWithKey(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"project_id": "p", "credentials": map[string]any{"workload_identity_provider": testProvider, "credentials_json": "{}"}})
	if err == nil || !strings.Contains(err.Error(), "excludes credentials_json") {
		t.Fatalf("got %v, want a refusal", err)
	}
}

func TestValidateWithKeyDoesNotFederate(t *testing.T) {
	out, err := (&Source{}).Validate(pluginsdk.RawConfig{"project_id": "p", "credentials": map[string]any{"credentials_json": "{}"}})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if pluginsdk.Federated(out) {
		t.Errorf("config federates without a provider: %v", out)
	}
}
