package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/hive/hive"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   hive.Meta(),
		Source: &hive.Source{},
	})
}
