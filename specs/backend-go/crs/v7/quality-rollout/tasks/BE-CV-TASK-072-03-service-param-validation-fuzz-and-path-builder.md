# BE-CV-TASK-072-03: Fuzz validate tham số service và trình dựng đường dẫn không thoát root

**From Solution:** BE-CV-SOL-072
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/grpc/params_fuzz_test.go` (mới), `.../internal/usecase/repo_source_path_builder_test.go` (mới)
**Depends on:** BE-CV-TASK-072-01, BE-CV-SOL-012, BE-CV-SOL-021, BE-CV-SOL-030
**Status:** `[x] DONE`

---

## Context

- PQ-04: mọi RPC nhận `selector{project_id, worktree_ref}`; `worktree_ref` dạng `::workspace:` bị `CODEINTEL_WORKTREE_REF_UNSUPPORTED`.
- `fs.*` của agent dùng nguyên đường dẫn tuyệt đối (`fs-agent-extensions.ts:49,120`): service không được gửi đường dẫn ngoài `workspace_root`.

## Việc cần làm

1. `FuzzValidateCodeIntelParams`: với mỗi RPC (bảng sinh từ `ServiceDesc`), request ngẫu nhiên không panic, trả `CODEINTEL_INVALID_PARAMS` hoặc hợp lệ; `selector` thiếu/quá dài bị từ chối.
2. `TestPathBuilderNeverEscapesBindingRoot`: đầu vào từ UI và từ `SymbolRef.key` (vector + sinh ngẫu nhiên có hạt giống) ⇒ đường dẫn gửi cho `fs.*`/`git.*` luôn nằm trong `workspace_root` của binding; ghi lại lời gọi ở agent giả.
3. Kiểm `NormalizeRepoPath` với kết quả công cụ lạ (tệp `..`, tuyệt đối, `\`).

## Kiểm thử

- `go test ./internal/... -run 'Params|PathBuilder'`; fuzz 10 s. Không DB (binding giả).

## Tiêu chí hoàn thành

- [x] Không có đường dẫn ngoài root trong mọi lời gọi ghi lại.
- [x] Fuzz không panic.

## Rủi ro và lưu ý

- Part B/SSH chưa kiểm chứng; ghi nhận trong PR.
