package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/gcs/gcs"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   gcs.Meta(),
		Source: &gcs.Source{},
	})
}
