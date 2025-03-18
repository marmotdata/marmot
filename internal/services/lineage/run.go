package lineage

import (
	"encoding/json"
)

type RunEvent struct {
	EventType string          `json:"eventType"`
	EventTime string          `json:"eventTime"`
	Run       Run             `json:"run"`
	Job       Job             `json:"job"`
	Inputs    []InputDataset  `json:"inputs"`
	Outputs   []OutputDataset `json:"outputs"`
	Producer  string          `json:"producer"`
	SchemaURL string          `json:"schemaURL"`
}

type Run struct {
	RunID  string                     `json:"runId"`
	Facets map[string]json.RawMessage `json:"facets,omitempty"`
}
