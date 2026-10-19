# TASK-REQ-039-03: Worktree đề xuất OpenSpec dùng `base_ref` và bộ kiểm ref qua git-gateway

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/{ports_workspace,base_ref_checker}.go` (sửa), `internal/adapter/grpcclient/{proposal_workspace,git_base_ref_checker}.go` (sửa/mới), `cmd/server/wire_plan.go`/`wire_execution.go` (sửa nơi dựng `ProposalWorkspace`), nơi gọi `Ensure` (`engine_openspec.go`, `openspec_plan_builder.go`)
**Depends on:** TASK-REQ-039-02
**Status:** [ ] TODO

---

## Context

- `ProposalWorkspace.Ensure` luôn dùng `BaseRefDefault`; `EnsureWorkspaceInput` không có nhánh.
- git-gateway đã có `PrefetchCreateBase` và lỗi `WORKTREE_BASE_REF_NOT_FOUND` kèm `available`.
- Khoá idempotency `request-proposal:<id>` và nhánh `ProposalBranch(số)` phải giữ nguyên.

## Việc cần làm

1. `EnsureWorkspaceInput.BaseRef`; nơi gọi `Ensure` truyền `req.BaseRef`.
2. `Ensure`: `base := in.BaseRef`; rỗng thì `BaseRefDefault`; dùng cho `create` lần đầu, lần tạo lại khi worktree đã bị xoá (khoá mới) và `existingFor`.
3. `GitBaseRefChecker` (mới) cài `BaseRefChecker` bằng `PrefetchCreateBase`: timeout 10 giây; `NotFound` khi mã lỗi `WORKTREE_BASE_REF_NOT_FOUND` (lấy `available` làm gợi ý, tối đa 5); `Unreachable` cho `Unavailable`, `DeadlineExceeded`, dev server offline.
4. Khi tạo worktree thất bại vì ref biến mất: trả lỗi miền `base_ref_not_found` để `request.activity`/Execution hiển thị nguyên nhân (không phải lỗi chung).
5. Worktree đề xuất đã tồn tại với gốc khác `base_ref` hiện tại: giữ nguyên (đã khoá); log mức info.

## Kiểm thử

- `TestEnsure_UsesRequestBaseRef`, `_EmptyUsesDefault`, `_RecreateAfterRemovalKeepsBaseRef`, `_BaseRefGone_TypedError`.
- `TestGitBaseRefChecker_Found/NotFound/Unreachable` với client gRPC giả có độ trễ và lỗi.
- Git thật: repo tạm có `main`, `release/x`; `Ensure` với `release/x` tạo worktree có `git rev-parse HEAD` bằng SHA `release/x`; không chỉ định thì bằng SHA `main`.

## Tiêu chí hoàn thành

- [ ] Worktree đề xuất rẽ từ `base_ref` khi có, từ nhánh mặc định khi rỗng.
- [ ] Idempotency và tên nhánh không đổi.
- [ ] Lỗi ref biến mất có mã riêng, hiển thị được.

## Rủi ro và lưu ý

- `PrefetchCreateBase` gọi mạng; trên SSH có thể chậm: đo và ghi vào PR.
- Không đổi hành vi dọn worktree đề xuất (`wire_proposal_janitor.go`).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
