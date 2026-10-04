# CR-PW-011 — `dispatchExecutor`/inline `ResolveConnection` callers never relay to a worktree's real dev server — always run `local`, even when a live dev server is bound

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-PW-011 |
| **Tên** | Worktree-keyed git dispatch (`dispatchExecutor` + ~15 inline `ResolveConnection` callers) always resolves `Connected=false` → `local` executor, vì quyết định này CHỈ dựa vào `infra.connections` (luôn rỗng) — không bao giờ dùng `DevServerReachability` như `dispatchExecutorForRepo` đã làm |
| **Loại** | Bug Fix |
| **Priority** | 🔴 P0 — đây là điều kiện cần còn lại để tab Git hoạt động cho worktree hosted trên dev server thật (trường hợp phổ biến, không phải edge case) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-15 |
| **Trạng thái** | ✅ **Đã fix, deploy, verify live thành công** (2026-09-15, version `2026.09.15-sol014-fix`) — `grpcurl GetStatus` cho cả 2 worktree ID trong log gốc BUG-012 đều trả về kết quả `git status` thật, log `trace_id` khớp xác nhận `RelayByDevServer` → `GetStatus` đều `rpc ok`. **Đóng toàn bộ chuỗi BUG-012 → BUG-015.** |
| **Tác giả** | Live-verify CR-PW-010's fix ngay sau deploy — `chdir` vào đúng path thật nhưng vẫn fail vì path đó không tồn tại trên host của `git-gateway-service` (repo sống trên dev server khác) |
| **Tác động HLD** | `git-gateway-service` — `internal/usecase/ports.go` (`ConnectionResolver` interface + `dispatchExecutor`), `internal/adapter/grpcclient/resolver.go`, và ~46 call site (32 qua `dispatchExecutor`, ~15 gọi `ResolveConnection` trực tiếp — xem SOL-014 để biết danh sách) |
| **Tác động Features** | Project Workspace — tab Git, và mọi thao tác git khác trên worktree hosted-trên-dev-server-thật: status, diff, commit, push, pull, merge, stash, branch create/delete, copy/rename file, generate commit message/PR fields, watch files |

---

## Bối cảnh & Vấn đề gốc

Ngay sau khi deploy [SOL-013](../../../../specs/backend-go/bugs/missing-v2/solutions/SOL-013-connection-resolver-worktree-path-fallback.md) (CR-PW-010's fix — path resolution), verify live bằng `grpcurl` cho đúng worktree gây ra BUG-012:

```json
{"code":"GITGATEWAY_STATUS_FAILED","cause":"git status --porcelain=v1 -b: chdir /opt/repos/aiops-v3-golang-production-ready: no such file or directory: "}
```

Path giờ ĐÚNG (khớp `project.worktrees.path` trong DB) — nhưng vẫn fail, vì:
1. `docker inspect orca-go-git-gateway` xác nhận container này chỉ mount `/data/repos` riêng của nó — không có filesystem của dev server.
2. `project.repos` xác nhận repo này CÓ `dev_server_id` thật (`a1825a89-...`) — không phải trường hợp "chưa từng gán dev server".
3. Nhưng `dispatchExecutor` vẫn chọn `local`, vì quyết định `relay` vs `local` chỉ dựa vào `ConnectionResolver.ResolveConnection`'s `Connected` field — mà field đó chỉ true khi có row trong `infra.connections`, và bảng này **luôn rỗng, toàn hệ thống** (đã xác nhận nhiều lần trong session, cùng 1 sự thật CR-PW-010 đã nêu).

`dispatchExecutorForRepo` (dùng cho `CreateWorktree`/`DetectWorktrees`) đã tự giải quyết đúng vấn đề này từ trước — nó tự kiểm tra `repo.DevServerID` + `DevServerReachability.IsReachable(...)` độc lập với `infra.connections`, và thread `WithDevServerID(ctx, ...)` để `RelayExecutor.relay()` gọi đúng `RelayByDevServer` RPC. `dispatchExecutor` (dùng cho `GetStatus`/34 usecase khác) và ~15 usecase gọi `ResolveConnection` trực tiếp (`MergeBranch`, `StashPush`/`Pop`, `CreateBranch`, `DeleteBranch`, `CopyFile`, `RenameFile`, `ReadFileChunk`, `PushStream`/`PullStream`, `WatchWorktreeFiles`, `GenerateCommitMessage`/`GeneratePullRequestFields`, `CheckWorktreeDeleteSafety`, `RemoveWorktree`) **chưa bao giờ có cơ chế fallback này** — đúng như comment sẵn có trong `relay_executor.go`'s `relay()` đã tự ghi chú: *"dispatchExecutor's worktree-keyed callers... don't set this and still fall through to Relay with repoPath-as-connectionId; that path is currently moot... but remains a separate, still-open gap, not fixed in this pass."*

## Giải pháp đề xuất

Xem [SOL-014](../../../../specs/backend-go/bugs/missing-v2/solutions/SOL-014-connection-resolver-relay-via-dev-server-reachability.md) cho thiết kế chi tiết. Tóm tắt: mở rộng `ConnectionResolver.ResolveConnection`'s interface để trả về `context.Context` đã (có thể) được enrich `WithDevServerID`/`WithHiddenTargetID`, để `relay()` (chokepoint dùng chung) tự động route đúng — tương tự cơ chế `dispatchExecutorForRepo` đã dùng thành công, không phát minh cơ chế mới.

## Không thuộc phạm vi CR này

- **`Mode` (ssh-relay vs websocket-relay) không được resolve cho path mới này** — `DevServerReachability.IsReachable` chỉ trả `bool`, không có khái niệm Mode. Các usecase check `conn.Mode == CONNECTION_MODE_RELAY_SSH` để fail-closed trước khi merge/stash/push qua ssh-relay (SOL-PW-03) **sẽ không được bảo vệ bởi check này khi connection được resolve qua path mới** (dev-server-reachability, không qua `infra.connections`) — xem SOL-014's "Không thuộc phạm vi" để biết đầy đủ. Đây là gap TIỀM TÀNG đã có sẵn từ trước (check này chưa từng chạy thật vì `infra.connections` luôn rỗng) nhưng CR này là lần đầu tiên đường relay thật sự được kích hoạt, nên gap trở thành rủi ro thực tế — cần CR riêng để bổ sung Mode-resolution cho path mới.
- Redesign `infra.connections`/tại sao nó luôn rỗng — vẫn ngoài phạm vi, như CR-PW-010 đã nêu.

## Liên quan

- [BUG-015](../../../../specs/backend-go/bugs/missing-v2/BUG-015-dispatch-executor-always-local-never-relays-to-dev-server.md) — bug report gốc
- [CR-PW-010](./CR-PW-010-git-status-worktree-id-resolver-broken.md) / [SOL-013](../../../../specs/backend-go/bugs/missing-v2/solutions/SOL-013-connection-resolver-worktree-path-fallback.md) — lớp lỗi liền trước, cùng chuỗi điều tra
