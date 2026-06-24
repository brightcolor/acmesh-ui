// Package ui embeds the web frontend into the binary and serves it. No assets
// are loaded from a CDN at runtime; everything ships inside the binary.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:web
var embedded embed.FS

// FS returns the embedded web filesystem rooted at the web/ directory.
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "web")
	if err != nil {
		panic(err)
	}
	return sub
}

// Handler serves the embedded SPA. Unknown non-asset paths fall back to
// index.html so client-side routing works.
func Handler() http.Handler {
	content := FS()
	fileServer := http.FileServer(http.FS(content))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve the file if it exists; otherwise fall back to index.html.
		path := r.URL.Path
		if path == "/" {
			serveIndex(w, r, content)
			return
		}
		if _, err := fs.Stat(content, trimLeadingSlash(path)); err != nil {
			serveIndex(w, r, content)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, content fs.FS) {
	data, err := fs.ReadFile(content, "index.html")
	if err != nil {
		http.Error(w, "index.html missing from build", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func trimLeadingSlash(p string) string {
	if len(p) > 0 && p[0] == '/' {
		return p[1:]
	}
	return p
}
