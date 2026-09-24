package domain_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/marmotdata/marmot/internal/core/domain"
)

// The aliases follow the UI's name for Unassigned; a new locale or a reworded
// label must be added to domain.UnassignedAliases too.
func TestUnassignedAliasesFollowTheUI(t *testing.T) {
	files, err := filepath.Glob("../../../web/marmot/messages/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no UI catalogues found: %v", err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var messages map[string]json.RawMessage
		if err := json.Unmarshal(data, &messages); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		var label string
		_ = json.Unmarshal(messages["domains_unassigned"], &label)
		if label == "" || !slices.Contains(domain.UnassignedAliases, label) {
			t.Errorf("%s: domains_unassigned %q is not in domain.UnassignedAliases", filepath.Base(file), label)
		}
	}
}
