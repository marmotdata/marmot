package plugin

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

func TestSupportsMarmot(t *testing.T) {
	cases := []struct {
		constraint, host string
		want             bool
		wantErr          bool
	}{
		{"", "0.10.0", true, false},
		{">=0.14.0", "0.14.0", true, false},
		{">=0.14.0", "v0.15.2", true, false},
		{">=0.14.0", "0.13.9", false, false},
		{">=0.14.0", "0.14.0-rc.1", true, false},
		{">=0.14.0", "dev", true, false},
		{"not a constraint", "0.14.0", false, true},
	}
	for _, tc := range cases {
		got, err := SupportsMarmot(tc.constraint, tc.host)
		if (err != nil) != tc.wantErr {
			t.Fatalf("SupportsMarmot(%q, %q) error = %v", tc.constraint, tc.host, err)
		}
		if got != tc.want {
			t.Errorf("SupportsMarmot(%q, %q) = %v, want %v", tc.constraint, tc.host, got, tc.want)
		}
	}
}

func TestCheckCompatibility(t *testing.T) {
	SetHostVersion("0.14.0")
	t.Cleanup(func() { SetHostVersion("dev") })

	if err := checkCompatibility(&pluginsdk.Meta{ID: "old"}); err != nil {
		t.Errorf("plugin without compatibility fields refused: %v", err)
	}
	if err := checkCompatibility(&pluginsdk.Meta{ID: "new", MarmotVersion: ">=0.15.0"}); err == nil {
		t.Error("plugin needing a newer Marmot was accepted")
	}
	if err := checkCompatibility(&pluginsdk.Meta{ID: "proto", ProtocolVersion: pluginsdk.ProtocolVersion + 1}); err == nil {
		t.Error("plugin on another protocol was accepted")
	}
}
