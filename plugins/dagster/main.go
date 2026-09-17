package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/dagster/dagster"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   dagster.Meta(),
		Source: &dagster.Source{},
	})
}
