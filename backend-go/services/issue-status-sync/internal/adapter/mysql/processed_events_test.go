//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// usage-service/internal/adapter/mysql/repository_test.go's setup shape.
//
// No tenant-isolation test here (unlike usage-service's mirror) —
// processed_events has no tenant_id column in either dialect (see
// migrations/mysql/0001_processed_events.up.sql's doc comment), so
// TASK-BE-DB-003's pattern doesn't apply to this service; see
// BE-DB-SOL-003 §4 for the explicit check that led to this decision.
package mysql

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
)

func setupStore(t *testing.T) *ProcessedEventsStore {
	t.Helper()
	// testutil.StartMySQL returns "mysql://root:orca@tcp(host:port)/db" —
	// valid as-is for golang-migrate's mysql driver CLI (used below), but
	// go-sql-driver/mysql's database/sql driver expects its OWN DSN format
	// with no "mysql://" scheme.
	rawDSN := testutil.StartMySQL(t, "issuestatussync")
	driverDSN := strings.TrimPrefix(rawDSN, "mysql://") + "?parseTime=true"

	migrationsPath, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", rawDSN, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	db, err := sql.Open("mysql", driverDSN)
	if err != nil {
		t.Fatalf("connecting to mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("pinging mysql: %v", err)
	}

	return New(db)
}

func TestProcessedEventsStore_SeenIsFalseForUnknownEvent(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	seen, err := store.Seen(ctx, uuid.NewString())
	if err != nil {
		t.Fatalf("Seen: %v", err)
	}
	if seen {
		t.Error("expected Seen to be false for an event never marked")
	}
}

func TestProcessedEventsStore_MarkSeenThenSeenIsTrue(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	eventID := uuid.NewString()

	if err := store.MarkSeen(ctx, eventID); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}
	seen, err := store.Seen(ctx, eventID)
	if err != nil {
		t.Fatalf("Seen: %v", err)
	}
	if !seen {
		t.Error("expected Seen to be true after MarkSeen")
	}
}

// TestProcessedEventsStore_MarkSeenIsIdempotent exercises the
// INSERT IGNORE / ON CONFLICT DO NOTHING equivalence — marking the same
// event twice (JetStream at-least-once redelivery) must not error.
func TestProcessedEventsStore_MarkSeenIsIdempotent(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	eventID := uuid.NewString()

	if err := store.MarkSeen(ctx, eventID); err != nil {
		t.Fatalf("first MarkSeen: %v", err)
	}
	if err := store.MarkSeen(ctx, eventID); err != nil {
		t.Fatalf("second MarkSeen (redelivery) should be a no-op, got error: %v", err)
	}

	seen, err := store.Seen(ctx, eventID)
	if err != nil {
		t.Fatalf("Seen: %v", err)
	}
	if !seen {
		t.Error("expected Seen to be true after idempotent MarkSeen")
	}
}
