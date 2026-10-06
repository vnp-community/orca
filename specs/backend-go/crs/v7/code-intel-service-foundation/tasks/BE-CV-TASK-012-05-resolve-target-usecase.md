# BE-CV-TASK-012-05: Use case `ResolveTarget` (thuật toán 7 bước, cache 30 s, kiểm dev server)

**From Solution:** BE-CV-SOL-012-target-resolution-and-bindings
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/{resolve_target.go,target_cache.go}` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-011-06, 012-03, 012-04
**Status:** [ ] TODO

---

## Context

SOL-012 mục 2.B. Kiểm quyền (`GetProject`) do SOL-013 gọi **trước** use case này; `ResolveTarget` nhận danh sách đã kiểm hoặc tự gọi `ListRepos`/`ListWorktrees` (cả hai đều khoá theo project+membership phía project-service). Cache khoá `(tenant, user, project, worktreeRef)`, TTL 30 s, chỉ sau quyền.

## Việc cần làm

1. `type Target struct{ ProjectID, RepoID, WorktreeID, WorkspaceRoot, DevServerID, Platform, ScopeKey, PathHash string }`; `ResolveTarget.Execute(ctx, sel domain.WorktreeSelector) (Target, domain.RepoBinding, error)`.
2. Bước 1–7 theo SOL; nhánh (b) kiểm đường dẫn theo thứ tự: `repo.url` → `Worktree.path` → `DetectWorktrees` (cache 60 s theo `(tenant, repoId)`); ngoài ba nguồn `CODEINTEL_PATH_NOT_ALLOWED`, **không** gọi agent.
3. Dev server: `ListDevServers` cache 30 s theo tenant; kiểm `kind`, `approval_status`, `mode` theo thứ tự SOL; ánh xạ lỗi PQ-03.
4. `Upsert` binding; khi đổi `workspace_root`/`dev_server_id` → đặt `index_scope='unresolved'`, xoá `gitnexus_repo`, `codegraph_path`, `SnapshotRepository.DeleteByBinding(id, "")`.
5. `(a)` không tìm thấy → `CODEINTEL_WORKTREE_NOT_FOUND` + `Delete` binding `wt:<id>` nếu có.
6. `target_cache.go`: cache TTL có đồng hồ tiêm được, kích thước tối đa (ví dụ 1024 mục, LRU hoặc xoá hết hạn), `Invalidate(tenant, project)`; `refresh=true` bỏ qua.
7. Không log đường dẫn tuyệt đối ở mức INFO (chỉ DEBUG).

## Kiểm thử

- Unit với fake cổng: mỗi nhánh lỗi 1 test (không gọi `RelayByDevServer`, cổng không có phương thức đó); bốn dạng id; path từ ba nguồn; `RebindRepoDevServer` giả lập; cache hit/miss/hết hạn/`refresh`; hai user khác nhau không dùng chung entry; tenant khác không thấy.
- `go test ./services/code-intel-service/internal/usecase/... -run ResolveTarget`

## Tiêu chí hoàn thành

- [ ] Các tiêu chí phân giải ở SOL mục 4 đạt.
- [ ] Binding đổi chỗ làm mất snapshot.

## Rủi ro và lưu ý

- Chi phí 3–4 lời gọi nội bộ mỗi lần chưa cache; theo dõi (SOL-071).
