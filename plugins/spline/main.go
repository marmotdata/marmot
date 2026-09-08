package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/spline/spline"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   spline.Meta(),
		Source: &spline.Source{},
	})
}
