package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/firehose/firehose"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   firehose.Meta(),
		Source: &firehose.Source{},
	})
}
