package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/dbt/dbt"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   dbt.Meta(),
		Source: &dbt.Source{},
	})
}
