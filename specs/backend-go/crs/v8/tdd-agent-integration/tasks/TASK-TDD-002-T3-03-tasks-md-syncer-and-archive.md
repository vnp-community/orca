# TASK-TDD-002-T3-03: TasksMdSyncer và archive thật (CR-REQ-026 TASK-07)

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.4 (bước 7)
**Checklist:** [T3.3](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** request-service (Go)
**Files:** `internal/usecase/openspec_archive_consumer.go` (stub rỗng), mới `internal/usecase/tasks_md_syncer.go`, `internal/adapter/eventbus/openspec_consumers.go`, `internal/domain/tasks_md_render.go` (`TickTask`), `cmd/server/wire_execution.go`, test đi kèm
**Depends on:** T2.3, T2.6, T2.12
**Status:** `[ ]` TODO

---

## Context

CR-REQ-026: khi task hoàn tất, đánh dấu `[x]` trong `tasks.md` trên nhánh proposal (không bao giờ bỏ tick); khi Request `completed`, chạy `openspec archive <id> --yes` trong worktree proposal và commit; lỗi thử lại tối đa 5 lần với backoff rồi `tasks_sync_state=failed` mà không ảnh hưởng `completed`; `cancelled` ⇒ `abandoned`, giữ nhánh; không merge, không mở PR. Dữ kiện đã kiểm: `OpenSpecArchiveConsumer.ConsumeRequestCompleted` rỗng; bảng `task_run_outcomes` (migration 0050) và `wire_execution.go` là nguồn sự kiện kết quả task; `openspec_changes` có `tasks_sync_state/digest/error/next_retry_at`.

## Việc cần làm

1. `TasksMdSyncer`: consumer của kết quả task (`outcome=succeeded`), gộp lô tối đa mỗi 30 s mỗi Request; `Ensure` lại worktree proposal nếu đã bị gỡ; `TickTask` đúng một dòng (id `T<p>.<q>` lấy từ ánh xạ lúc dựng Plan, T2.6); `Commit` chỉ `tasks.md`; cập nhật `tasks_sync_digest`.
2. Phát hiện lệch (drift): so `PlanRegionDigest` hiện tại với `tasks_md_digest` Orca vừa ghi; lệch ⇒ ghi đè **vùng Orca**, đếm metric `request_engine_drift_total`; không đụng văn bản ngoài vùng.
3. Lỗi: đặt `tasks_sync_state=failed`, `tasks_sync_error`, `next_retry_at`; không ảnh hưởng luồng Request.
4. `OpenSpecArchiveConsumer`: consumer bền của sự kiện Request hoàn tất (khử trùng `processed_events`); chạy `openspec archive <id> --yes` qua `DevServerExecutor.Exec` với `cwd` worktree; **mở rộng kiểm vùng ghi** cho bước này (archive di chuyển `openspec/changes/<id>` sang `openspec/changes/archive/...` và cập nhật `openspec/specs/**`); `Commit` lên nhánh proposal; `MarkArchived(commit)`.
5. Thử lại backoff tối đa 5 lần; sau đó `tasks_sync_state=failed` và cảnh báo trong bàn giao (T3.4).
6. `cancelled`: `Abandon()`, giữ nhánh, gỡ worktree (T2.12).

## Tiêu chí hoàn thành

- [ ] Task xong làm `tasks.md` trên nhánh có `[x]` đúng dòng; không bao giờ bỏ tick.
- [ ] Request `completed` ⇒ gói archive trên nhánh proposal, trạng thái `archived`; lỗi archive không làm mất `completed`.
- [ ] Không có stub nào còn lại trong consumer.

## Kiểm thử

- Test `TickTask`/syncer với workspace giả (lô, drift, lỗi), consumer archive với executor giả (thành công, lỗi, thử lại, khử trùng), test tích hợp với `openspec` thật trên repo tạm nếu có.

## Rủi ro và ghi chú

- Không bao giờ push hay mở PR.
- Phụ thuộc vào kết luận T1.8 về nơi lưu gói.
