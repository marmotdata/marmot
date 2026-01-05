//go:build production

package staticfiles

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed all:build
var embeddedFiles embed.FS

func init() {
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".mjs", "application/javascript")
	mime.AddExtensionType(".css", "text/css")
	mime.AddExtensionType(".html", "text/html")
	mime.AddExtensionType(".json", "application/json")
	mime.AddExtensionType(".svg", "image/svg+xml")
	mime.AddExtensionType(".png", "image/png")
	mime.AddExtensionType(".ico", "image/x-icon")
	mime.AddExtensionType(".woff", "font/woff")
	mime.AddExtensionType(".woff2", "font/woff2")
}

func (h *Handler) SetupRoutes(mux *http.ServeMux) error {
	buildFS, err := fs.Sub(embeddedFiles, "build")
	if err != nil {
		return err
	}

	fsHandler := http.FileServer(http.FS(buildFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		_, err := buildFS.Open(path)
		if err != nil {
			// SPA fallback - serve index.html for client-side routing
			content, _ := fs.ReadFile(buildFS, "index.html")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Write(content)
			return
		}

		ext := filepath.Ext(path)
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			w.Header().Set("Content-Type", mimeType)
		}

		if strings.HasPrefix(path, "_app/immutable/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if path == "index.html" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}

		fsHandler.ServeHTTP(w, r)
	})
	return nil
}
