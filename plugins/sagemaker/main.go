package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/sagemaker/sagemaker"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   sagemaker.Meta(),
		Source: &sagemaker.Source{},
	})
}
