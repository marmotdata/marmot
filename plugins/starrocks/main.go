package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/starrocks/starrocks"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   starrocks.Meta(),
		Source: &starrocks.Source{},
	})
}
