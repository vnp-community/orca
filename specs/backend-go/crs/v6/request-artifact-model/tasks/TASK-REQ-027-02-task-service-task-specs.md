# TASK-REQ-027-02: `task-service`: bảng `task.task_specs`, repository, `SetTaskSpec`, `GetTaskSpecs`, `LockTaskSpecs` và chốt `TASK_SPEC_LOCKED`

**From Solution:** BE-REQ-SOL-027
**Priority:** P0
**Service:** `task-service`
**File:** `backend-go/services/task-service/migrations/{postgres,mysql}/00NN_task_specs.{up,down}.sql`, `internal/domain/task_spec.go`, `internal/usecase/{set_task_spec,get_task_specs,lock_task_specs}.go`, `internal/usecase/ports.go` (sửa), `internal/usecase/update_task.go` (sửa), `internal/adapter/{postgres,mysql}/task_specs.go`, `internal/adapter/grpc/server_task_spec.go`, `backend-go/proto/orca/task/v1/task.proto` (sửa) và test
**Depends on:** TASK-REQ-011-01 (migrations `0015`, `0016`), TASK-REQ-011-02 (`Task.RequestID`)
**Status:** [ ] TODO

---

## Context

Đã đọc: `task-service/migrations/postgres/0001_init.up.sql` (FK `REFERENCES task.tasks(id) ON DELETE CASCADE`; policy `tenant_isolation`, **không có hiệu lực thật**, BE-REQ-SOL-002 C2), `migrations/mysql/0012_task_sources.up.sql` (FK có tên `fk_task_sources_task`, `ENGINE=InnoDB`), `0013_execution_leases.up.sql`, `adapter/postgres/task_sources.go` (`LinkSource`, SQLSTATE `23505` thành lỗi domain), `internal/usecase/update_task.go` (`UpdateTaskInput{ID, Title, Status, PRURL, WorktreeID, WorkflowTemplateID, Labels}`; `UpdateTask.Execute` dòng 65), `internal/usecase/ports.go` (`TxRunner.RunInTx(fn(ctx, tasks, edges))` dòng 431), `proto/orca/task/v1/task.proto` (không có RPC spec). Thư mục `internal/adapter/grpc/` có `server.go` (661 dòng) và `server_task_source.go`: thêm RPC mới ở file riêng `server_task_spec.go` theo cách đó, không phình `server.go`.

**Số migration:** hiện cao nhất `0014_task_sources_site`; SOL-011 dùng `0015`, `0016` (và SOL-013 có thể thêm cho sự kiện). Ở đây ghi `00NN`: bắt buộc `ls migrations/postgres migrations/mysql` lúc làm và lấy số kế tiếp, cùng số cho hai dialect.

Quyết định giữ: `task_specs` là bảng riêng (không cột JSON trên `tasks`) vì `tasks` là đường đọc nóng (`ListTasks`, Board, cây), MySQL JSON không lập chỉ mục được, và `locked_at` cần rõ ràng. Tenant luôn nằm trong `WHERE`.

## Việc cần làm

1. Migration Postgres: `CREATE TABLE task.task_specs (task_id UUID PRIMARY KEY REFERENCES task.tasks(id) ON DELETE CASCADE, tenant_id UUID NOT NULL, schema_version INT NOT NULL, spec JSONB NOT NULL, digest TEXT NOT NULL, locked_at TIMESTAMPTZ NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), version BIGINT NOT NULL DEFAULT 1)`;
   - `ENABLE RLS` + policy cho đồng bộ với bảng khác
   - `CREATE INDEX idx_task_specs_tenant ON task.task_specs (tenant_id, locked_at)`.
   - MySQL: `task_id CHAR(36) PRIMARY KEY`, `spec JSON NOT NULL`, `digest CHAR(64) NOT NULL`, `locked_at TIMESTAMP(6) NULL`, `CONSTRAINT fk_task_specs_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE`.
   - Down: `DROP TABLE`.
2. `domain/task_spec.go`: `TaskSpec{TaskID, TenantID string; SchemaVersion int; Spec []byte; Digest string; LockedAt *time.Time; CreatedAt, UpdatedAt time.Time; Version int64}`;
   - `MaxTaskSpecBytes = 32 * 1024`
   - `NewTaskSpec(taskID, tenantID string, schemaVersion int, spec []byte) (TaskSpec, error)` (JSON hợp lệ bằng `json.Valid`, ≤ 32 KB, `schemaVersion >= 1`, tính `Digest` = SHA-256 hex của `CanonicalJSON` — task-service **không** import `request-service`; copy hàm canonical ngắn vào `domain/canonical_json.go` của task-service và test cùng bộ mẫu vàng với request-service ở task 027-03)
   - `(s TaskSpec) IsLocked() bool`
   - lỗi `TASK_SPEC_INVALID`, `TASK_SPEC_LOCKED`, `TASK_SPEC_VERSION_CONFLICT`, `TASK_SPEC_NOT_FOUND`.
3. `ports.go`: `TaskSpecRepository{Upsert(ctx, s TaskSpec, expectedVersion int64) (TaskSpec, error); GetMany(ctx, tenantID string, taskIDs []string) ([]TaskSpec, error); LockSubtree(ctx, tenantID string, taskIDs []string, at time.Time) (int, error); IsLocked(ctx, tenantID, taskID string) (bool, error)}`.
4. Adapter hai dialect (`adapter/postgres/task_specs.go`, `adapter/mysql/task_specs.go`): `Upsert` Postgres `INSERT ... ON CONFLICT (task_id) DO UPDATE SET spec=..., digest=..., schema_version=..., updated_at=now(), version=task_specs.version+1 WHERE task_specs.tenant_id=$n AND task_specs.version=$m AND task_specs.locked_at IS NULL`, 0 hàng thì `SELECT` phân biệt `LOCKED`/`VERSION_CONFLICT`/`NOT_FOUND`;
   - MySQL `INSERT ... ON DUPLICATE KEY UPDATE` kèm điều kiện qua `UPDATE ... WHERE version=? AND locked_at IS NULL` (không dùng `RowsAffected` của ON DUPLICATE vì không tin cậy)
   - `GetMany` dùng `IN (...)` tối đa 200 id (chia lô)
   - `LockSubtree` `UPDATE ... SET locked_at=? WHERE tenant_id=? AND task_id IN (...) AND locked_at IS NULL` và trả số hàng đổi.
5. Use case: `SetTaskSpec.Execute(ctx, in{TaskID, SchemaVersion, SpecJSON, ExpectedVersion})`: `tenant.RequireTenantID`;
   - `repo.Get` task (`TASK_NOT_FOUND`)
   - kiểm quyền `edit` bằng `ResolvePermission` như `UpdateTask` (đọc cách `UpdateTask` kiểm)
   - `NewTaskSpec`
   - `Upsert`.
   - `GetTaskSpecs.Execute(ctx, taskIDs)` (tối đa 200, quá thì `TASK_SPEC_TOO_MANY`).
   - `LockTaskSpecs.Execute(ctx, planTaskID)`: `GetSubtree(planTaskID)` (đã có, `usecase/get_subtree.go`), `LockSubtree` trên danh sách id
   - idempotent, trả `{locked int}`.
6. `update_task.go` (sửa): khi `in.Title != nil` mà `TaskSpecRepository.IsLocked(taskID)` thì `TASK_SPEC_LOCKED`;
   - `Status`, `Labels`, `PRURL`, `WorktreeID` không bị chặn (CR: trạng thái và tiến độ vẫn đổi được).
   - Nếu `UpdateTask` chưa có cổng `TaskSpecRepository`, thêm qua constructor có tham số tuỳ chọn (nil nghĩa là không chặn) để không vỡ nơi gọi `NewUpdateTask(repo, edges)` hiện có
   - liệt kê nơi gọi bằng `codegraph explore "NewUpdateTask"` trước khi sửa.
7. Proto `task.proto`: `rpc SetTaskSpec`, `rpc GetTaskSpecs`, `rpc LockTaskSpecs`;
   - message `TaskSpec{task_id, schema_version, spec_json, digest, locked, version}`, `SetTaskSpecRequest{task_id, schema_version, spec_json, expected_version}`, `GetTaskSpecsRequest{task_ids}`, `LockTaskSpecsRequest{plan_task_id}`, `LockTaskSpecsResponse{locked}`
   - số trường kế tiếp theo file thật
   - `buf generate`, `buf breaking`.
8. `server_task_spec.go`: ánh xạ lỗi sang gRPC (`TASK_SPEC_LOCKED` thành `FailedPrecondition`, `TASK_SPEC_INVALID` thành `InvalidArgument`, `TASK_SPEC_VERSION_CONFLICT` thành `Aborted`).
   - `CreatePlanTree` mở rộng `spec_json` thuộc task 027-07 (SOL-012), **không** làm ở đây.

## Kiểm thử

- Domain: `TestNewTaskSpec_Valid`, `_InvalidJSON`, `_TooLarge` (32 KB + 1), `_DigestStableAcrossKeyOrder`, `_VietnameseNFC`.
- Use case (fake repo): `TestSetTaskSpec_Locked_Rejected`, `_VersionConflict`, `_NoPermission`, `_UnknownTask`; `TestLockTaskSpecs_IdempotentAndCountsOnlyNewlyLocked`; `TestUpdateTask_TitleBlockedWhenLocked_StatusAllowed`; `TestUpdateTask_NilSpecRepo_NoBehaviourChange`.
- Integration hai dialect (`-tags=integration`, theo `repository_test.go` của hai adapter): migration up/down; FK cascade (xoá task, mất spec); CAS `Upsert` với 8 goroutine; `LockSubtree` rồi `Upsert` bị chặn; `GetMany` quá 200 id chia lô; tiếng Việt qua cột JSON; tenant A không đọc, không ghi spec tenant B.
- Hợp đồng: `buf breaking`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/task-service/internal/domain/... ./services/task-service/internal/usecase/... -run 'TaskSpec|UpdateTask' && go test -tags=integration ./services/task-service/internal/adapter/... -run TaskSpec`.

## Tiêu chí hoàn thành

- [ ] `up`/`down` sạch ở cả hai dialect; `00NN` ghi trong PR.
- [ ] Sau `LockTaskSpecs`, `SetTaskSpec` trả `TASK_SPEC_LOCKED`; `UpdateTask` đổi `status` vẫn thành công, đổi `title` bị chặn.
- [ ] `LockTaskSpecs` gọi hai lần: lần hai `locked=0`, không lỗi.
- [ ] Mọi truy vấn có `tenant_id`; test chéo tenant xanh.
- [ ] Bộ test hiện có của `task-service` không đổi kỳ vọng.

## Rủi ro và lưu ý

- Chặn `title` có thể làm hỏng luồng "đổi tên task" của người dùng sau khi Plan duyệt; phạm vi khoá còn mở (Q3 của SOL-027). Mặc định đề xuất: chặn `title` chỉ khi task thuộc cây có Plan `approved` và có dòng `task_specs` bị khoá.
- `SetTaskSpec` và `LockTaskSpecs` là RPC nội bộ giữa service; `task-service` chưa kiểm danh tính service gọi (cùng rủi ro `ReportTaskExecutionResult`). Ghi vào README, không giải quyết ở task này.
- Hai bản hàm canonical JSON (request-service, task-service) có thể lệch; test mẫu vàng dùng chung tệp trong `testdata` của hai service, kèm bước CI so sánh.
