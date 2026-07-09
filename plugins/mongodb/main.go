package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/mongodb/mongodb"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   mongodb.Meta(),
		Source: &mongodb.Source{},
	})
}
