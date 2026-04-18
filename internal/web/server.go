package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"

	"github.com/vsangava/distractions-free/internal/config"
)

//go:embed static/*
var webFiles embed.FS

// ConfigHandler is a testable handler that returns the current config as JSON.
func ConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cfg := config.GetConfig()
	json.NewEncoder(w).Encode(cfg)
}

// StaticFileHandler returns a handler for serving embedded static files.
func StaticFileHandler() (http.Handler, error) {
	fsys, err := fs.Sub(webFiles, "static")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(fsys)), nil
}

func StartWebServer() {
	staticHandler, err := StaticFileHandler()
	if err != nil {
		log.Fatalf("Failed to load embedded web files: %v", err)
	}

	http.Handle("/", staticHandler)
	http.HandleFunc("/api/config", ConfigHandler)

	log.Println("Web server starting on http://localhost:8040")
	if err := http.ListenAndServe("127.0.0.1:8040", nil); err != nil {
		log.Fatalf("Web server failed: %v", err)
	}
}
