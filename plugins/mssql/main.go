package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/mssql/mssql"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   mssql.Meta(),
		Source: &mssql.Source{},
	})
}
