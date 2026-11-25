// Command fakeactor is a minimal action-capable plugin used by the ActionHost tests, after the plugin SDK's plugintest/testdata/actionplugin. It keeps a counter in the file its config names, since a test needs to see from outside whether an action ran.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	StateFile string `json:"state_file" validate:"required"`
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

func (s *Source) Discover(context.Context, pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	return &pluginsdk.DiscoveryResult{}, nil
}

func (s *Source) Actions() []pluginsdk.ActionSpec {
	return []pluginsdk.ActionSpec{{
		Name:          "set_count",
		Version:       1,
		Title:         "Set count",
		Description:   "Sets the counter",
		Targets:       []pluginsdk.TargetKind{"test/counter"},
		Reversibility: pluginsdk.ReversibilitySelfHealing,
		Risk:          pluginsdk.RiskBounded,
		Params: []pluginsdk.ActionParam{
			{Name: "count", Description: "New value", Type: pluginsdk.ParamInt, Required: true, Min: 0, Max: 10},
		},
	}}
}

func (s *Source) PlanAction(_ context.Context, raw pluginsdk.RawConfig, req pluginsdk.ActionRequest) (*pluginsdk.ActionPlan, error) {
	count, _ := req.IntParam("count")
	return &pluginsdk.ActionPlan{Target: "counter", TargetKind: "test/counter", Summary: fmt.Sprintf("set to %d", count), Precondition: "v1"}, nil
}

func (s *Source) ExecuteAction(_ context.Context, raw pluginsdk.RawConfig, req pluginsdk.ActionRequest) (*pluginsdk.ActionResult, error) {
	count, _ := req.IntParam("count")
	if err := os.WriteFile(raw["state_file"].(string), []byte(strconv.FormatInt(count, 10)), 0o600); err != nil {
		return nil, err
	}
	return &pluginsdk.ActionResult{Target: "counter", TargetKind: "test/counter", Summary: fmt.Sprintf("set to %d", count)}, nil
}

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta: pluginsdk.Meta{
			ID:          "fakeactor",
			Name:        "Fake Actor",
			Description: "Test plugin for the action host",
			Icon:        "fakeactor",
			Category:    "test",
			ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
		},
		Source: &Source{},
	})
}
