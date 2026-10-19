// Command ledgerd runs the LedgerLite HTTP server (SVC-004).
// Standard library only (CT-001, NFR-003): go.mod has no require directive.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ledgerlite/ledgerlite/internal/httpapi"
	"github.com/ledgerlite/ledgerlite/internal/ledger"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	limit := flag.Int64("daily-limit", 1_000_000, "default per-account daily transfer limit in minor units")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc, err := ledger.NewService(ledger.NewMemoryStore(), ledger.Config{DefaultDailyLimit: *limit})
	if err != nil {
		log.Error("cannot start ledger", "error", err)
		os.Exit(1)
	}
	srv := &http.Server{Addr: *addr, Handler: httpapi.New(svc, log), ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("ledgerd listening", "addr", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
