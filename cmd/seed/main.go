package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	dtemplate "datastar-template"
	"datastar-template/sql"
	"datastar-template/sql/sqlcgen"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// teardownDatabase removes the sqlite file and its WAL/SHM sidecars so
// NewDatabase recreates a fresh, empty database via migrations.
func teardownDatabase(dbPath string) error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", dbPath+suffix, err)
		}
	}
	return nil
}

func run(ctx context.Context) error {
	cfg := dtemplate.LoadSettings()

	if err := teardownDatabase(cfg.DBPath); err != nil {
		return fmt.Errorf("teardown db: %w", err)
	}

	db, err := sql.NewDatabase(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("initialize db: %w", err)
	}
	defer db.Close()
	q := sqlcgen.New(db)

	for range 5 {
		user, err := q.CreateUser(ctx, dtemplate.GenerateName())
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		slog.Info("seeded user", "id", user.ID, "username", user.Username)
	}
	return nil
}
