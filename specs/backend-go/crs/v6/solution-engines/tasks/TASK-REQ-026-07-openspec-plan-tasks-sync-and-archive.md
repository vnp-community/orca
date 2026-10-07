# TASK-REQ-026-07: `openspecEngine.GeneratePlan`, `TasksMdSyncer` và archive sau `request.completed`

**From Solution:** BE-REQ-SOL-026
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/openspec_plan_prompt.go`, `internal/usecase/engine_openspec.go` (sửa), `internal/usecase/tasks_md_sync.go`, `internal/usecase/openspec_archive_consumer.go`, `internal/adapter/eventbus/openspec_consumers.go`, `internal/usecase/commit_plan.go` (sửa, của SOL-012) và test (mới trừ các file sửa)
**Depends on:** TASK-REQ-026-03 (parser, `TickTask`), 026-05, 026-06, TASK-REQ-012-05 (`GeneratePlan`, `CommitPlan`), TASK-REQ-013-06 (`task_run_outcomes`, consumer), TASK-REQ-001-04 (outbox, `processed_events`)
**Status:** [x] DONE

---

## Context

CR-REQ-026 mục 2.5 (Plan) và 2.6 (hai nguồn sự thật). Đọc lại lúc làm: SOL-012 mục 2.4 (PROPOSE không ghi gì; COMMIT ghi `plan_task_id`, outbox `plan.generated`, `OpenApproval`, `TransitionRequest(plan_ready)`) và SOL-013 mục 2.5 (consumer `orca.task.task.statuschanged`, bảng `task_run_outcomes(id, tenant_id, request_id, task_id, container_id, outcome, cause, ...)`).

Ý chính: Orca là nguồn chính; `tasks.md` là bản chiếu. Task thực thi **không** đọc `tasks.md` (bối cảnh do Orca dựng ở SOL-012), nên mọi lỗi đồng bộ chỉ ảnh hưởng tài liệu, không ảnh hưởng luồng. Hồ sơ `light` (bug, security, performance) bắt đầu từ Chẩn đoán `approved` của SOL-008 và không có `design.md`.

Mâu thuẫn cần xử lý: SOL-012 bước PROPOSE "không ghi gì", trong khi engine `openspec` phải ghi tệp vào worktree. Chốt: PROPOSE ghi `tasks.md` **nháp** vào worktree nhưng **chưa commit**; COMMIT (khi Orca đã tạo cây task và có id thật) render lại vùng, chèn id và `Commit`. Nháp bị `DiscardAll` nếu PROPOSE lần sau bắt đầu.

## Việc cần làm

1. `openspec_plan_prompt.go`: `BuildOpenSpecPlanPrompt(in PlanInput, profile domain.OpenSpecProfile, changeID string) string`: nêu văn phạm vùng `plan` (mẫu ở task 026-03), giới hạn của Orca (`REQUEST_PLAN_MAX_PHASES`, `REQUEST_PLAN_MAX_TASKS_PER_CONTAINER`, `REQUEST_PLAN_MAX_TASKS_TOTAL` của SOL-012), nhãn được phép (`plan_labels.go`), đầu vào là phương án đã chọn (`full`) hoặc Chẩn đoán `approved` (`light`);
   - chỉ được ghi `openspec/changes/<change_id>/tasks.md` (và `proposal.md` với `light` nếu chưa có).
2. `GeneratePlan` của `openspecEngine`: `Preflight`;
   - `Ensure`
   - `DiscardAll` bản nháp cũ
   - `ExecPrompt`
   - `CheckProposalScope`
   - `ReadFile tasks.md`
   - `ParsePlanRegion(md, "REQ-<number>")`
   - lỗi parse thì thử lại một lần với danh sách `Violation` (số dòng)
   - `Exec openspec validate <change_id>`
   - trả `(PlanProposal, PlanRaw=md)`.
   - `ValidateProposal` do use case gọi sau (không trong engine).
   - Không `Commit` ở bước này.
3. Sửa `CommitPlan` (SOL-012): sau khi `CreatePlanTree` xong và transaction ghi `plan_task_id` commit, nếu `engine=openspec` thì gọi `engine.OnPlanCommitted(ctx, req, proposal, ids)` (thêm vào giao diện hoặc một cổng `PlanProjection` riêng để không phình `SolutionEngine`): `RenderPlanRegion(ref, proposal, ids)` với `ids` là `T<p>.<q>` thành uuid task, `WriteFile tasks.md`, `Commit`, `OpenSpecChange.UpdateSync(in_sync, digest)`.
   - Best-effort: lỗi chỉ đặt `tasks_sync_state=failed`, không làm `CommitPlan` thất bại.
   - Với người dùng sửa đề xuất ở UI trước COMMIT: render từ `proposal` do client gửi (đã qua `ValidateProposal`), không từ nháp.
4. `tasks_md_sync.go`: `TasksMdSyncer.OnTaskSucceeded(ctx, requestID, taskID string) error`: đánh dấu `pending` (UPDATE `tasks_sync_state` và ghi hàng đợi tick vào bộ nhớ hoặc cột; khuyến nghị lấy danh sách task `succeeded` từ `task_run_outcomes` tại thời điểm chạy thay vì giữ hàng đợi, để không mất khi restart).
   - `Flush(ctx, requestID)` chạy theo lô: tối đa 1 lần mỗi 30 giây mỗi Request (`REQUEST_OPENSPEC_SYNC_INTERVAL`), đọc `tasks.md`, so `PlanRegionDigest` với `tasks_md_digest`
   - lệch thì ghi nhận drift (metric, audit), **dù sao** ghi đè vùng Orca
   - `TickTask` cho từng task `succeeded` (cần ánh xạ `task_id` sang `T<p>.<q>` bằng các chú thích `orca:task`), `WriteFile`, `Commit`, `UpdateSync(in_sync)`.
5. Vòng quét `RunSyncLoop(ctx, 30s)`: `ListPendingSync(limit)` rồi `Flush`;
   - `failed` được thử lại theo backoff tăng dần (tối đa 5 phút), đếm `request_openspec_sync_state{state}`.
   - Khoá hai instance bằng `FOR UPDATE SKIP LOCKED` (Postgres) và `GET_LOCK`/`SKIP LOCKED` (MySQL ≥ 8.0.1) theo mẫu SOL-013 `ReconcileExecutingRequests`
   - thêm phương thức `ClaimPendingSync` vào repository của task 026-02 nếu cần.
6. `openspec_archive_consumer.go`: consumer bền `orca.request.request.completed` (stream của request-service; `Durable: "request-service-openspec-archive"`), khử trùng bằng `processed_events(event_id)`;
   - bỏ qua Request không có `OpenSpecChange` hoặc `status!=ready`
   - `Exec openspec archive <change_id> --yes` (cờ chưa kiểm chứng) ở worktree
   - `TimedOut` hoặc `ExitCode != 0` thì thử lại backoff tối đa 5 lần rồi `tasks_sync_state=failed` (không đổi trạng thái Request đã `completed`)
   - thành công: `ChangedPaths`, `Commit`, `MarkArchived(sha)`.
   - Request `cancelled`: `Abandon()`, giữ nhánh, `Remove` worktree sau 7 ngày (hoặc theo cấu hình, vòng dọn riêng, không làm ở task này; ghi TODO ở README).
7. Đăng ký hai consumer và vòng quét ở `cmd/server/main.go` sau cờ cấu hình `REQUEST_OPENSPEC_ENABLED` (mặc định `false`); khi tắt, không có chi phí nào.

## Kiểm thử

- `TestGeneratePlan_OpenSpec_ParseAndValidate` (golden `tasks_ok.md`)
- `TestGeneratePlan_UnknownLine_RetriesWithLineNumber`
- `TestGeneratePlan_ProseCheckboxOutsideRegion_Ignored`
- `TestGeneratePlan_LightProfile_StartsFromDiagnosis`
- `TestGeneratePlan_DoesNotCommit`.
- `TestCommitPlan_RendersRegionWithTaskIds`: sau COMMIT, vùng có `orca:task T1.1 id=<uuid>`; lỗi ghi tệp không làm `CommitPlan` lỗi (`tasks_sync_state=failed`).
- `TestSync_TicksOnlySucceeded_NeverUnticks`
- `TestSync_BatchesWithin30s` (fake clock)
- `TestSync_DriftOverwritesAndCounts`
- `TestSync_FailureSetsFailedAndRetries`.
- `TestArchiveConsumer_RunsOnce_OnRedelivery` (cùng `event_id` hai lần: một lần `archive`)
- `TestArchiveConsumer_FailureDoesNotChangeRequestStatus`
- `TestArchiveConsumer_SkipsRequestWithoutChange`
- `TestArchiveConsumer_CancelledAbandons`.
- Integration hai dialect: `ClaimPendingSync` hai worker không xử lý trùng.
- Lệnh: `cd /opt/repos/orca/backend-go && go test -race ./services/request-service/internal/usecase/... -run 'TasksMd|OpenSpecPlan|Archive|CommitPlan' && go test -tags=integration ./services/request-service/internal/adapter/... -run 'PendingSync'`.

## Tiêu chí hoàn thành

- [x] Cờ `REQUEST_OPENSPEC_ENABLED=false`: không consumer nào được đăng ký (test wiring).
- [x] Task `succeeded` làm tick `[x]` đúng dòng sau tối đa một chu kỳ; không bao giờ bỏ tick.
- [x] Vùng Orca bị sửa tay: bị ghi đè và `request_engine_drift_total` tăng; văn bản ngoài vùng giữ nguyên byte.
- [x] `request.completed` giao lặp chỉ chạy `archive` một lần; lỗi archive không đổi trạng thái Request.
- [x] `CommitPlan` không thất bại vì lỗi bản chiếu.
- [x] `light` không tạo `design.md`.

## Rủi ro và lưu ý

- `openspec archive` có thể cập nhật `openspec/specs/` (gộp delta); kết quả đó nằm trên nhánh `request/<n>-proposal`, không tự merge. Chưa kiểm chứng hành vi `archive` thật; đó là lý do cờ mặc định tắt.
- Hồ sơ `light`: CR không nói rõ `proposal.md` do ai tạo khi không có Solution; task này giao agent tạo từ Chẩn đoán, cần xác nhận (ghi vào câu hỏi mở của PR).
- Chuyển ánh xạ `task_id` sang `T<p>.<q>` dựa chú thích `orca:task` do Orca chèn; nếu người dùng xoá chú thích, tick bị bỏ qua và đếm `engine_drift`.
- `Remove` worktree sau `cancelled` chưa có vòng dọn: worktree tồn tại lâu làm tăng quota dev server (ghi nợ kỹ thuật).
- `ReportTaskOutcome` có thể trễ so với trạng thái thật; bản chiếu chỉ cần nhất quán sau cùng.
