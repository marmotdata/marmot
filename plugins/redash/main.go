package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/redash/redash"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   redash.Meta(),
		Source: &redash.Source{},
	})
}
