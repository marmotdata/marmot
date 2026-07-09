package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/nats/nats"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   nats.Meta(),
		Source: &nats.Source{},
	})
}
