package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/sqlite/sqlite"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   sqlite.Meta(),
		Source: &sqlite.Source{},
	})
}
