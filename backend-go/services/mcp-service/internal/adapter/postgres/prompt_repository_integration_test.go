//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// setupPrompts applies EVERY migration found on disk (other solutions add
// theirs concurrently) and returns repositories on the non-superuser role.
func setupPrompts(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()
	dsn := testutil.StartPostgres(t, "mcp")
	ctx := context.Background()
	conn := adminConn(t, dsn)
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "migrations", "postgres", "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations: %v", err)
	}
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		execScript(t, ctx, conn, string(b))
	}
	execScript(t, ctx, conn, `
		CREATE ROLE `+appRole+` LOGIN PASSWORD '`+appPass+`' NOSUPERUSER NOBYPASSRLS;
		GRANT USAGE ON SCHEMA mcp TO `+appRole+`;
		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA mcp TO `+appRole+`;`)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.User, cfg.ConnConfig.Password = appRole, appPass
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return New(pool), pool
}

func newPrompt(name string) domain.CustomPrompt {
	return domain.CustomPrompt{ID: uuid.NewString(), Name: name, Template: "Hello {{who}}",
		Arguments: []domain.PromptArgument{{Name: "who", Required: true}}, UpdatedBy: userA1, UpdatedAt: time.Now().UTC()}
}

func TestPrompts_RLSVersioningNamesAndOutbox(t *testing.T) {
	r, pool := setupPrompts(t)
	ctx := context.Background()

	ev, _ := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectPromptChanged, tenantA, time.Now(), map[string]any{"action": "upserted"})
	created, err := r.CreatePrompt(ctx, tenantA, newPrompt("daily_summary"), []domain.OutboxRecord{ev})
	if err != nil || created.Version != 1 || len(created.Arguments) != 1 {
		t.Fatalf("%+v %v", created, err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM mcp.outbox_events WHERE subject = $1`, domain.SubjectPromptChanged).Scan(&n); err == nil && n != 0 {
		// Outbox rows are tenant-scoped by RLS: a session without app.tenant_id sees none.
		t.Fatalf("outbox must be invisible without a tenant, saw %d", n)
	}

	// Tenant B sees nothing and can neither update nor delete A's prompt (RLS).
	if ps, err := r.ListPrompts(ctx, tenantB); err != nil || len(ps) != 0 {
		t.Fatalf("tenant B saw %v %v", ps, err)
	}
	cur := created
	if _, err := r.UpdatePrompt(ctx, tenantB, cur, nil); err == nil || !strings.Contains(err.Error(), domain.CodeNotFound) {
		t.Fatalf("cross-tenant update: %v", err)
	}
	if err := r.DeletePrompt(ctx, tenantB, created.ID, nil); err == nil || !strings.Contains(err.Error(), domain.CodeNotFound) {
		t.Fatalf("cross-tenant delete: %v", err)
	}
	// The same name in another tenant is fine; a duplicate in the same tenant conflicts.
	if _, err := r.CreatePrompt(ctx, tenantB, newPrompt("daily_summary"), nil); err != nil {
		t.Fatalf("same name in another tenant: %v", err)
	}
	if _, err := r.CreatePrompt(ctx, tenantA, newPrompt("daily_summary"), nil); err == nil || !strings.Contains(err.Error(), domain.CodePromptNameConflict) {
		t.Fatalf("duplicate name: %v", err)
	}

	// Concurrent writers from two pools: exactly one wins per version.
	r2 := New(pool)
	var wins, conflicts int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			q := created
			q.Description, q.UpdatedAt = "edit", time.Now().UTC()
			repo := r
			if i%2 == 1 {
				repo = r2
			}
			_, err := repo.UpdatePrompt(ctx, tenantA, q, nil)
			var ae *apperrors.AppError
			switch {
			case err == nil:
				atomic.AddInt32(&wins, 1)
			case errors.As(err, &ae) && ae.Code == domain.CodePromptVersionConflict:
				atomic.AddInt32(&conflicts, 1)
			default:
				t.Errorf("unexpected: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 || conflicts != 7 {
		t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
	}

	// Soft delete frees the name; the old row stays out of lists.
	if err := r.DeletePrompt(ctx, tenantA, created.ID, nil); err != nil {
		t.Fatal(err)
	}
	if ps, _ := r.ListPrompts(ctx, tenantA); len(ps) != 0 {
		t.Fatalf("deleted prompt listed: %v", ps)
	}
	if _, err := r.CreatePrompt(ctx, tenantA, newPrompt("daily_summary"), nil); err != nil {
		t.Fatalf("name must be reusable after delete: %v", err)
	}

	// The DB itself rejects a bad name (CHECK) and enforces the tenant cap.
	bad := newPrompt("Bad Name")
	if _, err := r.CreatePrompt(ctx, tenantA, bad, nil); err == nil {
		t.Fatal("CHECK constraint must reject the name")
	}
	for i := 0; ; i++ {
		_, err := r.CreatePrompt(ctx, tenantB, newPrompt("p_"+strings.Repeat("x", 3)+string(rune('a'+i/26))+string(rune('a'+i%26))), nil)
		if err != nil {
			if !strings.Contains(err.Error(), domain.CodePromptInvalid) || i < domain.MaxPromptsPerTenant-1 {
				t.Fatalf("cap hit at %d: %v", i, err)
			}
			break
		}
		if i > domain.MaxPromptsPerTenant+2 {
			t.Fatal("cap not enforced")
		}
	}
}
