// Command fakequerier is a minimal query-capable plugin used by the instance
// manager tests. It plans every statement as touching one table and streams
// back a row per requested count, echoing the caller's identity.
package main

import (
	"context"
	"fmt"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Name string `json:"name" validate:"required"`
}

type Source struct{}

func (s *Source) Validate(raw pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](raw)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Source) Discover(_ context.Context, _ pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	return &pluginsdk.DiscoveryResult{}, nil
}

func (s *Source) PlanQuery(_ context.Context, _ pluginsdk.RawConfig, _ pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	return &pluginsdk.QueryPlan{ReferencedMRNs: []string{"mrn://table/fake/orders"}}, nil
}

func (s *Source) ExecuteQuery(_ context.Context, _ pluginsdk.RawConfig, req pluginsdk.QueryRequest, emit func(pluginsdk.QueryResultChunk) error) error {
	if err := emit(pluginsdk.QueryResultChunk{
		Columns: []pluginsdk.QueryColumn{{Name: "id", Type: "integer"}, {Name: "who", Type: "varchar"}},
	}); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if err := emit(pluginsdk.QueryResultChunk{Rows: [][]any{{i, req.Identity}}}); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta: pluginsdk.Meta{
			ID:          "fakequerier",
			Name:        "Fake Querier",
			Description: "Test plugin for the query instance manager",
			Icon:        "fakequerier",
			Category:    "test",
			ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
		},
		Source: &Source{},
	})
}
