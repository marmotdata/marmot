package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/unitycatalog/unitycatalog"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   unitycatalog.Meta(),
		Source: &unitycatalog.Source{},
	})
}
