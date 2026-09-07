package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/cockroachdb/cockroachdb"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   cockroachdb.Meta(),
		Source: &cockroachdb.Source{},
	})
}
