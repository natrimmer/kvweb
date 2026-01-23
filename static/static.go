package static

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist/*
var content embed.FS

// Handler returns an http.Handler that serves the embedded static files
func Handler() http.Handler {
	// Strip the "dist" prefix so files are served from root
	dist, err := fs.Sub(content, "dist")
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file
		// If it doesn't exist, serve index.html for SPA routing
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if file exists
		if _, err := fs.Stat(dist, path[1:]); err != nil {
			// File doesn't exist, serve index.html for SPA
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})
}
