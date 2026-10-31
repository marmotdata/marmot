package assets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCreateExtras(t *testing.T) {
	base := CreateRequest{Name: "returns_forecast", Type: "spreadsheet", Providers: []string{"Google Drive"}}
	assert.Empty(t, validateCreateExtras(base))

	withExtras := base
	withExtras.Owners = []CreateOwner{{Type: "user", ID: "6f1c1f9e-2d3b-4a8e-9a55-0d3c8f1e2b47"}, {Type: "team", ID: "0b8f3c2a-1d4e-4f5a-8b6c-7d8e9f0a1b2c"}}
	withExtras.Terms = []string{"9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d"}
	assert.Empty(t, validateCreateExtras(withExtras))

	badKind := base
	badKind.Owners = []CreateOwner{{Type: "group", ID: "6f1c1f9e-2d3b-4a8e-9a55-0d3c8f1e2b47"}}
	assert.NotEmpty(t, validateCreateExtras(badKind))

	badOwner := base
	badOwner.Owners = []CreateOwner{{Type: "user", ID: "jane"}}
	assert.NotEmpty(t, validateCreateExtras(badOwner))

	badTerm := base
	badTerm.Terms = []string{"Net revenue"}
	assert.NotEmpty(t, validateCreateExtras(badTerm))

	noProvider := base
	noProvider.Providers = nil
	assert.NotEmpty(t, validateCreateExtras(noProvider))
}
