package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/doris/doris"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   doris.Meta(),
		Source: &doris.Source{},
	})
}
