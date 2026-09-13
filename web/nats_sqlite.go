package web

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"datastar-template"
	"datastar-template/sql/sqlcgen"

	"github.com/nats-io/nats.go"
	"github.com/starfederation/datastar-go/datastar"
)

func natsSqliteHome(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := sqlcgen.New(db).GetAllUsers(r.Context())
		if err != nil {
			slog.Error("query error", "component", "GetAllUsers", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := NatsSqlite(users, "").Render(r.Context(), w); err != nil {
			slog.Debug("render error", "component", "NatsSqlite", "err", err)
		}
	}
}

func sseNatsSqlite(db *sql.DB, nc *nats.Conn) http.HandlerFunc {
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
		// that may already be present, this stacks them instead
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
