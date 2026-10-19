# TASK-V2-027: `git.status` của worktree task trả danh sách rỗng (gateway đọc sai `entries` của agent)

**From Solution:** (gap G5c của TASK-FE-E2E-011, agent N; G5 sửa một nửa ở frontend bởi TASK-FE-E2E-014, agent T) · **Resolves:** G5c, mã gap `UI-NO-WORKTREE-VIEW`
**Priority:** P1 · **Service/Area:** git-gateway-service, api-gateway (wscompat), proto gitgateway
**Files:**
- backend-go/proto/orca/gitgateway/v1/gitgateway.proto (+ mã sinh `gitgateway.pb.go`): chỉ cộng trường
- backend-go/services/git-gateway-service/internal/domain/domain.go (`FileStatus`, `StatusUpstream`, `GitStatus`, `FileStateCopied`)
- backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_git_status_wire.go (mới), relay_executor.go (`GetStatus`, `SubmoduleStatus`) (+ `relay_executor_git_status_test.go` mới, `grpcclient_test.go`)
- backend-go/services/git-gateway-service/internal/adapter/grpc/status_mapping.go (mới) + test, server.go
- backend-go/services/git-gateway-service/internal/adapter/localgit/porcelain_v1_status.go (mới) + test, executor.go, executor_test.go
- backend-go/services/git-gateway-service/internal/usecase/check_worktree_delete_safety.go (+ test)
- backend-go/services/api-gateway/internal/adapter/wscompat/channels_git_status_view.go (mới) + test; `channels.go` chỉ bị xoá khối view cũ
**Depends on:** TASK-FE-E2E-011 (G5c), TASK-FE-E2E-014 (nửa frontend của G5)
**Status:** `[x]` XONG và đã kiểm trên stack thật (2026-10-10): `git.status` của worktree task trả `src/calc.py` và `tests/test_calc.py`; còn G5d/G5e (diff, ahead/behind) chưa sửa.

---

## Context

Sau khi agent Claude thật sửa file trong worktree của task, "Open in Git" điều hướng đúng sang Source Control của worktree nhưng `git.status {"worktree":"id:<id>"}` trả `{"branch":"refs/heads/task/<taskId>","entries":[]}` dù trên đĩa có 2 file chưa commit (`src/calc.py` sửa, `tests/test_calc.py` mới), 0 commit ahead. Source Control hiện ô Message và "Publish Branch" nhưng 0 dòng file.

## Gốc rễ THẬT

Không phải cwd, không phải `/var` so với `/private/var`, không phải tham số. Chuỗi gọi chạy đúng thư mục worktree (bằng chứng ngay trong dữ liệu của N: `branch` trả về là nhánh `task/<id>` của worktree, nếu chạy ở repo chính thì phải là nhánh chính). Lỗi là **gateway giải mã sai đáp án của agent**:

- Agent (`getStatusOp`, `agent/src/relay/git-handler-status-ops.ts`) trả `{ entries: [{path,status,area,oldPath?,added?,removed?}], conflictOperation, head, branch: "refs/heads/<nhánh>", upstreamStatus }`.
- `grpcclient.RelayExecutor.GetStatus` giải thẳng JSON đó vào `domain.GitStatus{Branch, Files}`. `branch` khớp tên nên có, còn `entries` không có trường nào trong struct nên bị bỏ im lặng (Go `json.Unmarshal` không báo lỗi khi thiếu trường). Kết quả: `Files` rỗng với MỌI worktree chạy qua relay (dev server). Test cũ `TestRelayExecutor_GetStatus_Success` dùng fixture tự bịa `{"branch","files"}` nên không bao giờ bắt được.
- Cùng gốc rễ cho `git.submoduleStatus` (cùng kiểu trả về).
- Hệ quả nặng hơn Source Control trống: `CheckWorktreeDeleteSafety` đọc `status.Files`, nên với worktree relay nó luôn báo "sạch" (`SafeToDelete=true` dù có file chưa commit), và `GenerateCommitMessage`/`gatherFullDiffFromStatus` thấy không có thay đổi nào.
- Lớp view của api-gateway (`toGitStatusResultView`) còn gán cứng `area:"unstaged"` cho mọi dòng và bỏ `oldPath`, `added/removed`, `conflictOperation`, `head`, `upstreamStatus`, nên dù sửa phần decode, các file staged/untracked vẫn bị gom sai nhóm.

Đường host-local (`localgit`, khi repo không gắn dev server) có hai lỗi riêng cùng triệu chứng: `git status --porcelain=v1 -b` mặc định gộp thư mục untracked mới thành một dòng `?? tests/` (không phải `tests/test_calc.py`), và `AM`/`RM` bị gộp thành một dòng một trạng thái.

### Bằng chứng (đỏ trước, xanh sau)

1. Chạy chính `getStatusOp` của agent (esbuild bundle trong scratchpad) trên repo tạm thật + `git worktree add -b task/abc <repo>-task-abc` (realpath `/private/var/...`) có file sửa, file đã stage rồi sửa tiếp, file đổi tên, file untracked: trả đúng dạng `entries` ở trên, `branch:"refs/heads/task/abc"`. Payload này được dùng nguyên văn làm fixture.
2. `TestRelayExecutor_GetStatus_DecodesRealAgentEntries` ĐỎ trên mã cũ: `expected the agent's 5 entries, got 0`; XANH sau sửa.
3. `git status --porcelain=v1 -b` (lệnh cũ của localgit) trên cùng worktree in `RM src/calc.py -> src/calc2.py`, `AM staged.txt`, `?? tests/`: xác nhận lỗi gộp thư mục.
4. Đã loại: cwd sai (branch đúng chứng minh), realpath (git luôn in realpath; test `ViaSymlinkedPath` chạy `GetStatus` qua đường dẫn symlink, xanh), lọc "tracked" ở agent (agent dùng `--untracked-files=all`).

## Thay đổi

1. **proto (chỉ cộng, `buf lint` 307, `buf breaking` xanh so với `main`)**: `FileStatus` thêm `area, old_path, conflict_kind, conflict_status, optional added, optional removed` (3-8); message mới `StatusUpstream`; `GetStatusResponse` thêm `conflict_operation, head, upstream, did_hit_limit, status_length` (3-7). Sinh bằng `buf generate --path orca/gitgateway`, header giữ `protoc-gen-go v1.36.11 / protoc (unknown)` như tệp gốc; không đụng mã sinh của task/infrafleet.
2. **grpcclient**: `gitStatusWire` giải đúng dạng agent (con trỏ cho `added/removed` vì agent bỏ trường cho file binary và untracked chỉ có `added`), `GetStatus` và `SubmoduleStatus` dùng nó.
3. **domain + grpc adapter**: `domain.GitStatus` mang đủ trường; `toProtoGitStatus` chuyển hết (một chỗ cho cả `GetStatus` lẫn `SubmoduleStatus`).
4. **localgit**: `-c core.quotePath=false status --porcelain=v1 -b --untracked-files=all` (mọi cờ có trước Git 2.25, không cần `GitCapabilityCache`); một dòng `XY` thành một dòng mỗi vùng (staged/unstaged) như agent; `old -> new` tách `OldPath`; mã xung đột có `conflictKind` + `conflictStatus=unresolved`.
5. **api-gateway**: view mới trả `entries` (đủ trường), `conflictOperation` (mặc định `unknown`), `head`, `branch`, `upstreamStatus`, `didHitLimit/statusLength`. `area` rỗng suy ra (`untracked` nếu trạng thái untracked, còn lại `unstaged`); `conflicted` của executor cục bộ đổi thành `modified` vì `GitFileStatus` của frontend không có giá trị đó.
6. **usecase**: `CheckWorktreeDeleteSafety` đếm theo path (một path staged + unstaged là một file).
7. **Test cũ sửa đúng gốc**: `initRepo` (localgit) dùng `filepath.EvalSymlinks(t.TempDir())`. Đây là gốc rễ của `TestListWorktreePaths_*` (lỗi có sẵn trên main): git luôn in realpath (`/private/var/...`), test so với đường dẫn symlink (`/var/...`). Mã sản xuất của `ListWorktreePaths` đúng, không đổi.

## Tính đúng của "chạy trên máy nào"

`GetStatus` giữ nguyên cách điều phối: `dispatchExecutor` -> relay qua dev server khi repo gắn dev server tới được, nếu không thì `localgit` trên máy git-gateway. Tôi không đổi chỗ chạy git. Quyền/tenant không đổi (cùng `withTenantMetadata`, cùng handler).

## Rà các kênh git khác (yêu cầu 3)

Cách giải `worktree:id:` KHÔNG có lỗi cwd: mọi kênh `git.*` dùng cùng `stripWorktreeSelectorPrefix` rồi `ConnectionResolver`, đường dẫn là `wt.Path` của project-service. Đã sửa cùng gốc rễ "decode sai đáp án agent" cho `git.submoduleStatus`. Phát hiện thêm, KHÔNG sửa ở task này (khác gốc, cần thiết kế hợp đồng; ghi cho người điều phối):

- **G5d `[ ]` (api-gateway + git-gateway) các kênh trả proto thô qua `encoding/json`** nên khoá là snake_case, frontend đọc camelCase. Đã đo: `json.Marshal(UpstreamStatusResponse{HasUpstream:true})` ra `{"has_upstream":true,"ahead":1}` (frontend cần `hasUpstream`); `git.diff` ra `{"unified_diff":"..."}` trong khi frontend cần `GitDiffResult {kind, originalContent, modifiedContent}`. Ảnh hưởng `git.upstreamStatus`, `git.diff`, `git.commitCompare/branchCompare/commitDiff/branchDiff`, `git.history`, `git.forkSync`. Với "Open in Git" thì danh sách file đã hiện (task này), nhưng bấm vào file để xem diff và nhãn "Publish Branch"/ahead-behind vẫn nhiều khả năng sai.
- **G5e `[ ]` (git-gateway) `RelayExecutor.GetDiff`** giải đáp án `git.diff` của agent (`{kind, originalContent, modifiedContent, ...}`) vào `DiffResult{UnifiedDiff}` nên luôn rỗng qua relay; `Commit` mong `CommitSHA` nhưng agent trả `{success:true}` (SHA luôn rỗng qua relay). Cùng họ lỗi decode với G5c, cần quyết định hợp đồng diff (proto `GetDiffResponse`) trước khi sửa.
- `branch` của relay là `refs/heads/<nhánh>` còn của localgit là tên ngắn (`main`): đã có từ trước, frontend chịu được cả hai (chưa kiểm bằng UI), không đổi.
- Tài liệu `docs/reference/git-compatibility.md` mà AGENTS.md dẫn tới không tồn tại trong cây hiện tại.

## Kiểm

Môi trường: `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock`, `TESTCONTAINERS_RYUK_DISABLED=true`, `buf` ở scratchpad.

| Lệnh | Kết quả |
|---|---|
| `go build ./... && go vet ./... && go test ./...` trong `services/git-gateway-service` | xanh, kể cả 3 `TestListWorktreePaths_*` (trước: đỏ có sẵn) |
| cùng ba lệnh trong `services/api-gateway`, `request-service`, `task-service`, `workflow-service` (đều import gitgateway proto) | xanh |
| `proto`: `go build/vet/test ./...` | xanh |
| `buf lint` / `buf breaking --against '../../.git#branch=main,subdir=backend-go/proto'` | 307 (như cũ) / xanh |
| test localgit trạng thái (`GetStatus`, `ParsePorcelain`, `Stage`, `ListWorktreePaths`) với Git 2.54.0 (Apple) và Git 2.51.1 (Homebrew) | xanh cả hai |
| `gofmt -l` các tệp tôi đụng | sạch |

Test mới đáng chú ý: `TestRelayExecutor_GetStatus_DecodesRealAgentEntries` (+ `KeepsAreaOldPathLineStatsAndUpstream`, `UnmergedRowKeepsConflictKind`, `SubmoduleStatus_DecodesAgentEntries`), `TestGetStatus_LinkedTaskWorktree_ReportsEditedAndNewFileInNewDir`, `..._ViaSymlinkedPath`, `TestGetStatus_StagedThenEditedYieldsOneRowPerArea`, `TestGetStatus_RenameKeepsOldPath`, `TestToProtoGitStatus_*`, `TestGitStatusChannel_PreservesAreasAndStatusSummary`, `TestCheckWorktreeDeleteSafety_PathInBothAreasCountsOnce`. Chưa có test chạy tay agent Node thật trong CI Go; fixture là đầu ra thật đã chụp.

## Blast radius (codegraph)

`toGitStatusResultView` -> `registerGitChannels` -> `RegisterRealChannels` (api-gateway), thêm công cụ MCP `git_status` dùng cùng kênh; `parsePorcelainStatus` <- `localgit.GetStatus`, `localgit.SubmoduleStatus`; `RelayExecutor.GetStatus` <- usecase `GetStatus`, `CheckWorktreeDeleteSafety`, `GenerateCommitMessage` (qua `gatherFullDiffFromStatus`), `RemoveWorktree`. Hành vi đổi có chủ đích: các luồng này giờ thấy thay đổi thật qua relay (đặc biệt kiểm tra an toàn khi xoá worktree sẽ chặn xoá khi còn file chưa commit).

## Việc còn lại / cần người điều phối

- Build lại và khởi động lại **git-gateway-service** và **api-gateway** (proto đổi, cả hai nhận trường mới; nhận/gửi tương thích ngược vì chỉ cộng). KHÔNG cần đổi infra-fleet-service hay agent.
- Chạy lại `--real monitor`: sau "Open in Git" phải có dòng `source-control-entry` cho `src/calc.py` (unstaged) và `tests/test_calc.py` (untracked).
- G5d/G5e ở trên (diff và upstream) là nơi gap tiếp theo nhiều khả năng xuất hiện khi người dùng bấm vào file.
