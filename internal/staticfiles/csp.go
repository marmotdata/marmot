package staticfiles

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
)

// inlineScript matches a <script> element with no src attribute and captures its exact body, which is what a browser hashes for a CSP 'sha256-' source.
var inlineScript = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)

var srcAttr = regexp.MustCompile(`(?i)\bsrc\s*=`)

// inlineScriptHashes holds the CSP hash sources for the inline scripts in the embedded index.html. It is set once by the production build and empty otherwise.
var inlineScriptHashes []string

// InlineScriptHashes returns the 'sha256-…' sources that allow the console's own inline scripts under a Content-Security-Policy, so the policy needs no 'unsafe-inline'.
func InlineScriptHashes() []string {
	return inlineScriptHashes
}

// HashInlineScripts returns a CSP 'sha256-…' source for each inline script in html, in document order and without duplicates. Scripts with a src attribute load from 'self' and need no hash.
func HashInlineScripts(html []byte) []string {
	var hashes []string
	seen := map[string]bool{}
	for _, m := range inlineScript.FindAllSubmatch(html, -1) {
		if srcAttr.Match(m[1]) {
			continue
		}
		h := ScriptHash(m[2])
		if !seen[h] {
			seen[h] = true
			hashes = append(hashes, h)
		}
	}
	return hashes
}

// ScriptHash returns the CSP source that allows exactly this inline script body.
func ScriptHash(body []byte) string {
	sum := sha256.Sum256(body)
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}
