package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/airflow/airflow"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   airflow.Meta(),
		Source: &airflow.Source{},
	})
}
