package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/flink/flink"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   flink.Meta(),
		Source: &flink.Source{},
	})
}
