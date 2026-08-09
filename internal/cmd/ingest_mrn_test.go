package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The CLI is the only thing that can tell the server what identity a
// plugin assigned. If this field stops being sent, identity silently
// falls back to the display name and distinct objects merge.

func TestCreateAssetRequest_SendsTheMRN(t *testing.T) {
	encoded, err := json.Marshal(CreateAssetRequest{
		Name: "orders",
		MRN:  "mrn://table/postgresql/public.orders",
		Type: "Table",
	})
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(encoded, &out))
	assert.Equal(t, "mrn://table/postgresql/public.orders", out["mrn"])
	assert.Equal(t, "orders", out["name"])
}

func TestCreateAssetRequest_OmitsAnEmptyMRN(t *testing.T) {
	encoded, err := json.Marshal(CreateAssetRequest{Name: "orders", Type: "Table"})
	require.NoError(t, err)

	assert.NotContains(t, string(encoded), `"mrn"`)
}
