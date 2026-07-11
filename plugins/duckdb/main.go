package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/duckdb/duckdb"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   duckdb.Meta(),
		Source: &duckdb.Source{},
	})
}
