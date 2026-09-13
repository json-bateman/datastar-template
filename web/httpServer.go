package web

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"datastar-template"

	"datastar-template/natsrv"
	"datastar-template/sql/sqlcgen"

	"github.com/benbjohnson/hashfs"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go"
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
// assets that ignore hashfs hashing.
func withDefaultCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}

func setupRoutes(db *sql.DB, nc *nats.Conn) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Handle("/static/*", withDefaultCache(hashfs.FileServer(StaticSys)))

	r.Get("/", home(db))
	r.Get("/sse/home", sseHome(db, nc))
	r.Post("/user", addUser(db, nc))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if err := NotFound().Render(r.Context(), w); err != nil {
			slog.Debug("render error", "component", "NotFound", "err", err)
		}
	})

	return r
}

func home(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := sqlcgen.New(db).GetAllUsers(r.Context())
		if err != nil {
			slog.Error("query error", "component", "GetAllUsers", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := Home(users, "").Render(r.Context(), w); err != nil {
			slog.Debug("render error", "component", "Home", "err", err)
		}
	}
}

func sseHome(db *sql.DB, nc *nats.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r, datastar.WithCompression(datastar.WithBrotli()))

		ch := make(chan *nats.Msg, 64)
		sub, err := nc.ChanSubscribe("home", ch)
		if err != nil {
			slog.Error("nats subscribe failed", "err", err)
			return
		}
		defer sub.Unsubscribe()

		ticker := time.NewTicker(time.Millisecond * 300)
		defer ticker.Stop()

		s := "Now with Nats and SQLite!"
		t := 0

		for {
			select {
			case <-ticker.C:
				if len(s) <= t {
					return
				}
				t++
				if err := sse.PatchElementTempl(Message(s[:t])); err != nil {
					return
				}

			case <-ch:
				users, err := sqlcgen.New(db).GetAllUsers(r.Context())
				if err != nil {
					slog.Error("query error", "component", "GetAllUsers", "err", err)
					return
				}
				if err := sse.PatchElementTempl(UserTable(users)); err != nil {
					return
				}
			case <-r.Context().Done():
				return
			}
		}
	}
}

// toastError appends an error toast into #toast-host.
func toastError(w http.ResponseWriter, r *http.Request, message string) {
	sse := datastar.NewSSE(w, r)
	id := fmt.Sprintf("toast-%d", time.Now().UnixNano())
	if err := sse.PatchElementTempl(
		Toast(id, message),
		datastar.WithSelector("#toast-host"),
		// Needs to be append mode because we don't want to destroy a toast
		// That may already be present, this stacks them instead
		datastar.WithMode(datastar.ElementPatchModeAppend),
	); err != nil {
		slog.Error("toast: patch failed", "err", err)
	}
}

func addUser(db *sql.DB, nc *nats.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := sqlcgen.New(db).CreateUser(r.Context(), dtemplate.GenerateName())
		if err != nil {
			slog.Error("query error", "component", "CreateUser", "err", err)
			toastError(w, r, "Could not add user")
			return
		}
		// Publish to NATS, this is being listened for on the open SSE connection that's subscribed to "home"
		if err := nc.Publish("home", nil); err != nil {
			slog.Error("nats publish failed", "err", err)
		}
	}
}

// RunBlocking starts the HTTP server and blocks until setupCtx is cancelled, at
// which point it shuts down gracefully.
func RunBlocking(setupCtx context.Context, db *sql.DB) error {
	if Version == "dev" {
		Version = getVersion()
	}
	nc, err := natsrv.StartNats()
	if err != nil {
		return fmt.Errorf("start nats: %w", err)
	}
	defer nc.Close()

	router := setupRoutes(db, nc)

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
