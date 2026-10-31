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

func TestClientOfTellsTheEditorFromOtherAPICalls(t *testing.T) {
	editor := httptest.NewRequest("POST", "/api/v1/query/run", nil)
	editor.Header.Set("X-Marmot-Client", "editor")
	if got := clientOf(editor); got != dataquery.ClientEditor {
		t.Errorf("editor request recorded as %q, want %q", got, dataquery.ClientEditor)
	}
}

func TestExportRowNamesTheOutcomeAndGuardsFormulas(t *testing.T) {
	sa := "svc-support"
	row := exportRow(&dataquery.Query{
		ID: "q1", AuditSubject: "service_account:svc-support", PrincipalType: "service_account", ServiceAccount: &sa,
		Client: "mcp", SourceName: "warehouse", Statement: "=HYPERLINK(1)", Decision: dataquery.DecisionDenied,
		Status: dataquery.StatusDenied, OutcomeReason: dataquery.ReasonPermission,
		DecisionDetail: map[string]any{"reason": "querying data needs the assets:query permission"},
		ReferencedMRNs: []string{"mrn://source/warehouse", "mrn://table/postgresql/orders"},
	})
	if len(row) != len(exportColumns) {
		t.Fatalf("row has %d cells, want %d", len(row), len(exportColumns))
	}
	cell := func(name string) string {
		for i, c := range exportColumns {
			if c == name {
				return row[i]
			}
		}
		t.Fatalf("no column %q", name)
		return ""
	}
	if got := cell("outcome"); got != "denied" {
		t.Errorf("outcome = %q", got)
	}
	if got := cell("caller"); got != "svc-support" {
		t.Errorf("caller = %q", got)
	}
	if got := cell("tables"); got != "mrn://table/postgresql/orders" {
		t.Errorf("tables = %q, the whole-source resource is not a table", got)
	}
	if got := cell("statement"); got != "'=HYPERLINK(1)" {
		t.Errorf("statement = %q, a formula must not be live in a spreadsheet", got)
	}
}
