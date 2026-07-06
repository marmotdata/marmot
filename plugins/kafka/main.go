package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/kafka/kafka"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   kafka.Meta(),
		Source: &kafka.Source{},
	})
}
