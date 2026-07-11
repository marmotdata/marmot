package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/clickhouse/clickhouse"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   clickhouse.Meta(),
		Source: &clickhouse.Source{},
	})
}
