package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/marmotdata/marmot/internal/plugin"
)

func TestMaskedConfigDropsConfigForUnknownPlugin(t *testing.T) {
	plugin.GetLoadState().MarkReady()

	got := maskedConfig("no-such-plugin", map[string]interface{}{"password": "s3cret"})

	assert.Nil(t, got, "a config that cannot be masked must not be stored")
}
