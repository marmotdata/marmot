package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/athena/athena"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   athena.Meta(),
		Source: &athena.Source{},
	})
}
