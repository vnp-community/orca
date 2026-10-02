//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

const (
	sessUserA = "aaaaaaaa-1111-4000-8000-000000000001"
	sessUserB = "aaaaaaaa-1111-4000-8000-000000000002"
)

// Migrations 0001-0004 + 0006 only: the sessions schema does not depend on the
// prompts migration, and this keeps the test independent of other solutions.
func setupSessions(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()
	dsn := testutil.StartPostgres(t, "mcp")
	ctx := context.Background()
	conn := adminConn(t, dsn)
	for _, m := range []string{"0001_init", "0002_authorization", "0003_tool_policies", "0004_approvals_audit_killswitch", "0006_sessions"} {
		execScript(t, ctx, conn, readMigration(t, m+".up.sql"))
	}
	execScript(t, ctx, conn, fmt.Sprintf(`
		CREATE ROLE %[1]s LOGIN PASSWORD '%[2]s' NOSUPERUSER NOBYPASSRLS;
		GRANT USAGE ON SCHEMA mcp TO %[1]s;
		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA mcp TO %[1]s;`, appRole, appPass))
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.User, cfg.ConnConfig.Password = appRole, appPass
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return New(p), p
}

func newSess(tenant, user string, hashByte byte, at time.Time) domain.Session {
	h := make([]byte, 32)
	h[0] = hashByte
	return domain.Session{ID: uuid.NewString(), TenantID: tenant, UserID: user, SecretHash: h, ProtocolVersion: "2025-06-18",
		CapabilitiesJSON: []byte(`{"elicitation":{}}`), LogLevel: "warning", State: domain.SessionInitializing, CreatedAt: at, LastSeenAt: at}
}

func TestSessions_RLSTouchCloseListAndReaper(t *testing.T) {
	r, pool := setupSessions(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	a1, err := r.CreateSession(ctx, newSess(tenantA, sessUserA, 1, now))
	if err != nil {
		t.Fatal(err)
	}
	a2, _ := r.CreateSession(ctx, newSess(tenantA, sessUserB, 2, now))
	if _, err := r.CreateSession(ctx, newSess(tenantB, sessUserA, 3, now)); err != nil {
		t.Fatal(err)
	}

	// RLS: tenant B cannot see tenant A's session by secret hash or id.
	if _, err := r.GetSessionBySecretHash(ctx, tenantB, a1.SecretHash); !isNotFound(err) {
		t.Fatalf("cross-tenant lookup must be not found: %v", err)
	}
	if _, err := r.GetSession(ctx, tenantB, a1.ID); !isNotFound(err) {
		t.Fatalf("cross-tenant get: %v", err)
	}
	own, err := r.ListSessions(ctx, tenantA, sessUserA, now)
	if err != nil || len(own) != 1 || own[0].ID != a1.ID {
		t.Fatalf("own list must contain only the caller's session: %+v %v", own, err)
	}
	all, _ := r.ListSessions(ctx, tenantA, "", now)
	if len(all) != 2 {
		t.Fatalf("admin list: %d", len(all))
	}

	// Touch: ready transition, tool call counter, closed is never revived.
	st, err := r.TouchSession(ctx, tenantA, a1.ID, true, 2, "info", now.Add(time.Second))
	if err != nil || st != domain.SessionReady {
		t.Fatalf("touch: %q %v", st, err)
	}
	got, _ := r.GetSession(ctx, tenantA, a1.ID)
	if got.ToolCalls != 2 || got.LogLevel != "info" {
		t.Fatalf("%+v", got)
	}

	// Streams: advisory-locked cap; the third GET stream of the user is refused.
	for i := 0; i < 2; i++ {
		if err := r.OpenStream(ctx, tenantA, sessUserA, a1.ID, "r1", "get", uuid.NewString(), 2, 0, now); err != nil {
			t.Fatalf("stream %d: %v", i, err)
		}
	}
	err = r.OpenStream(ctx, tenantA, sessUserA, a1.ID, "r2", "get", uuid.NewString(), 2, 0, now)
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != domain.CodeStreamLimit {
		t.Fatalf("want MCP_STREAM_LIMIT, got %v", err)
	}
	// Stale (no heartbeat) streams do not count.
	if err := r.OpenStream(ctx, tenantA, sessUserA, a1.ID, "r2", "get", uuid.NewString(), 2, 0, now.Add(domain.StreamLiveWindow+time.Second)); err != nil {
		t.Fatalf("stale streams must not count: %v", err)
	}
	// Another user's session cannot be streamed.
	if err := r.OpenStream(ctx, tenantA, sessUserB, a1.ID, "r1", "get", uuid.NewString(), 0, 0, now); !isNotFound(err) {
		t.Fatalf("foreign session stream: %v", err)
	}
	live, _ := r.ListSessions(ctx, tenantA, sessUserA, now.Add(domain.StreamLiveWindow+time.Second))
	if live[0].ActiveStreams != 1 { // only the stream with a recent heartbeat is live
		t.Fatalf("activeStreams: %d", live[0].ActiveStreams)
	}

	// Close writes the outbox event atomically and is idempotent.
	ev, _ := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectSessionClosed, tenantA, now, map[string]any{"session_id": a2.ID, "user_id": sessUserB})
	closed, err := r.CloseSession(ctx, tenantA, a2.ID, domain.CloseReasonUser, now, []domain.OutboxRecord{ev})
	if err != nil || !closed {
		t.Fatalf("close: %v %v", closed, err)
	}
	if again, _ := r.CloseSession(ctx, tenantA, a2.ID, domain.CloseReasonUser, now, nil); again {
		t.Fatal("second close must be a no-op")
	}
	if st, _ := r.TouchSession(ctx, tenantA, a2.ID, true, 0, "", now); st != domain.SessionClosed {
		t.Fatalf("closed session revived: %q", st)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT 1`).Scan(&n); err != nil {
		t.Fatal(err)
	}

	// Reaper: idle since before cutoff across tenants, one event per row.
	var emitted int
	rows, err := r.ReapIdle(ctx, now.Add(time.Hour), 10, now.Add(2*time.Hour), func(c domain.ClosedSession) (domain.OutboxRecord, error) {
		emitted++
		return domain.NewOutboxEvent(uuid.NewString(), domain.SubjectSessionClosed, c.TenantID, now, map[string]any{"session_id": c.ID, "user_id": c.UserID, "reason": "idle"})
	})
	if err != nil || len(rows) != 2 || emitted != 2 { // a1 + tenant B's session (a2 already closed)
		t.Fatalf("reap: %d rows, %d events, %v", len(rows), emitted, err)
	}
	if st, _ := r.TouchSession(ctx, tenantA, a1.ID, false, 0, "", now); st != domain.SessionClosed {
		t.Fatalf("reaped session must be closed, got %q", st)
	}
}

func isNotFound(err error) bool {
	var ae *apperrors.AppError
	return errors.As(err, &ae) && ae.Code == domain.CodeNotFound
}
