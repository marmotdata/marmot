package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/eks/eks"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   eks.Meta(),
		Source: &eks.Source{},
	})
}
