package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/grafana/grafana"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   grafana.Meta(),
		Source: &grafana.Source{},
	})
}
