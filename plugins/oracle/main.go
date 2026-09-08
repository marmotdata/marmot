package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/oracle/oracle"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   oracle.Meta(),
		Source: &oracle.Source{},
	})
}
