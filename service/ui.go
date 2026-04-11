package service

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
)

//go:embed ui.html
var uiHTML []byte

//go:embed assets/*
var assets embed.FS

//go:embed docs/*.html
var docsFS embed.FS

func HandleUI(res http.ResponseWriter, req *http.Request) {
	buf := bytes.NewBuffer(uiHTML)

	res.Header().Set("Content-Type", mimeTypeHTML)
	res.Header().Set("X-Content-Type-Options", "nosniff")
	res.WriteHeader(http.StatusOK)
	_, _ = buf.WriteTo(res)
}

func HandleAssets() http.Handler {
	return http.FileServer(http.FS(assets))
}

func HandleDocs() http.Handler {
	// Create a sub-filesystem rooted at "docs" directory
	docsSubFS, err := fs.Sub(docsFS, "docs")
	if err != nil {
		panic("missing docs")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle /docs/ or /docs (without trailing slash) → redirect to getting-started.html
		if r.URL.Path == "/" || r.URL.Path == "" {
			http.Redirect(w, r, "/docs/getting-started.html", http.StatusFound)
			return
		}

		// Serve the file from embedded FS (now rooted at docs/)
		http.FileServer(http.FS(docsSubFS)).ServeHTTP(w, r)
	})
}
