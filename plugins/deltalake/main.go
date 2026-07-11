package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/deltalake/deltalake"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   deltalake.Meta(),
		Source: &deltalake.Source{},
	})
}
