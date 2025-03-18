//go:build production

package staticfiles

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:build
var embeddedFiles embed.FS

func (h *Handler) SetupRoutes(mux *http.ServeMux) error {
	buildFS, err := fs.Sub(embeddedFiles, "build")
	if err != nil {
		return err
	}

	fsHandler := http.FileServer(http.FS(buildFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Content-Type", "application/javascript")
		}

		_, err := buildFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil {
			indexFile, _ := buildFS.Open("index.html")
			content, _ := fs.ReadFile(buildFS, "index.html")
			w.Write(content)
			indexFile.Close()
			return
		}

		fsHandler.ServeHTTP(w, r)
	})
	return nil
}
