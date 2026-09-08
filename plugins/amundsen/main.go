package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/amundsen/amundsen"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   amundsen.Meta(),
		Source: &amundsen.Source{},
	})
}
