package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/mariadb/mariadb"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   mariadb.Meta(),
		Source: &mariadb.Source{},
	})
}
