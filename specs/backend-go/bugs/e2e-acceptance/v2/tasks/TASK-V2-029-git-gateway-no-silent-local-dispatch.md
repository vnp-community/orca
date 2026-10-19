# TASK-V2-029: git-gateway: một điểm quyết định host, không fallback im lặng về local, mã lỗi `GITGATEWAY_REPO_HOST_UNBOUND` / `GITGATEWAY_DEV_SERVER_UNREACHABLE`

**From Solution:** SOL-V2-002 (mục 3.4, 3.5) · **Resolves:** L3; "dev server ngắt trong lúc tạo worktree thì báo lỗi rõ"
**Priority:** P0 · **Service/Area:** git-gateway-service
**Files:**
- backend-go/services/git-gateway-service/internal/usecase/repo_host_dispatch.go (mới: `resolveRepoHost`, `LocalRepoPolicy`, `hostResolutionError`)
- backend-go/services/git-gateway-service/internal/usecase/ports.go (`DevServerReachability` 335; `dispatchExecutorForRepo` 673-691; `dispatchFilesystemExecutorForRepo` 699-717)
- 13 usecase gọi dispatch theo repo, đổi dòng bọc lỗi để giữ mã: create_worktree.go:80-83, detect_worktrees.go:42-45, prefetch_create_base.go:29-32, resolve_pr_base.go:35-38, resolve_mr_base.go:35-38, base_ref_default.go:38-41, check_hooks.go:45-48, get_remote_url.go:37-40, read_ephemeral_vm_recipes.go:53-56, read_issue_command.go:39-42, write_issue_command.go:35-38, scan_setup_script_imports.go:34-37, search_refs.go:38-41
- clone.go:36-56, init_repo.go:41-56 (bỏ `executor := uc.local` khi `!reachable`)
- backend-go/services/git-gateway-service/internal/adapter/grpcclient/resolver.go (`ResolveConnection` 113-176, `resolveLocal` 208-239) và relay_executor.go (~97, ánh xạ `INFRA_DEV_SERVER_NOT_CONNECTED`)
- backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:156 (`WORKTREE_CREATE_FAILED`)
- backend-go/services/git-gateway-service/internal/config/config.go (`ORCA_LOCAL_REPO_ROOT`, `GIT_GATEWAY_HOST_DISPATCH_MODE=strict|legacy`), cmd/server/main.go (98 `local := localgit.New()`, 204-235 dựng usecase)
**Depends on:** TASK-V2-031 (báo cáo repo mồ côi trước khi bật `strict` trên stack có dữ liệu thật); TASK-V2-028 (cùng biến `ORCA_LOCAL_REPO_ROOT`)
**Status:** `[x]` XONG qua TASK-V2-035 (2026-10-11), đã kiểm live; các mục dự kiến bị đơn giản hoá theo quyết định của người dùng (không có chế độ warn/legacy) ghi ở TASK-V2-035.
**Triển khai thực tế (2026-10-10, chưa commit, chưa redeploy):** [TASK-V2-035](./TASK-V2-035-repo-host-binding-and-no-silent-local.md) — phần lớn (một điểm quyết định, không fallback, 3 mã lỗi, ánh xạ relay; khác SOL: không có `strict|legacy`, dùng cờ bool)

---

## Context

`DevServerID == ""` hoặc `!reachable` đều trả executor `local` (git trong container). Mẫu sức khoẻ cập nhật 30 giây một lần nên cả repo gắn đúng host cũng bị chạy cục bộ sau khi agent rớt. Cause bị `apperrors` giấu nên client chỉ thấy mã chung.

## Việc cần làm

- [ ] `resolveRepoHost` theo sơ đồ SOL mục 3.4; mã lỗi: `GITGATEWAY_REPO_HOST_UNBOUND` (`KindFailedPrecondition`), `GITGATEWAY_DEV_SERVER_UNREACHABLE` (`KindUnavailable`), `GITGATEWAY_HOST_CHECK_FAILED` (`KindUnavailable`).
- [ ] Hai hàm dispatch gọi nó; `LocalRepoPolicy.AllowsLocal` dùng cùng quy tắc gốc local với project-service (`path.Clean`, ranh giới thư mục); gốc rỗng = không cho local.
- [ ] 13 usecase: nếu lỗi dispatch là `*AppError` thuộc ba mã trên thì trả nguyên, ngược lại bọc như cũ (một helper, một dòng mỗi nơi).
- [ ] `Clone`/`InitRepo`: host không tới được thì trả `GITGATEWAY_DEV_SERVER_UNREACHABLE`; host rỗng giữ `GITGATEWAY_MISSING_DEV_SERVER_ID` trừ khi đích dưới gốc local.
- [ ] `ConnectionResolver`: bỏ `Connected:false` cho repo có host không tới được (trả lỗi); `GetRepo` lỗi ở hai nhánh "degrades to Path-only" trả `GITGATEWAY_HOST_CHECK_FAILED`; repo không host áp `AllowsLocal`. Bao phủ 48 điểm gọi `dispatchExecutor`/`dispatchFilesystemExecutor`.
- [ ] Ánh xạ lỗi relay `INFRA_DEV_SERVER_NOT_CONNECTED` (và agent rớt) thành `GITGATEWAY_DEV_SERVER_UNREACHABLE` ở điểm `relay` chung và ở `create_worktree.go:156`, để "host ngắt giữa chừng" không thành `WORKTREE_CREATE_FAILED ... chdir`.
- [ ] `GIT_GATEWAY_HOST_DISPATCH_MODE=legacy`: hành vi cũ + log WARN (rollback không đổi nhị phân).
- [ ] Log có cấu trúc `repo_host_dispatch` (không tenant/đường dẫn đầy đủ); chưa có `/metrics` ở service này (xem 034).
- [ ] Rà `task-service/internal/adapter/grpcclient/worktree_provisioner.go` xác nhận xử lý mã mới (lỗi cuối hoặc thử lại có giới hạn, không vòng lặp vô hạn) và ghi kết quả.

## Tiêu chí hoàn thành

- Repo có host đang kết nối: relay như cũ. Host offline: `GITGATEWAY_DEV_SERVER_UNREACHABLE` (Unavailable, thử lại được). Repo không host ngoài gốc local: `GITGATEWAY_REPO_HOST_UNBOUND`. Không còn chạy git trong container cho bất kỳ trường hợp nào ngoài gốc local khai báo.
- `go build ./... && go vet ./... && go test ./...` xanh trong `services/git-gateway-service`.

## Kiểm thử

Sửa (KHÔNG xoá) các test đang khẳng định fallback: `detect_worktrees_test.go:12,53`, `dispatch_test.go:1873` (`..._LocalFallback_NeverSetsHiddenTargetID`), `read_ephemeral_vm_recipes_test.go:46`, `grpcclient_test.go:289,318`; thêm: bảng `resolveRepoHost` (host+reachable, host+unreachable, host+lỗi đọc, rỗng+trong gốc, rỗng+ngoài gốc, gốc rỗng, `legacy`), 13 usecase giữ mã lỗi, `Clone`/`InitRepo` không dùng local khi unreachable, `ConnectionResolver` (3 nhánh `resolveLocal`), ánh xạ `INFRA_DEV_SERVER_NOT_CONNECTED`. Chạy cả `dispatch_test.go`.

## Rủi ro · Ước lượng

HIGH (đổi lỗi của mọi thao tác git theo repo/worktree với dữ liệu cũ; cần 031 và `legacy` làm lưới an toàn). 2 ngày.
