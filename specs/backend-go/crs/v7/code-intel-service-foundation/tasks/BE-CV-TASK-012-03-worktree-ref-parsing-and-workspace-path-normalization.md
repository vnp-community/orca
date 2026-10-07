# BE-CV-TASK-012-03: Domain: `ParseWorktreeRef`, `NormalizeWorkspacePath`, `scope_key`, `path_hash`

**From Solution:** BE-CV-SOL-012-target-resolution-and-bindings
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/{worktree_ref.go,workspace_path.go}` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-011-04
**Status:** [x] DONE

---

## Context

PQ-04 (ba dạng ref + dạng bị từ chối), SOL-012 mục 2.B bước 1, 6 và mục 2.C. Hàm thuần, không I/O. Đường dẫn thuộc dev server (có thể khác OS) nên không dùng `path/filepath`. AGENTS.md: đa nền tảng.

## Việc cần làm

1. `ParseWorktreeRef(raw string) (WorktreeRef, error)`: `TrimSpace`, bỏ tiền tố `id:` rồi `repo:`; kiểm dài ≤ 512, không NUL/điều khiển. Loại: `RefWorktreeID` (UUID hợp lệ), `RefRepoPath` (`<repoUUID>::<path>` không chứa `::workspace:`), `RefRepoRoot` (UUID trần), còn `::workspace:` → `CODEINTEL_WORKTREE_REF_UNSUPPORTED`, khác → `CODEINTEL_INVALID_PARAMS`. Trả `{Kind, WorktreeID, RepoID, Path}`.
2. `NormalizeWorkspacePath(raw, platform string) (NormalizedPath, error)` theo SOL-012 2.C; `NormalizedPath{Display, Compare}` (`Compare` hạ chữ thường + `\` cho Windows). Lỗi: tương đối, NUL/điều khiển, `..`, > 4096 → `CODEINTEL_INVALID_PARAMS` (hoặc `PATH_NOT_ALLOWED` cho `..`, chọn một và ghi test).
3. `PathHash(p NormalizedPath) string` (SHA-256 hex của `Compare`) và `ScopeKeyForWorktree(id)`, `ScopeKeyForPath(repoID, pathHash)` (≤ 128 ký tự).
4. `SamePath(a, b NormalizedPath) bool`.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/... -run 'WorktreeRef|WorkspacePath'`
- Bảng: ref (`id:<uuid>`, `repo:<uuid>`, `<repo>::/a/b`, `<repo>::workspace:<uuid>`, rỗng, 513 ký tự, UUID sai); POSIX (`/a//b/`, `/a/./b`, `/a/../b`, `/`); Windows (`C:\Repo\`, `c:/repo`, `\\host\share\x`); tương đối; NUL; dài > 4096; Windows hai chữ hoa/thường cho cùng `Compare`.

## Tiêu chí hoàn thành

- [x] Bảng test đủ; hàm thuần không import `path/filepath`.
- [x] `scope_key` ≤ 128 ký tự mọi đầu vào hợp lệ.

## Rủi ro và lưu ý

- WSL/macOS chưa chốt (O-14); ghi chú trong code, không đoán.
