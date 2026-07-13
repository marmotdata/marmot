package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/gke/gke"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   gke.Meta(),
		Source: &gke.Source{},
	})
}
