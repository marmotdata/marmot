package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/pubsub/pubsub"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   pubsub.Meta(),
		Source: &pubsub.Source{},
	})
}
