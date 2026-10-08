//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

type specDB struct {
	repo *Repository
	pool *pgxpool.Pool
	dsn  string
}

func setupSpecDB(t *testing.T) specDB {
	t.Helper()
	dsn := testutil.StartPostgres(t, "task")
	migrate(t, dsn, "up")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return specDB{repo: New(pool), pool: pool, dsn: dsn}
}

func migrate(t *testing.T, dsn string, args ...string) {
	t.Helper()
	path, _ := filepath.Abs("../../../migrations/postgres")
	cmd := exec.Command("migrate", append([]string{"-path", path, "-database", dsn}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("migrate %v: %v\n%s", args, err, out)
	}
}

func mkTask(t *testing.T, r *Repository, tenantID, parentID string) string {
	t.Helper()
	task, err := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, parentID, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	return task.ID
}

func mkSpec(t *testing.T, tenantID, taskID, json string) domain.TaskSpec {
	t.Helper()
	s, err := domain.NewTaskSpec(taskID, tenantID, 1, []byte(json))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMigration0020And0021_UpDownUp(t *testing.T) {
	db := setupSpecDB(t)
	ctx := context.Background()
	exists := func() (bool, bool) {
		var a, b bool
		_ = db.pool.QueryRow(ctx, `SELECT to_regclass('task.task_specs') IS NOT NULL, to_regclass('task.task_execution_records') IS NOT NULL`).Scan(&a, &b)
		return a, b
	}
	if a, b := exists(); !a || !b {
		t.Fatal("tables missing after up")
	}
	migrate(t, db.dsn, "down", "2")
	if a, b := exists(); a || b {
		t.Fatal("tables still present after down 2")
	}
	migrate(t, db.dsn, "up")
	if a, b := exists(); !a || !b {
		t.Fatal("tables missing after second up")
	}
}

func TestTaskSpecRepository_CreateGetUpdate(t *testing.T) {
	db := setupSpecDB(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	id := mkTask(t, db.repo, tenantID, "")

	created, err := db.repo.Upsert(ctx, mkSpec(t, tenantID, id, `{"title":"Nguyễn Văn — kiểm thử","n":1.0}`), 0)
	if err != nil || created.Version != 1 {
		t.Fatalf("create: %+v %v", created, err)
	}
	got, err := db.repo.GetMany(ctx, tenantID, []string{id, uuid.NewString()})
	if err != nil || len(got) != 1 {
		t.Fatalf("get: %+v %v", got, err)
	}
	canon, err := got[0].Canonicalized()
	if err != nil || string(canon.Spec) != `{"n":1,"title":"Nguyễn Văn — kiểm thử"}` || domain.DigestOfCanonical(canon.Spec) != got[0].Digest {
		t.Fatalf("round trip lost data or digest: %s (%v)", canon.Spec, err)
	}
	if _, err := db.repo.Upsert(ctx, mkSpec(t, tenantID, id, `{"v":2}`), 0); !errors.Is(err, domain.ErrTaskSpecVersionConflict) {
		t.Fatalf("second create must conflict, got %v", err)
	}
	if _, err := db.repo.Upsert(ctx, mkSpec(t, tenantID, id, `{"v":2}`), 9); !errors.Is(err, domain.ErrTaskSpecVersionConflict) {
		t.Fatalf("stale version must conflict, got %v", err)
	}
	upd, err := db.repo.Upsert(ctx, mkSpec(t, tenantID, id, `{"v":2}`), 1)
	if err != nil || upd.Version != 2 {
		t.Fatalf("update: %+v %v", upd, err)
	}
	if _, err := db.repo.Upsert(ctx, mkSpec(t, tenantID, uuid.NewString(), `{}`), 3); !errors.Is(err, domain.ErrTaskSpecNotFound) {
		t.Fatalf("update of a missing spec must be NotFound, got %v", err)
	}
	if _, err := db.repo.Upsert(ctx, mkSpec(t, tenantID, uuid.NewString(), `{}`), 0); !errors.Is(err, domain.ErrTaskSpecNotFound) {
		t.Fatalf("create for a missing task must be NotFound (FK), got %v", err)
	}
}

func TestTaskSpecRepository_CASWithEightGoroutines(t *testing.T) {
	db := setupSpecDB(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	id := mkTask(t, db.repo, tenantID, "")
	if _, err := db.repo.Upsert(ctx, mkSpec(t, tenantID, id, `{"v":0}`), 0); err != nil {
		t.Fatal(err)
	}
	var wins, conflicts atomic.Int32
	var wg sync.WaitGroup
	for i := 1; i <= 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := db.repo.Upsert(ctx, mkSpec(t, tenantID, id, fmt.Sprintf(`{"v":%d}`, i)), 1)
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, domain.ErrTaskSpecVersionConflict):
				conflicts.Add(1)
			default:
				t.Errorf("unexpected: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if wins.Load() != 1 || conflicts.Load() != 7 {
		t.Fatalf("wins=%d conflicts=%d", wins.Load(), conflicts.Load())
	}
}

func TestTaskSpecRepository_LockSubtreeBlocksUpsert(t *testing.T) {
	db := setupSpecDB(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	a, b := mkTask(t, db.repo, tenantID, ""), mkTask(t, db.repo, tenantID, "")
	for _, id := range []string{a, b} {
		if _, err := db.repo.Upsert(ctx, mkSpec(t, tenantID, id, `{}`), 0); err != nil {
			t.Fatal(err)
		}
	}
	n, err := db.repo.LockSubtree(ctx, tenantID, []string{a}, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("lock: %d %v", n, err)
	}
	if n, _ := db.repo.LockSubtree(ctx, tenantID, []string{a, b}, time.Now()); n != 1 {
		t.Fatalf("only the unlocked spec counts, got %d", n)
	}
	if _, err := db.repo.Upsert(ctx, mkSpec(t, tenantID, a, `{"x":1}`), 1); !errors.Is(err, domain.ErrTaskSpecLocked) {
		t.Fatalf("got %v", err)
	}
	if locked, err := db.repo.IsLocked(ctx, tenantID, a); err != nil || !locked {
		t.Fatalf("IsLocked: %v %v", locked, err)
	}
	if locked, _ := db.repo.IsLocked(ctx, tenantID, uuid.NewString()); locked {
		t.Fatal("unknown task cannot be locked")
	}
	if has, _ := db.repo.HasSpec(ctx, tenantID, a); !has {
		t.Fatal("HasSpec false")
	}
}

func TestTaskSpecRepository_CascadeOnTaskDelete(t *testing.T) {
	db := setupSpecDB(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	id := mkTask(t, db.repo, tenantID, "")
	_, _ = db.repo.Upsert(ctx, mkSpec(t, tenantID, id, `{}`), 0)
	if err := db.repo.Delete(ctx, tenantID, id); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.repo.GetMany(ctx, tenantID, []string{id}); len(got) != 0 {
		t.Fatal("spec survived its task")
	}
}

func TestTaskSpecRepository_GetManyBatchesOver200(t *testing.T) {
	db := setupSpecDB(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	var ids []string
	for i := 0; i < 205; i++ {
		id := mkTask(t, db.repo, tenantID, "")
		ids = append(ids, id)
		if i%5 == 0 {
			if _, err := db.repo.Upsert(ctx, mkSpec(t, tenantID, id, `{}`), 0); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := db.repo.GetMany(ctx, tenantID, ids)
	if err != nil || len(got) != 41 {
		t.Fatalf("got %d specs, err %v", len(got), err)
	}
}

func TestTaskSpecRepository_TenantIsolation(t *testing.T) {
	db := setupSpecDB(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.NewString(), uuid.NewString()
	id := mkTask(t, db.repo, tenantA, "")
	_, _ = db.repo.Upsert(ctx, mkSpec(t, tenantA, id, `{}`), 0)
	if got, _ := db.repo.GetMany(ctx, tenantB, []string{id}); len(got) != 0 {
		t.Fatal("tenant B read tenant A's spec")
	}
	if n, _ := db.repo.LockSubtree(ctx, tenantB, []string{id}, time.Now()); n != 0 {
		t.Fatal("tenant B locked tenant A's spec")
	}
	if _, err := db.repo.Upsert(ctx, mkSpec(t, tenantB, id, `{"evil":1}`), 1); err == nil {
		t.Fatal("tenant B overwrote tenant A's spec")
	}
}

// Superusers bypass RLS, so the policy itself is proven through an unprivileged role.
func TestTaskSpecAndRecords_RLSEnforcedForNonSuperuser(t *testing.T) {
	db := setupSpecDB(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	id := mkTask(t, db.repo, tenantID, "")
	_, _ = db.repo.Upsert(ctx, mkSpec(t, tenantID, id, `{}`), 0)
	_, _ = db.repo.InsertExecutionRecord(ctx, domain.ExecutionRecord{TenantID: tenantID, TaskID: id, Attempt: 1, ParseStatus: domain.ParseStatusOK})
	for _, q := range []string{
		`CREATE ROLE rls_app LOGIN PASSWORD 'x' NOSUPERUSER`,
		`GRANT USAGE ON SCHEMA task TO rls_app`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA task TO rls_app`,
	} {
		if _, err := db.pool.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	cfg, _ := pgxpool.ParseConfig(db.dsn)
	cfg.ConnConfig.User, cfg.ConnConfig.Password = "rls_app", "x"
	app, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	var n int
	_ = app.QueryRow(ctx, `SELECT count(*) FROM task.task_specs`).Scan(&n)
	if n != 0 {
		t.Fatalf("without a tenant GUC the role must see nothing, saw %d", n)
	}
	_ = app.QueryRow(ctx, `SELECT count(*) FROM task.task_execution_records`).Scan(&n)
	if n != 0 {
		t.Fatalf("records visible without tenant GUC: %d", n)
	}
	appRepo := New(app)
	if got, err := appRepo.GetMany(ctx, tenantID, []string{id}); err != nil || len(got) != 1 {
		t.Fatalf("repository with tenant must see its row: %v %v", got, err)
	}
	if got, _ := appRepo.GetMany(ctx, uuid.NewString(), []string{id}); len(got) != 0 {
		t.Fatal("other tenant saw the row")
	}
	// The repository sets the GUC from its tenant argument, so a row for another tenant cannot be written.
	other := uuid.NewString()
	if _, err := app.Exec(ctx, `SELECT set_config('app.tenant_id', $1, false)`, other); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Exec(ctx, `INSERT INTO task.task_specs (task_id, tenant_id, schema_version, spec, digest) VALUES ($1, $2, 1, '{}', 'd')`, id, tenantID); err == nil {
		t.Fatal("WITH CHECK must reject a row for a tenant other than the GUC's")
	}
}

func TestRunInTxWithSpecs_RollsBackSpecsAndTasksTogether(t *testing.T) {
	db := setupSpecDB(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	var created string
	err := db.repo.RunInTxWithSpecs(ctx, func(ctx context.Context, tasks usecase.TaskRepository, _ usecase.EdgeRepository, specs usecase.TaskSpecRepository) error {
		task, _ := domain.NewTask(uuid.NewString(), tenantID, "in tx", domain.StatusOpen, "", "")
		if _, err := tasks.Create(ctx, task); err != nil {
			return err
		}
		created = task.ID
		if _, err := specs.Upsert(ctx, mkSpec(t, tenantID, task.ID, `{"ok":1}`), 0); err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if err == nil {
		t.Fatal("expected the forced error")
	}
	if _, err := db.repo.Get(ctx, tenantID, created); err == nil {
		t.Fatal("task survived the rollback")
	}
	if got, _ := db.repo.GetMany(ctx, tenantID, []string{created}); len(got) != 0 {
		t.Fatal("spec survived the rollback")
	}
	// And a commit keeps both.
	if err := db.repo.RunInTxWithSpecs(ctx, func(ctx context.Context, tasks usecase.TaskRepository, _ usecase.EdgeRepository, specs usecase.TaskSpecRepository) error {
		task, _ := domain.NewTask(uuid.NewString(), tenantID, "kept", domain.StatusOpen, "", "")
		if _, err := tasks.Create(ctx, task); err != nil {
			return err
		}
		created = task.ID
		_, err := specs.Upsert(ctx, mkSpec(t, tenantID, task.ID, `{"ok":1}`), 0)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.repo.GetMany(ctx, tenantID, []string{created}); len(got) != 1 {
		t.Fatal("committed spec missing")
	}
}

func TestTaskExecutionRecordRepository_InsertListLatestUnicode(t *testing.T) {
	db := setupSpecDB(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	a, b := mkTask(t, db.repo, tenantID, ""), mkTask(t, db.repo, tenantID, "")
	ins := func(task string, attempt int, class domain.FailureClass) domain.ExecutionRecord {
		r, err := db.repo.InsertExecutionRecord(ctx, domain.ExecutionRecord{
			TenantID: tenantID, TaskID: task, Attempt: attempt, ParseStatus: domain.ParseStatusOK, FailureClass: class,
			Result: []byte(`{"summary":"Nguyễn Văn"}`), Changes: []byte(`{"files":["a.go"]}`), StdoutTail: "đuôi", ExecutionLinkID: uuid.NewString(),
		})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
		return r
	}
	ins(a, 1, domain.FailureRetryable)
	lastA := ins(a, 2, "")
	ins(b, 1, domain.FailureAgentDefect)
	if lastA.ID == "" || lastA.FailureClass != "" || lastA.StdoutTail != "đuôi" {
		t.Fatalf("returned record: %+v", lastA)
	}

	all, err := db.repo.ListExecutionRecords(ctx, tenantID, []string{a, b}, false, 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("all: %d %v", len(all), err)
	}
	latest, err := db.repo.ListExecutionRecords(ctx, tenantID, []string{a, b}, true, 0)
	if err != nil || len(latest) != 2 {
		t.Fatalf("latest: %d %v", len(latest), err)
	}
	for _, r := range latest {
		if r.TaskID == a && r.Attempt != 2 {
			t.Fatalf("latest for a must be attempt 2: %+v", r)
		}
	}
	var got domain.ExecutionRecord
	for _, r := range all {
		if r.ID == lastA.ID {
			got = r
		}
	}
	if string(got.Result) != `{"summary": "Nguyễn Văn"}` && string(got.Result) != `{"summary":"Nguyễn Văn"}` {
		t.Fatalf("unicode JSON round trip: %s", got.Result)
	}
	if limited, _ := db.repo.ListExecutionRecords(ctx, tenantID, []string{a, b}, false, 1); len(limited) != 1 {
		t.Fatal("limit ignored")
	}
}

func TestTaskExecutionRecordRepository_Guards(t *testing.T) {
	db := setupSpecDB(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.NewString(), uuid.NewString()
	id := mkTask(t, db.repo, tenantA, "")
	if _, err := db.repo.InsertExecutionRecord(ctx, domain.ExecutionRecord{TenantID: tenantA, TaskID: id, ParseStatus: "weird"}); err == nil {
		t.Fatal("CHECK must reject an unknown parse_status")
	}
	if _, err := db.repo.InsertExecutionRecord(ctx, domain.ExecutionRecord{TenantID: tenantA, TaskID: id, ParseStatus: domain.ParseStatusOK, FailureClass: "weird"}); err == nil {
		t.Fatal("CHECK must reject an unknown failure_class")
	}
	if _, err := db.repo.ListExecutionRecords(ctx, tenantA, nil, false, 0); !errors.Is(err, usecase.ErrInvalidArgument) {
		t.Fatalf("empty ids: %v", err)
	}
	_, _ = db.repo.InsertExecutionRecord(ctx, domain.ExecutionRecord{TenantID: tenantA, TaskID: id, Attempt: 1, ParseStatus: domain.ParseStatusMissing})
	if got, _ := db.repo.ListExecutionRecords(ctx, tenantB, []string{id}, false, 0); len(got) != 0 {
		t.Fatal("tenant B saw tenant A's records")
	}
	if err := db.repo.Delete(ctx, tenantA, id); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.repo.ListExecutionRecords(ctx, tenantA, []string{id}, false, 0); len(got) != 0 {
		t.Fatal("records survived the task (cascade)")
	}
}
