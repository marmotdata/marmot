package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/postgresql/postgresql"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   postgresql.Meta(),
		Source: &postgresql.Source{},
	})
}
