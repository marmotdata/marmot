package cloudrun

import (
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

const testProvider = "projects/123456789/locations/global/workloadIdentityPools/marmot/providers/marmot"

// A provider names the audience the host mints for; no key is involved.
func TestValidateWithProviderFederates(t *testing.T) {
	s := &Source{}
	out, err := s.Validate(pluginsdk.RawConfig{"project_id": "p", "workload_identity_provider": "//iam.googleapis.com/" + testProvider})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got := pluginsdk.Audience(out); got != "https://iam.googleapis.com/"+testProvider {
		t.Errorf("audience %q", got)
	}
	if s.config.WorkloadIdentityProvider != testProvider {
		t.Errorf("provider %q, want it bare", s.config.WorkloadIdentityProvider)
	}
}

func TestValidateRefusesProviderWithKey(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"project_id": "p", "workload_identity_provider": testProvider, "credentials_json": "{}"})
	if err == nil || !strings.Contains(err.Error(), "excludes credentials_json") {
		t.Fatalf("got %v, want a refusal", err)
	}
}

func TestValidateWithKeyDoesNotFederate(t *testing.T) {
	out, err := (&Source{}).Validate(pluginsdk.RawConfig{"project_id": "p", "credentials_json": "{}"})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if pluginsdk.Federated(out) {
		t.Errorf("config federates without a provider: %v", out)
	}
}
