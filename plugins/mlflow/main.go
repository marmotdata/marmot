package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/mlflow/mlflow"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   mlflow.Meta(),
		Source: &mlflow.Source{},
	})
}
