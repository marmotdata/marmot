package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/kubernetes/kubernetes"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   kubernetes.Meta(),
		Source: &kubernetes.Source{},
	})
}
