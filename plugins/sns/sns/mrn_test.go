package sns

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

func TestTopicMRNCarriesZone(t *testing.T) {
	cfg := &Config{}
	if got, want := cfg.AssetMRN("Topic", "SNS", "orders"), "mrn://asset/topic/sns/orders"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	cfg.BaseConfig = pluginsdk.BaseConfig{Zone: "prod"}
	if got, want := cfg.AssetMRN("Topic", "SNS", "orders"), "mrn://asset/topic/sns@prod/orders"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
