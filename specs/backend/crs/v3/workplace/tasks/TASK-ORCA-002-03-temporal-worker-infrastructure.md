> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-002-03 — `planner-service`: Infrastructure — Postgres `planner.orca_task_tracking`

**Phase:** 0 — Nền tảng
**Scope:** ✅ vnp-workplace — Go (`backend/services/planner-service`, `:3013`)
**Source:** [SOL-ORCA-002 §3.3–§3.4](../solutions/SOL-ORCA-002-planner-orca-dispatcher.md#33-repository-interface--postgres-implementation)
**Depends On:** [TASK-ORCA-002-02](./TASK-ORCA-002-02-temporal-worker-domain.md) (cần `orcatask.Tracking`, `orcatask.Repository`, `orcatask.Result`)
**Estimated Files:** 4 files (1 migration up + 1 migration down + 1 repo impl + 1 test)
**Working Dir:** `/opt/repos/vnp-workplace/backend/services/planner-service/`

---

## Bối cảnh quan trọng

> **CR-ORCA-002 gốc KHÔNG có Change riêng cho migration** (khác thiết kế `temporal-worker` cũ, vốn có Change 3.4 rõ ràng) — SOL-ORCA-002 §2 D6 / §9 Discoveries #5 bổ sung phần này vì `orcatask.Repository` không chạy được nếu thiếu bảng thật.

- Schema `planner` là schema **mới** cho `planner-service` — chưa liệt kê trong `backend/specs/tdd/v1/02-database.md` (v3.0, viết trước khi `planner-service` tồn tại). CR-TASK-001 (Task Planning Engine) tạo migration nền tảng cho `plan_node`/`goal_node`/`plan_task` **trước** — migration của task này **land sau** migration nền tảng đó. Xác nhận số thứ tự file thật bằng `ls migrations/planner/` tại thời điểm implement, KHÔNG hard-code số thứ tự.
- Format migration theo TDD-02 §8 (`goose`, comment `-- +goose Up` / `-- +goose Down` trong **cùng 1 file**, không phải 2 file `.up.sql`/`.down.sql` riêng như convention `vnp-planner` cũ).
- Không có RLS/`org_id` — `PlanTask` (CR-TASK-001) hiện không mang trường org/workspace nào (xem SOL-ORCA-002 §3.4). Nếu multi-tenant RLS cần cho `planner-service` sau này, bổ sung cột `workspace_id` + policy cùng lúc với các bảng `plan`/`plantask` khác — không tự ý thêm riêng lẻ ở bảng tracking này.
- `planner-service` **chưa có DB pool** ở runtime hiện tại tại thời điểm task này bắt đầu (service hoàn toàn mới) — cấu hình `DatabaseURL`/`sqlx.DB` connection wiring thực hiện ở TASK-ORCA-002-06 (wiring), task này chỉ viết migration + repository implementation (không tự kết nối DB, nhận `*sqlx.DB` từ constructor).

---

## Mục tiêu

Migration SQL cho bảng `planner.orca_task_tracking` + implement `orcatask.Repository` bằng PostgreSQL (`sqlx`), theo đúng pattern TDD-00 §4 (Row có `db` tags riêng biệt khỏi domain entity, mapper `toRow`/`toDomain`, wrap `sql.ErrNoRows → orcatask.ErrNotFound` tại boundary).

---

## Acceptance Criteria

- [ ] Migration áp dụng được (`goose up`/`goose down` roundtrip sạch) — số thứ tự xác nhận thật tại thời điểm implement
- [ ] `OrcaTaskTrackingRepository` implement đủ 5 method của `orcatask.Repository`
- [ ] `FindByPlannerTaskID`/`FindByOrcaTaskID` trả `orcatask.ErrNotFound` khi không tìm thấy (wrap `sql.ErrNoRows`, KHÔNG rò rỉ `sql.ErrNoRows` ra ngoài package — TDD-00 §3 Error Wrapping)
- [ ] `UpdateResult` marshal `orcatask.Result` sang cột `result JSONB`, `NULL`-safe khi `result == nil`
- [ ] `Save` dùng `NamedExecContext` theo pattern TDD-00 §4
- [ ] Test dùng `sqlmock` (`github.com/DATA-DOG/go-sqlmock`) hoặc testcontainers Postgres — coverage ≥ 70% (mức infra)

---

## File 1: `migrations/planner/00X_create_orca_task_tracking.sql`

> Thay `00X` bằng số thứ tự thật tiếp theo trong `migrations/planner/` tại thời điểm implement (xem "Bối cảnh quan trọng").

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS planner.orca_task_tracking (
    id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    planner_task_id VARCHAR(64)  NOT NULL,      -- PlanTask.ID (CR-TASK-001)
    planner_job_id  VARCHAR(64),                -- Plan.ID
    orca_task_id    VARCHAR(128),               -- NULL until SubmitTask succeeds
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',
    result          JSONB,                      -- orcatask.Result, NULL until terminal
    dispatched_at   TIMESTAMPTZ  NOT NULL,
    deadline_at     TIMESTAMPTZ  NOT NULL,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_orca_task_tracking_planner_task
    ON planner.orca_task_tracking(planner_task_id);

CREATE INDEX IF NOT EXISTS idx_orca_task_tracking_orca_task
    ON planner.orca_task_tracking(orca_task_id);

CREATE INDEX IF NOT EXISTS idx_orca_task_tracking_active
    ON planner.orca_task_tracking(status) WHERE status = 'in_progress';

-- +goose Down
DROP TABLE IF EXISTS planner.orca_task_tracking;
```

---

## File 2: `internal/infrastructure/persistence/postgres/orcatask_repo.go`

```go
// Package postgres provides PostgreSQL implementations of planner-service's
// domain repositories.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/vnptech/kwp/services/planner-service/internal/domain/orcatask"
)

// orcaTaskTrackingRow is the database row mapping (NOT the domain entity).
type orcaTaskTrackingRow struct {
	ID            string         `db:"id"`
	PlannerTaskID string         `db:"planner_task_id"`
	PlannerJobID  sql.NullString `db:"planner_job_id"`
	OrcaTaskID    sql.NullString `db:"orca_task_id"`
	Status        string         `db:"status"`
	ResultJSON    []byte         `db:"result"`
	DispatchedAt  sql.NullTime   `db:"dispatched_at"`
	DeadlineAt    sql.NullTime   `db:"deadline_at"`
	CompletedAt   sql.NullTime   `db:"completed_at"`
	CreatedAt     sql.NullTime   `db:"created_at"`
	UpdatedAt     sql.NullTime   `db:"updated_at"`
}

func toRow(t *orcatask.Tracking) orcaTaskTrackingRow {
	row := orcaTaskTrackingRow{
		ID: t.ID, PlannerTaskID: t.PlannerTaskID, Status: string(t.Status),
		DispatchedAt: sql.NullTime{Time: t.DispatchedAt, Valid: true},
		DeadlineAt:   sql.NullTime{Time: t.DeadlineAt, Valid: true},
		CreatedAt:    sql.NullTime{Time: t.CreatedAt, Valid: true},
		UpdatedAt:    sql.NullTime{Time: t.UpdatedAt, Valid: true},
	}
	if t.PlannerJobID != "" {
		row.PlannerJobID = sql.NullString{String: t.PlannerJobID, Valid: true}
	}
	if t.OrcaTaskID != "" {
		row.OrcaTaskID = sql.NullString{String: t.OrcaTaskID, Valid: true}
	}
	if t.CompletedAt != nil {
		row.CompletedAt = sql.NullTime{Time: *t.CompletedAt, Valid: true}
	}
	return row
}

func (r orcaTaskTrackingRow) toDomain() (*orcatask.Tracking, error) {
	t := &orcatask.Tracking{
		ID: r.ID, PlannerTaskID: r.PlannerTaskID, Status: orcatask.Status(r.Status),
		DispatchedAt: r.DispatchedAt.Time, DeadlineAt: r.DeadlineAt.Time,
		CreatedAt: r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time,
	}
	if r.PlannerJobID.Valid {
		t.PlannerJobID = r.PlannerJobID.String
	}
	if r.OrcaTaskID.Valid {
		t.OrcaTaskID = r.OrcaTaskID.String
	}
	if r.CompletedAt.Valid {
		ct := r.CompletedAt.Time
		t.CompletedAt = &ct
	}
	if len(r.ResultJSON) > 0 {
		var res orcatask.Result
		if err := json.Unmarshal(r.ResultJSON, &res); err != nil {
			return nil, fmt.Errorf("unmarshaling orca_task_tracking.result: %w", err)
		}
		t.Result = &res
	}
	return t, nil
}

func marshalResult(result *orcatask.Result) ([]byte, error) {
	if result == nil {
		return nil, nil
	}
	return json.Marshal(result)
}

// OrcaTaskTrackingRepository implements orcatask.Repository.
type OrcaTaskTrackingRepository struct {
	db *sqlx.DB
}

func NewOrcaTaskTrackingRepository(db *sqlx.DB) orcatask.Repository {
	return &OrcaTaskTrackingRepository{db: db}
}

func (r *OrcaTaskTrackingRepository) Save(ctx context.Context, t *orcatask.Tracking) error {
	row := toRow(t)
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO planner.orca_task_tracking (
			id, planner_task_id, planner_job_id, orca_task_id, status,
			dispatched_at, deadline_at, created_at, updated_at
		) VALUES (
			:id, :planner_task_id, :planner_job_id, :orca_task_id, :status,
			:dispatched_at, :deadline_at, :created_at, :updated_at
		)`, row)
	if err != nil {
		return fmt.Errorf("inserting orca_task_tracking: %w", err)
	}
	return nil
}

func (r *OrcaTaskTrackingRepository) FindByPlannerTaskID(ctx context.Context, plannerTaskID string) (*orcatask.Tracking, error) {
	var row orcaTaskTrackingRow
	err := r.db.GetContext(ctx, &row,
		`SELECT * FROM planner.orca_task_tracking WHERE planner_task_id = $1`, plannerTaskID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, orcatask.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying orca_task_tracking by planner_task_id: %w", err)
	}
	return row.toDomain()
}

func (r *OrcaTaskTrackingRepository) FindByOrcaTaskID(ctx context.Context, orcaTaskID string) (*orcatask.Tracking, error) {
	var row orcaTaskTrackingRow
	err := r.db.GetContext(ctx, &row,
		`SELECT * FROM planner.orca_task_tracking WHERE orca_task_id = $1`, orcaTaskID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, orcatask.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying orca_task_tracking by orca_task_id: %w", err)
	}
	return row.toDomain()
}

func (r *OrcaTaskTrackingRepository) UpdateStatus(ctx context.Context, orcaTaskID string, status orcatask.Status, reason string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE planner.orca_task_tracking
		SET status = $2, completed_at = CASE WHEN $3 THEN NOW() ELSE completed_at END, updated_at = NOW()
		WHERE orca_task_id = $1`, orcaTaskID, string(status), status.IsTerminal())
	if err != nil {
		return fmt.Errorf("updating orca_task_tracking status: %w", err)
	}
	return nil
}

func (r *OrcaTaskTrackingRepository) UpdateResult(ctx context.Context, orcaTaskID string, status orcatask.Status, result *orcatask.Result) error {
	data, err := marshalResult(result)
	if err != nil {
		return fmt.Errorf("marshaling orca task result: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE planner.orca_task_tracking
		SET status = $2, result = $3, completed_at = NOW(), updated_at = NOW()
		WHERE orca_task_id = $1`, orcaTaskID, string(status), data)
	if err != nil {
		return fmt.Errorf("updating orca_task_tracking result: %w", err)
	}
	return nil
}
```

---

## Test File 3: `internal/infrastructure/persistence/postgres/orcatask_repo_test.go`

Dùng `github.com/DATA-DOG/go-sqlmock` hoặc testcontainers Postgres theo pattern hiện có trong các service khác của `vnp-workplace`. Test cases bắt buộc:

```go
func TestOrcaTaskTrackingRepository_Save_InsertsRow(t *testing.T)
func TestOrcaTaskTrackingRepository_FindByPlannerTaskID_NotFound_ReturnsOrcataskErrNotFound(t *testing.T)
func TestOrcaTaskTrackingRepository_FindByOrcaTaskID_Found_MapsResultJSON(t *testing.T)
func TestOrcaTaskTrackingRepository_UpdateStatus_TerminalSetsCompletedAt(t *testing.T)
func TestOrcaTaskTrackingRepository_UpdateResult_SetsResultJSONAndCompletedAt(t *testing.T)
```

---

## Verification

```bash
cd /opt/repos/vnp-workplace/backend/services/planner-service

go build ./internal/infrastructure/persistence/postgres/...
go vet ./internal/infrastructure/persistence/postgres/...
go test ./internal/infrastructure/persistence/postgres/... -v -race -cover

# Migration roundtrip (cần Postgres cục bộ hoặc testcontainers)
goose -dir migrations/planner postgres "$DATABASE_URL" up
goose -dir migrations/planner postgres "$DATABASE_URL" down
```
