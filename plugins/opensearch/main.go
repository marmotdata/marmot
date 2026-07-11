package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/opensearch/opensearch"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   opensearch.Meta(),
		Source: &opensearch.Source{},
	})
}
