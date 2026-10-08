//go:build integration

package postgres

import (
	"context"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
)

func migrate(t *testing.T, dsn string, args ...string) {
	t.Helper()
	path, err := filepath.Abs("../../../migrations/postgres")
	if err != nil {
		t.Fatal(err)
	}
	full := append([]string{"-path", path, "-database", dsn}, args...)
	if out, err := exec.Command("migrate", full...).CombinedOutput(); err != nil {
		t.Fatalf("migrate %v: %v\n%s", args, err, out)
	}
}

func setupPool(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := testutil.StartPostgres(t, "issuestatussync")
	migrate(t, dsn, "up")
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, dsn
}

func TestRequestSyncStateStore_Advance(t *testing.T) {
	pool, _ := setupPool(t)
	store := NewRequestSyncStateStore(pool)
	ctx := context.Background()
	tenant, req := uuid.NewString(), uuid.NewString()

	steps := []struct {
		version int64
		want    bool
	}{{1, true}, {2, true}, {2, false}, {1, false}, {7, true}}
	for _, s := range steps {
		got, err := store.Advance(ctx, tenant, req, s.version, "In Progress")
		if err != nil {
			t.Fatal(err)
		}
		if got != s.want {
			t.Errorf("Advance(v=%d) = %v, want %v", s.version, got, s.want)
		}
	}
	// A different tenant with the same request id has its own state.
	if got, err := store.Advance(ctx, uuid.NewString(), req, 1, "Done"); err != nil || !got {
		t.Errorf("other tenant: applied=%v err=%v", got, err)
	}
}

func TestRequestSyncStateStore_ConcurrentAdvanceHasOneWinner(t *testing.T) {
	pool, _ := setupPool(t)
	store := NewRequestSyncStateStore(pool)
	tenant, req := uuid.NewString(), uuid.NewString()
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			applied, err := store.Advance(context.Background(), tenant, req, 5, "Done")
			if err != nil {
				t.Error(err)
			}
			if applied {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Errorf("winners = %d, want exactly 1", winners.Load())
	}
}

func TestMigrationsUpDownUp(t *testing.T) {
	_, dsn := setupPool(t)       // already up
	migrate(t, dsn, "down", "1") // drops 0002
	migrate(t, dsn, "up")
	migrate(t, dsn, "down", "-all")
	migrate(t, dsn, "up")
}
