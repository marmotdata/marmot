package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/mysql/mysql"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   mysql.Meta(),
		Source: &mysql.Source{},
	})
}
