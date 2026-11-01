package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/questdb/questdb"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   questdb.Meta(),
		Source: &questdb.Source{},
	})
}
