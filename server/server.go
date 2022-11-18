package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jordan-wright/unindexed"
	"zo-backend/server/api/v1"
)

// Root is a sample handler
func Root(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "root page")
	fmt.Fprintf(w, "use endpoint /api/v1/... to get some information from SendPulse")
}

// NewRouter returns a new HTTP handler that implements the main server routes
func NewRouter() (string, http.Handler, error) {
	router := chi.NewRouter()

	// Set up our middleware with sane defaults
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Compress(5, "gzip"))
	router.Use(middleware.Timeout(30 * time.Second))

	// Set up our root handlers
	router.Get("/", Root)
	// Set up our API
	r, err := v1.NewRouter()
	router.Mount("/api/v1/", r.Handler)

	// Set up static file serving
	staticPath, _ := filepath.Abs("../../static/")
	fs := http.FileServer(unindexed.Dir(staticPath))
	router.Handle("/*", fs)

	return r.Port, router, err
}
