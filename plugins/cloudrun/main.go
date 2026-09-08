package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/cloudrun/cloudrun"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   cloudrun.Meta(),
		Source: &cloudrun.Source{},
	})
}
