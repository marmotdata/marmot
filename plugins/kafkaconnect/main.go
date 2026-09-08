package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/kafkaconnect/kafkaconnect"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   kafkaconnect.Meta(),
		Source: &kafkaconnect.Source{},
	})
}
