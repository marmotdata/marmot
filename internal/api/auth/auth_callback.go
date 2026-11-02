package auth

import (
	"html/template"
	"net/http"

	"github.com/marmotdata/marmot/internal/staticfiles"
)

// authCallbackScript is constant so the page's Content-Security-Policy can allow it by hash; the root URL it redirects to comes from the page's data-root attribute rather than being templated into the script.
const authCallbackScript = `
(function() {
	var root = document.documentElement.getAttribute("data-root") || "";
	fetch("/auth/exchange", {
		method: "POST",
		credentials: "same-origin"
	}).then(function(r) {
		if (!r.ok) { throw new Error("exchange failed"); }
		return r.json();
	}).then(function(data) {
		if (!data.access_token) { throw new Error("no token"); }
		localStorage.setItem("jwt", data.access_token);
		return fetch("/oauth/authorize/pending", { credentials: "same-origin" }).then(function(p) {
			if (p.ok) {
				window.location.replace(root + "/login?oauth_pending=1");
			} else {
				window.location.replace(root + "/");
			}
		});
	}).catch(function() {
		window.location.replace(root + "/login?error=Authentication%20failed");
	});
})();
`

// authCallbackCSP lets the page run only its own script and call Marmot, nothing else.
var authCallbackCSP = "default-src 'none'; script-src " + staticfiles.ScriptHash([]byte(authCallbackScript)) + "; connect-src 'self'; img-src 'self'; base-uri 'none'; frame-ancestors 'none'"

var authCallbackTmpl = template.Must(template.New("callback").Parse(`<!DOCTYPE html>
<html data-root="{{.RootURL}}">
<head><meta charset="utf-8"><title>Signing in…</title></head>
<body>
<noscript>JavaScript is required to complete sign in.</noscript>
<script>` + authCallbackScript + `</script>
</body>
</html>`))

func (h *Handler) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", authCallbackCSP)
	_ = authCallbackTmpl.Execute(w, map[string]string{
		"RootURL": h.config.Server.RootURL,
	})
}
