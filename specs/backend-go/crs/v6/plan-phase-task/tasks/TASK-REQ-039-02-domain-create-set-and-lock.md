# TASK-REQ-039-02: Miền `WorktreeBaseRef`, `CreateRequest`, `SetRequestBaseRef` và khoá

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/domain/{worktree_base_ref,base_ref_errors}.go` (mới), `internal/usecase/{set_request_base_ref,create_request,worktree_presence,base_ref_checker}.go` (mới/sửa), `internal/adapter/{postgres,mysql}/worktree_presence.go` (mới), `cmd/server/wire_request_lifecycle.go` (sửa), `proto/orca/request/v1/request.proto` (sửa) + mã sinh, `internal/adapter/grpc/` mapper và handler (sửa)
**Depends on:** TASK-REQ-039-01
**Status:** [ ] TODO

---

## Context

- `ChangeRequestType` là mẫu cho một ghi có khoá lạc quan kèm `ExecutionGuard`.
- Worktree đã có được nhận ra bằng `request.openspec_changes` (trừ `abandoned`) và `TaskView.WorktreeID` trong cây thực thi.
- `request.proto`: `Request` trống từ trường 34; `CreateRequestRequest` trống từ trường 9.

## Việc cần làm

1. Proto: `Request.base_ref = 34`, `Request.base_ref_locked = 35`, `Request.base_ref_lock_reason = 36` (`"proposal"|"task"|"terminal"|""`), `CreateRequestRequest.base_ref = 9`, RPC `SetRequestBaseRef`. Sinh lại mã bằng quy trình của repo; không sửa tay file sinh.
2. `domain.ParseWorktreeBaseRef` và `Request.WorktreeBaseRefOrDefault` theo SOL-039 mục 2.2; ba lỗi theo mục 2.7.
3. `WorktreePresence` (cổng) với hai cài đặt SQL; `Has(ctx, requestID) (present bool, reason string, err error)`. Request ở trạng thái cuối trả `present=true, reason="terminal"`.
4. `CreateRequest`: nhận `BaseRef`, `ParseWorktreeBaseRef`, `BaseRefChecker.Check` (fail-open khi `Unreachable`, fail-closed khi `NotFound`), lưu; kết quả mang `BaseRefUnverified`.
5. `SetRequestBaseRef`: một transaction: `GetForUpdate` → kiểm quyền → `ParseWorktreeBaseRef` → `WorktreePresence.Has` → `BaseRefChecker.Check` → `SetBaseRef`. Thành công ghi sự kiện outbox `request.base_ref_changed` (không chứa nội dung Request) và một dòng audit theo CR-REQ-035.
6. Mapper proto: `base_ref`, và `base_ref_locked`/`lock_reason` tính khi đọc (một truy vấn `WorktreePresence` cho `Get`; cho `List` dùng truy vấn gộp theo lô để không N+1).
7. Nối trong `wire_request_lifecycle.go`; thiếu cổng thì khởi động lỗi rõ (không tắt lặng lẽ).

## Kiểm thử

- Bảng `ParseWorktreeBaseRef` (≥ 30 ca, mỗi ký tự cấm một ca).
- `TestSetRequestBaseRef_Unlocked_Succeeds`, `_LockedByProposal`, `_LockedByTask`, `_Terminal`, `_VersionConflict`, `_ConcurrentWithWorktreeCreation_NoLostLock`, `_NotFoundRef_RejectedNoWrite`, `_Unreachable_StoredWithWarning`, `_PermissionDenied`.
- `TestCreateRequest_BaseRef_*` cho rỗng, hợp lệ, không hợp lệ, `NotFound`, `Unreachable`.
- Hợp đồng hai dialect cho `WorktreePresence` (có `openspec_changes`, có task có worktree, không có gì, `abandoned`).
- `go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... -run 'BaseRef|WorktreePresence' -v`.

## Tiêu chí hoàn thành

- [ ] Tạo và sửa `base_ref` đúng quy tắc khoá; không có khe giữa kiểm và ghi (test đồng thời).
- [ ] Ba mã lỗi trả đúng loại gRPC và `details`.
- [ ] `List` không N+1 (test đếm truy vấn hoặc `EXPLAIN`).
- [ ] Thiếu cổng khi khởi động thì lỗi, không tắt im lặng.

## Rủi ro và lưu ý

- `Get`/`List` thêm một truy vấn presence: đo trên Request có nhiều task.
- Chạy `gitnexus_impact` cho `CreateRequest.Execute`, `Request` (struct) và mapper.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
