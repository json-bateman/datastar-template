package web

import (
	"context"
	"embed"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"datastar-template"

	"github.com/benbjohnson/hashfs"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
)

//go:embed static/*
var StaticFS embed.FS

var Version = "dev"

// StaticSys serves embedded static files under content-hashed names so they can
// be cached forever; when a file changes its hash changes and busts the cache.
var StaticSys = hashfs.NewFS(StaticFS)

// StaticPath returns the hashed URL for a file under static/, e.g.
// StaticPath("css/main.css") -> "/static/css/main.abc123.css".
func StaticPath(format string, args ...any) string {
	return "/" + StaticSys.HashName(fmt.Sprintf("static/"+format, args...))
}

// getVersion returns the nearest git tag (e.g. "v1.2.3"), or "v1.2.3-4-gabcdef"
// if HEAD is ahead of the tag, or just the short commit hash if the repo has
// no tags at all.
func getVersion() string {
	cmd := exec.Command("git", "describe", "--tags", "--always")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// withDefaultCache sets a long-lived Cache-Control header before the wrapped
// handler runs. hashfs's FileServer overwrites this with its own header for
// content-hashed requests, so this only takes effect for plain-named static
// assets that bypass hashing.
func withDefaultCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}

func setupRoutes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Handle("/static/*", withDefaultCache(hashfs.FileServer(StaticSys)))

	r.Get("/", home)
	r.Get("/sse/aloha", sseAloha)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if err := NotFound().Render(r.Context(), w); err != nil {
			slog.Debug("render error", "component", "NotFound", "err", err)
		}
	})

	return r
}

func home(w http.ResponseWriter, r *http.Request) {
	if err := Home("").Render(r.Context(), w); err != nil {
		slog.Debug("render error", "component", "NotFound", "err", err)
	}
}

func sseAloha(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r, datastar.WithCompression(datastar.WithBrotli()))

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	s := "Aloha Traveler"
	t := 0

	for {
		if err := sse.PatchElementTempl(Home(s[:t])); err != nil {
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if len(s) <= t {
				return
			}
			t++
		}
	}
}

// RunBlocking starts the HTTP server and blocks until setupCtx is cancelled, at
// which point it shuts down gracefully.
func RunBlocking(setupCtx context.Context) error {
	if Version == "dev" {
		Version = getVersion()
	}
	router := setupRoutes()

	addr := fmt.Sprintf(":%d", dtemplate.Env.Port)
	srv := http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		<-setupCtx.Done()
		log.Printf("shutdown 💽__💽")
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down server: %v", err)
		}
	}()

	log.Printf("Starting server on http://localhost%s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Error starting server: %v", err)
	}
	return nil
}
