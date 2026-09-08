package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/bigtable/bigtable"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   bigtable.Meta(),
		Source: &bigtable.Source{},
	})
}
