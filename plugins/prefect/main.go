package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/prefect/prefect"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   prefect.Meta(),
		Source: &prefect.Source{},
	})
}
