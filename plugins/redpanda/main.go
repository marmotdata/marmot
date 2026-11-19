package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/redpanda/redpanda"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   redpanda.Meta(),
		Source: &redpanda.Source{},
	})
}
