package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/trino/trino"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   trino.Meta(),
		Source: &trino.Source{},
	})
}
