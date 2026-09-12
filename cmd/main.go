package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"datastar-template"
	"datastar-template/web"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	// CTRL+C sends SIGINT; `kill` and service stop send SIGTERM. Both trigger a graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	dtemplate.LoadSettings()

	if err := web.RunBlocking(ctx); err != nil {
		return fmt.Errorf("run web server: %w", err)
	}
	return nil
}
