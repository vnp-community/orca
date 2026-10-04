# CR-PW-010 — `GetStatus` (và các usecase dùng `dispatchExecutor`/`ConnectionResolver` khác) dùng worktree ID làm filesystem path khi không có connection row

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-PW-010 |
| **Tên** | `ConnectionResolver.ResolveConnection`'s `!Connected` branch trả worktree ID làm `RepoPath` — `git status`/có thể cả `GetDiff`/`History` chạy `chdir` vào 1 UUID, không phải path thật |
| **Loại** | Bug Fix |
| **Priority** | 🔴 P0 — tab Git trong Project Workspace không tải được cho MỌI worktree đi qua nhánh này |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-15 |
| **Trạng thái** | ✅ SOL-013 đã deploy + verify live (path-echo bug đã hết) — nhưng phát hiện lớp lỗi thứ 2 ngay khi verify, xem [BUG-015](../../../../specs/backend-go/bugs/missing-v2/BUG-015-dispatch-executor-always-local-never-relays-to-dev-server.md) (executor selection — chạy local thay vì relay — vẫn sai cho worktree có dev server thật) |
| **Tác giả** | Điều tra BUG-012 (`GITGATEWAY_STATUS_FAILED`), confirm bằng log thật sau khi SOL-012 (cause-logging) deploy |
| **Tác động HLD** | `git-gateway-service` (`internal/usecase/get_status.go`, `internal/adapter/grpcclient/resolver.go`, `internal/usecase/ports.go`'s `dispatchExecutor`) |
| **Tác động Features** | Project Workspace — tab Git (mọi worktree không có `infra.connections` row, tức "system-wide, zero rows" theo đúng comment code) |

---

## Bối cảnh & Vấn đề gốc

Log thật (sau khi SOL-012 deploy) bắt được chính xác:
```
git status --porcelain=v1 -b: chdir 01579d39-64c0-482f-a0e6-eeb577e404f7: no such file or directory
git status --porcelain=v1 -b: chdir 45f573e8-be1a-4b1a-894b-a7c261fb7331::/opt/repos/aiops-v3: no such file or directory
```
Cả 2 target `chdir` đều **không phải path thật** — 1 cái là UUID worktree, 1 cái là chuỗi `repoId::path` mà `mergeDetectedWorktrees` tự sinh cho worktree "external" (chỉ để hiển thị, không phải identifier thật).

Nguồn gốc — `resolver.go`:
```go
if !resp.GetConnected() {
    return usecase.ResolvedConnection{Connected: false, RepoPath: dispatchID}, nil
}
```
Khi infra-fleet-service báo "không có connection" (đúng theo comment code có sẵn: *"No infra.connections row exists for these repos — confirmed live: zero rows, system-wide"*), hàm này trả về **chính worktree ID đầu vào** làm `RepoPath` — rõ ràng là 1 giá trị placeholder, không phải path thật, nhưng `GetStatus.Execute` (`get_status.go`) truyền thẳng xuống `git status` làm `cwd` mà không validate.

`ports.go`'s `dispatchExecutorForRepo` (hàm thay thế, đã dùng cho `CreateWorktree`/`DetectWorktrees`) tự ghi rõ trong comment: **"routing these through ConnectionResolver never worked"** — nghĩa là đây là gap đã biết, nhưng `GetStatus` (và nhiều khả năng `GetDiff`/`History`/mọi usecase còn dùng `dispatchExecutor` thay vì `dispatchExecutorForRepo`) **chưa được migrate sang cơ chế đúng**.

## Giải pháp đề xuất

`GetStatusInput` hiện chỉ có `WorktreeID` — không đủ thông tin để gọi `dispatchExecutorForRepo` (cần `domain.RepoInfo`: repo URL + dev server ID). Cần 1 trong 2 hướng:

1. Thêm method mới vào `ProjectClient` interface: `GetWorktree(worktreeID) → domain.RepoInfo` (hoặc tương đương) để `GetStatus` (và các usecase cùng dạng) resolve đúng trước khi dispatch — theo đúng pattern `DetectWorktrees` đã dùng (`GetRepo` theo repo ID).
2. Hoặc sửa hẹp hơn: `ConnectionResolver.ResolveConnection`'s nhánh `!Connected` tự gọi project-service để lấy path thật thay vì echo lại input — chỉ fix cho `GetStatus`, không chắc fix hết các usecase khác cùng lỗi.

**✅ Audit đã hoàn thành (2026-09-15)** — `gitnexus impact({target: "dispatchExecutor", direction: "upstream"})` → **risk: CRITICAL, impactedCount: 38, direct: 34**. Grep xác nhận cùng danh sách 32 file usecase gọi thẳng `dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)`: `abort_rebase`, `abort_merge`, `check_ignored`, `check_worktree_delete_safety`, `checkout`, `conflict_operation`, `branch_compare`, `branch_diff`, `discard`, `merge_worktree_into_base` (x2), `fork_sync`, `stage`, `get_diff`, `commit_diff`, `list_local_branches`, `resolve_conflict`, `commit_compare`, `bulk_discard`, `remove_worktree`, `fetch`, `commit`, `get_status`, `force_delete_branch`, `history`, `push`, `pull`, `remote_commit_url`, `remote_file_url`, `unstage`, `rebase_from_base`, `fast_forward`, `submodule_status`, `upstream_status`, `compare_worktrees`. Gián tiếp (depth 2-3): `diff_composer.go`'s `gatherFullDiff`/`gatherFullDiffFromStatus`, rồi `generate_commit_message.go`/`generate_pull_request_fields.go`.

Đã thử disambiguate impact trực tiếp trên implementation cụ thể `ConnectionResolver.ResolveConnection` (`target_uid` trỏ đúng file `resolver.go`) — trả về `direct: 0, risk: LOW`, vì lời gọi thật đi qua interface (`uc.resolver.ResolveConnection(...)`, dynamic dispatch) nên gitnexus không nối được cạnh tĩnh tới implementation cụ thể. Bằng chứng đáng tin cậy là từ phía gọi (`dispatchExecutor`), không phải từ phía bị gọi — kết luận CRITICAL ở trên dựa trên số liệu đó, không phải suy đoán.

**Kết luận: KHÔNG còn là "nghi ngờ" — đây là bug hệ thống ảnh hưởng ~34 usecase**, không phải riêng `GetStatus`. Xem thiết kế fix chi tiết tại [SOL-013](../../../../specs/backend-go/bugs/missing-v2/solutions/SOL-013-connection-resolver-worktree-path-fallback.md).

## Không thuộc phạm vi CR này

- Redesign lại toàn bộ khái niệm `infra.connections` (tại sao bảng này luôn rỗng "system-wide") — có thể là 1 vấn đề sâu hơn đáng điều tra riêng, không mở rộng ở đây.

## Liên quan

- [BUG-012](../../../../specs/backend-go/bugs/missing-v2/BUG-012-gitgateway-status-failed-opaque-relay-error.md) / [SOL-012](../../../../specs/backend-go/bugs/missing-v2/solutions/SOL-012-git-gateway-service-cause-logging.md) — chính log của SOL-012 tìm ra root cause này
- [SOL-013](../../../../specs/backend-go/bugs/missing-v2/solutions/SOL-013-connection-resolver-worktree-path-fallback.md) — thiết kế fix chi tiết (chưa implement), kèm đầy đủ số liệu blast radius CRITICAL
- [CR-PW-007](./CR-PW-007-worktree-creation-and-visibility-reliability.md) — cùng khu vực code (`dispatchExecutorForRepo` vs `dispatchExecutor`), cùng nguyên nhân hệ thống
