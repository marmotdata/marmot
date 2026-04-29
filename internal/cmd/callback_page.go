package cmd

import (
	_ "embed"
	"html/template"
	"net/http"
)

//go:embed marmot.svg
var marmotLogoSVG string

type callbackPageData struct {
	Title    string
	Message  string
	Logo     template.HTML
	IsError  bool
}

var callbackPageTmpl = template.Must(template.New("cb").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}} — Marmot</title>
<style>
*,*::before,*::after { box-sizing: border-box; }
html,body { margin: 0; padding: 0; min-height: 100%; }
body {
	font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
	background: #fefcfb;
	color: #111827;
	-webkit-font-smoothing: antialiased;
	display: flex;
	align-items: center;
	justify-content: center;
	min-height: 100vh;
	padding: 1rem;
}
.card {
	background: #ffffff;
	border: 1px solid #e5e7eb;
	border-radius: 0.75rem;
	box-shadow: 0 10px 25px -5px rgba(0,0,0,0.08), 0 4px 10px -3px rgba(0,0,0,0.04);
	padding: 2.5rem 2rem;
	text-align: center;
	max-width: 28rem;
	width: 100%;
}
.logo { display: block; margin: 0 auto 1.25rem; width: 4rem; height: 4rem; }
.logo svg { width: 100%; height: 100%; display: block; }
.badge {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 3rem;
	height: 3rem;
	border-radius: 9999px;
	background: rgba(139, 168, 139, 0.18);
	margin-bottom: 1rem;
}
.badge svg { width: 1.5rem; height: 1.5rem; color: #4a674a; }
.badge.error { background: rgba(199, 70, 36, 0.15); }
.badge.error svg { color: #c74624; }
h1 {
	margin: 0 0 0.5rem;
	font-size: 1.375rem;
	font-weight: 700;
	color: #111827;
	letter-spacing: -0.01em;
}
p { margin: 0; color: #6b7280; font-size: 0.95rem; line-height: 1.55; }
@media (prefers-color-scheme: dark) {
	body { background: #0f1419; color: #f3f4f6; }
	.card { background: #1f2937; border-color: #374151; box-shadow: 0 10px 25px -5px rgba(0,0,0,0.5), 0 4px 10px -3px rgba(0,0,0,0.3); }
	.badge { background: rgba(139, 168, 139, 0.18); }
	.badge svg { color: #8ba88b; }
	.badge.error { background: rgba(255, 138, 102, 0.15); }
	.badge.error svg { color: #ff8a66; }
	h1 { color: #f3f4f6; }
	p { color: #9ca3af; }
}
</style>
</head>
<body>
<main class="card">
	<div class="logo" aria-hidden="true">{{.Logo}}</div>
	{{if .IsError}}
	<div class="badge error">
		<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
			<line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
		</svg>
	</div>
	{{else}}
	<div class="badge">
		<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
			<polyline points="20 6 9 17 4 12"/>
		</svg>
	</div>
	{{end}}
	<h1>{{.Title}}</h1>
	<p>{{.Message}}</p>
</main>
</body>
</html>`))

func writeCallbackPage(w http.ResponseWriter, data callbackPageData) {
	data.Logo = template.HTML(marmotLogoSVG) // #nosec G203 -- compile-time embedded asset
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = callbackPageTmpl.Execute(w, data)
}
