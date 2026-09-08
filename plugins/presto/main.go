package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/presto/presto"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   presto.Meta(),
		Source: &presto.Source{},
	})
}
