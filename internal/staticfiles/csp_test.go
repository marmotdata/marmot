package staticfiles

import (
	"reflect"
	"testing"
)

func TestScriptHashMatchesTheCSPSpecExample(t *testing.T) {
	// MDN's worked example: the hash a browser computes for this inline script body.
	if got, want := ScriptHash([]byte("alert('Hello, world.');")), "'sha256-qznLcsROx4GACP2dm0UCKCzCG+HiZ1guq6ZZDob/Tng='"; got != want {
		t.Fatalf("ScriptHash = %s, want %s", got, want)
	}
}

func TestHashInlineScriptsHashesExactBodiesAndSkipsExternalScripts(t *testing.T) {
	html := []byte("<html><head><script>\n\tvar a = 1;\n</script><script src=\"/_app/x.js\"></script>" +
		"<SCRIPT type=\"module\">b()</SCRIPT><script>\n\tvar a = 1;\n</script></head></html>")
	got := HashInlineScripts(html)
	want := []string{ScriptHash([]byte("\n\tvar a = 1;\n")), ScriptHash([]byte("b()"))}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HashInlineScripts = %v, want %v", got, want)
	}
}
