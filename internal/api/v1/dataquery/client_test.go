package dataquery

import (
	"net/http/httptest"
	"testing"

	"github.com/marmotdata/marmot/internal/core/dataquery"
)

func TestClientOfTellsTheCLIFromOtherAPICalls(t *testing.T) {
	cli := httptest.NewRequest("POST", "/api/v1/query/run", nil)
	cli.Header.Set("User-Agent", "marmot-cli")
	if got := clientOf(cli); got != dataquery.ClientCLI {
		t.Errorf("CLI request recorded as %q, want %q", got, dataquery.ClientCLI)
	}

	sdk := httptest.NewRequest("POST", "/api/v1/query/run", nil)
	sdk.Header.Set("User-Agent", "marmot-sdk-go")
	if got := clientOf(sdk); got != dataquery.ClientAPI {
		t.Errorf("SDK request recorded as %q, want %q", got, dataquery.ClientAPI)
	}
}

func TestSplitListDropsEmptyParts(t *testing.T) {
	got := splitList(" mcp, ,cli,")
	if len(got) != 2 || got[0] != "mcp" || got[1] != "cli" {
		t.Errorf("splitList = %q, want [mcp cli]", got)
	}
}
