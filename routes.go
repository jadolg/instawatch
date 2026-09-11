package main

import "net/http"

func setupRoutes(tmpDir string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFiles)))
	mux.HandleFunc("GET /sw.js", handleServiceWorker)
	mux.HandleFunc("GET "+videoRoute+"{hash}", handleVideo)
	mux.HandleFunc("GET /download/{hash}", handleDownload)
	mux.HandleFunc("GET /description/{hash}", handleDescription)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		handleRoot(w, r, tmpDir)
	})
	return mux
}
