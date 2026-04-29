package auth

import (
	"html/template"
	"net/http"
)

var authCallbackTmpl = template.Must(template.New("callback").Parse(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Signing in…</title></head>
<body>
<noscript>JavaScript is required to complete sign in.</noscript>
<script>
(function() {
	var root = "{{.RootURL}}";
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
</script>
</body>
</html>`))

func (h *Handler) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cache-Control", "no-store")
	_ = authCallbackTmpl.Execute(w, map[string]string{
		"RootURL": h.config.Server.RootURL,
	})
}
