package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/cassandra/cassandra"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   cassandra.Meta(),
		Source: &cassandra.Source{},
	})
}
