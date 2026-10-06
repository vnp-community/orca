# TASK-REQ-011-06: Nối `SyncContainerStatus` vào mọi điểm con đổi trạng thái và thêm `ReconcileContainerStatuses`

**From Solution:** BE-REQ-SOL-011
**Priority:** P0
**Service:** `task-service`
**File:** `internal/usecase/update_task.go`, `internal/usecase/execute_task.go`, `internal/usecase/report_execution_result.go`, `internal/usecase/execution_lease.go`, `internal/usecase/reconcile_container_statuses.go` (mới), `internal/adapter/{postgres,mysql}/container_status.go`, `cmd/server/main.go`
**Depends on:** TASK-REQ-011-05
**Status:** `[ ] TODO`

---

## Context

Các nơi con đổi status (đã đọc, số dòng ngày 2026-10-06):
- `update_task.go`: `uc.repo.Update(...)` rồi khối un-block (dòng ~131 đến 160). Cascade tiến độ cũ chạy khi con đạt `done/cancelled`.
- `execute_task.go`: claim ở dòng 229 (`claimer.ClaimForExecution`) hoặc 236 (`UpdateStatus(in_progress)`); các hoàn tác `UpdateStatus(previousStatus)` ở dòng 250, 262, 316, 376; `CompleteExecution` ở dòng 385 (trong `dispatchDirectAgentAsync`).
- `report_execution_result.go`: `CompleteExecution` dòng 85, `ReleaseExecution` dòng 100.
- Vòng phục hồi `RecoverInterruptedExecutions.Execute` (`execution_lease.go` dòng 146) gọi `ReleaseUnlinkedInProgress` (dòng 180) là update hàng loạt, **không trả id task**, nên không gọi `Sync` trực tiếp được; cần bước đối soát riêng ở vòng quét 30 giây (`RunRecoveryLoop`, dòng 216).
- Cả ba usecase hiện nhận `repo TaskRepository` qua constructor; thêm `WithContainerSync(*SyncContainerStatus)` kiểu option như `WithExecutionLeases` để không phá chữ ký constructor.

## Việc cần làm

1. Thêm field `sync *SyncContainerStatus` và `WithContainerSync` cho `UpdateTask`, `ExecuteTask`, `ReportTaskExecutionResult` (nil = hành vi cũ). Hàm nhỏ `syncParent(ctx, id)` log lỗi, không làm hỏng lệnh gốc (best-effort, như `RecalculateProgress`).
2. `UpdateTask`: gọi `syncParent` sau `repo.Update` thành công và sau bước un-block, bất kể `reachedTerminal` (giữ nguyên điều kiện cũ cho cascade tiến độ).
3. `ExecuteTask`: gọi sau claim thành công và ở bốn nhánh hoàn tác; `dispatchDirectAgentAsync` gọi sau `CompleteExecution` và sau hoàn tác.
4. `ReportTaskExecutionResult`: gọi ở cả nhánh thành công lẫn thất bại.
5. `reconcile_container_statuses.go`: `ReconcileContainerStatuses.Execute(ctx) (int, error)` dùng cổng `ContainerReconcileRepository` mới: `ListStaleContainers(ctx, limit int) ([]domain.Task, error)` chọn container có `updated_at < MAX(child.updated_at)` (Postgres `FOR UPDATE SKIP LOCKED`, MySQL ≥ 8.0.1 cũng `SKIP LOCKED`). Với mỗi container gọi `SyncContainerStatus.Execute(firstChildID)` hoặc hàm `SyncOne(containerID)` mới; nếu không đổi thì "chạm" `updated_at` để khỏi quét lại.
6. Nối bước đối soát vào `RunRecoveryLoop` (cùng nhịp 30 giây), tách hàm để test.
7. `cmd/server/main.go`: dựng `SyncContainerStatus` một lần và truyền vào ba usecase; chạy `gitnexus_impact` trên `NewUpdateTask`, `NewExecuteTask`, `NewReportTaskExecutionResult` trước khi sửa.

## Kiểm thử

- Unit: `TestUpdateTask_SyncsParentContainer`, `TestExecuteTask_ClaimSyncsPhase_InProgress`, `TestExecuteTask_RevertSyncsPhase_BackToOpen`, `TestReportExecutionResult_SuccessSyncsPhase_Review`, `TestReconcile_TouchesWhenUnchanged`, `TestSyncFailure_DoesNotFailUpdateTask`.
- Integration hai dialect: vòng đời `open → in_progress → review → done` của một task lá làm phase và plan đổi theo, mỗi lần đổi có đúng một dòng outbox `statuschanged` với `cause=derived` (đếm `task.outbox_events`); `TestReconcile_ReleaseUnlinked_FixesPhase`.
- `cd /opt/repos/orca/backend-go && go test ./services/task-service/internal/usecase/... && go test -tags=integration ./services/task-service/internal/adapter/...`.

## Tiêu chí hoàn thành

- [ ] Con đổi qua `ExecuteTask`, `ReportTaskExecutionResult`, `UpdateTask` thì phase và plan đổi theo.
- [ ] Mỗi lần đổi container có đúng một dòng outbox `statuschanged`, `cause=derived`, có `task_type`.
- [ ] Sau `ReleaseUnlinkedInProgress` đặt task lá về `open`, vòng đối soát đưa phase về đúng trạng thái trong một chu kỳ.
- [ ] Constructor cũ vẫn biên dịch; test hiện có không đổi.
- [ ] Lỗi `Sync` chỉ ghi log, không làm lệnh gốc thất bại.

## Rủi ro và lưu ý

- Gọi `Sync` ngoài transaction của lệnh gốc: có cửa sổ ngắn trạng thái container cũ; đối soát là lưới an toàn.
- Tải: mỗi đổi trạng thái task lá thêm 1 đến 3 truy vấn; chỉ khi parent là container (task thường dừng sớm ở bước 1).
- BE-REQ-SOL-013 sẽ thêm sự kiện cho chính task lá ở cùng điểm gọi; tránh trùng sửa một khối: ghi chú cho người làm TASK-REQ-013-02.
