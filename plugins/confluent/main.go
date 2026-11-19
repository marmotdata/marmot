package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/confluent/confluent"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   confluent.Meta(),
		Source: &confluent.Source{},
	})
}
