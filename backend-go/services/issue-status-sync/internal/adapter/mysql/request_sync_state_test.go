//go:build integration

package mysql

import (
	"context"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

func TestRequestSyncStateStore_Advance(t *testing.T) {
	ps, _ := setupStoreAndDSN(t)
	store := NewRequestSyncStateStore(ps.db)
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
	if got, err := store.Advance(ctx, uuid.NewString(), req, 1, "Done"); err != nil || !got {
		t.Errorf("other tenant: applied=%v err=%v", got, err)
	}
}

func TestRequestSyncStateStore_ConcurrentAdvanceHasOneWinner(t *testing.T) {
	ps, _ := setupStoreAndDSN(t)
	store := NewRequestSyncStateStore(ps.db)
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
	_, rawDSN := setupStoreAndDSN(t) // already up
	path, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"down", "1"}, {"up"}, {"down", "-all"}, {"up"}} {
		full := append([]string{"-path", path, "-database", rawDSN}, args...)
		if out, err := exec.Command("migrate", full...).CombinedOutput(); err != nil {
			t.Fatalf("migrate %v: %v\n%s", args, err, out)
		}
	}
}
