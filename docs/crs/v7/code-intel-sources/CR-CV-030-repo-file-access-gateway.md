# CR-CV-030 — Cổng đọc file repo qua `fs.*` / `git.*` của agent

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-030 |
| **Tên** | Cổng `RepoSourceReader` ở `code-intel-service`: đọc thư mục, file, lịch sử và diff của repo trên dev server qua `RelayByDevServer` với giới hạn kích thước, bỏ file nhị phân, an toàn đường dẫn phía backend và cache theo nội dung |
| **Loại** | Feature (adapter + cổng usecase, không đổi agent) |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-010 (module `code-intel-service`), CR-CV-012 (`repo_bindings.workspace_root`, `dev_server_id`), CR-CV-021 (client gRPC tới `infra-fleet-service`; nếu chưa merge thì CR này tự khai báo cổng `AgentRelay` tối thiểu, CR-CV-021 dùng lại) |
| **Mở khoá** | CR-CV-031, 032, 033, 034, 035, 036 (đọc diff), 037 (`git log`) |
| **Tác động** | `backend-go/services/code-intel-service/internal/usecase` (cổng `RepoSourceReader`, `AgentRelay`), `internal/adapter/agentrepofs` (mới), `internal/config` (hạn mức đọc). Không sửa `agent/`, không sửa `infra-fleet-service` |
| **Phụ thuộc dữ liệu ngoài** | E1–E4, E8, E10 (đọc file qua cổng này); E5 (lịch sử git, khoảng thời gian mặc định 90 ngày); E12 (`CODEOWNERS`, chưa xác nhận có file); E16 (do CR-CV-012) |

---

## 1. Bối cảnh và vấn đề

README v7 D6 chốt: các view C4, ERD, lưu trữ, hợp đồng đọc file nhỏ qua RPC `fs.*`/`git.*` đã có của agent và parse ở `code-intel-service`. Cần một cổng duy nhất để mọi CR 031–037 đọc repo, vì khảo sát code (2026-10-05) cho thấy các RPC này có nhiều điểm không an toàn nếu gọi thẳng.

Đã đọc code, kết quả:

1. **Đường mà MVP đi (D2, `direct-websocket`) là Part A của agent**, router `agent/src/relay/agent-rpc-dispatch*.ts`. Backend đi `InfraFleetService.RelayByDevServer` (`backend-go/services/infra-fleet-service/internal/usecase/relay_by_dev_server.go`): kiểm tra tenant sở hữu dev server, `IsConnected`, rồi `agent.Exec`; lỗi bất kỳ của `Exec` bị gói thành `INFRA_AGENT_EXEC_FAILED` (`KindInternal`). Timeout mỗi cuộc gọi là `RequestTimeout = 30s` (`devserveragent/config.go:57`), chỉ riêng `agent.execPrompt` được nới (`client.go:412`). Khung WS tối đa `MaxMessageSize = 16 MiB` (`devserveragent/frame.go:24`).
2. **Bản catalog `specs/agent/api/agent-rpc-catalog-git-fs.md` đã cũ so với code**: Part A hiện còn có `git.status`, `git.diff`, `git.stage`, `git.fetch`, `git.upstreamStatus`, … (`agent-rpc-dispatch-git-status.ts`), không chỉ `git.exec`/`git.history`/`*Compare`/`*Diff`. Tên và tham số dưới đây lấy từ code, không từ catalog.
3. **`fs.*` Part A không có giới hạn đường dẫn khi đọc.** `handleFsReadDir`, `handleFsReadFile`, `handleFsStat` (`agent/src/relay/fs-agent-extensions.ts`) dùng `isAbsolute(raw) ? raw : join(workDir, raw)`; `handleFsGlob` dùng `cwd` thẳng; `handleFsGrep` dùng `root` thẳng. Chữ "SecureFs" chỉ xuất hiện ở `handleFsWriteFile` (`fs-agent-write-extensions.ts:43`), nơi duy nhất kiểm tra nằm trong `workDir`. Hệ quả: **toàn bộ việc chặn đọc ngoài workspace phải làm ở backend**.
4. **`git.exec` Part A** (`agent-git-handler.ts`): whitelist 21 subcommand (`status, diff, add, restore, commit, push, pull, fetch, branch, checkout, merge, rebase, stash, log, worktree, remote, tag, show, rev-parse, config, describe, shortlog`) và regex `SHELL_METACHARACTERS = /[&|;$\`<>\\!]/` trên mọi tham số. **Không có** `ls-tree`, `cat-file`, `ls-files`, `merge-base`, `for-each-ref`, `blame`. Cờ đứng trước subcommand bị từ chối (`assertNoGitInjectionFlags`), nên **không dùng được `-c core.quotePath=false`**; tên file không ASCII sẽ bị git trích dẫn kiểu bát phân trừ khi dùng `-z`. Tham số `timeout` bị chặn ở 60 000 ms; stdout/stderr gom bằng `chunk.toString()` từng khúc, **không có giới hạn dung lượng** và có thể cắt đôi ký tự UTF-8 ở ranh giới khúc.
5. **`fs.readFile`** gọi `readRelayFileContent` (`fs-handler-file-read.ts`): trần 10 MiB cho văn bản (`MAX_TEXT_FILE_SIZE`), 50 MiB cho ảnh; file nhị phân (dò NUL) trả `{content:"", isBinary:true}`; quá cỡ trả lỗi `InvalidParams` với message `FILE_TOO_LARGE`. Tham số `maxBytes` mà `git-gateway-service/.../relay_executor.go:1127` gửi **bị agent bỏ qua** (không có trong handler), nên không dựa vào nó.
6. **`fs.readDir`** (`depth` kẹp ≤ 5, mặc định 1) không có danh sách bỏ qua và không giới hạn số mục: `depth` lớn trên `node_modules` là rủi ro. Mục là symlink có `type: "file"` và **`size` không có** (`entry.isFile()` sai cho symlink nên `size: undefined`, `fs-agent-extensions.ts` hàm `readDirRecursive`). Đây là dấu hiệu dùng để loại symlink. `fs.stat` dùng `stat()` (theo symlink) nên `isLink` luôn sai, không dùng để phát hiện symlink.
7. Lỗi của agent (`{error:{code,message}}`) có thể về tới backend như một kết quả bình thường. `git-gateway-service/internal/adapter/grpcclient/relay_executor.go:128-148` kiểm tra thủ công trường `error` ở mọi cuộc gọi vì từng bị nuốt lỗi. Cổng mới phải làm tương tự (chưa tự kiểm chứng đường đi chính xác của envelope; xem mục 6).
8. Lệch đã quan sát ở consumer hiện có (không sửa ở đây, không sao chép): `relay_executor.go` `History` đọc `result.commits` trong khi `git.history` của agent trả `items` (`agent/src/shared/git-history.ts`); `Search` gửi `repoPath/isRegex/pathGlob` trong khi `fs.grep` đọc `root/pattern/maxResults`. Vì vậy **không tái dùng `RelayExecutor` của git-gateway** mà viết adapter riêng, đọc lại tham số từ code agent.

## 2. Giải pháp đề xuất

### 2.1 Cổng usecase (mới)

`internal/usecase/repo_source_reader.go` (mới). Tên file theo khái niệm, không `helpers`/`utils`.

```go
// RepoRef lấy từ repo_bindings (CR-CV-012), không bao giờ từ client.
type RepoRef struct{ TenantID, DevServerID, WorkspaceRoot string }

// SourceRef: đọc working tree hay đọc một commit.
type SourceRef struct{ Kind string /* "worktree" | "commit" */; Commit string }

type RepoSourceReader interface {
    ResolveHead(ctx, RepoRef) (commit string, err error)
    DirtyPaths(ctx, RepoRef, scope []string) (map[string]struct{}, error)
    ListDir(ctx, RepoRef, relDir string) ([]DirEntry, error)              // 1 cấp, đã lọc
    ReadFile(ctx, RepoRef, SourceRef, relPath string) (FileContent, error)
    ReadFiles(ctx, RepoRef, SourceRef, relPaths []string) ([]FileContent, ReadReport, error)
    BlobIDs(ctx, RepoRef, commit string, relPaths []string) (map[string]string, error)
    ChangedFiles(ctx, RepoRef, baseRef string) (BranchCompare, error)     // merge-base..HEAD
    FileDiff(ctx, RepoRef, baseRef, relPath string) (DiffPair, error)
    Log(ctx, RepoRef, LogQuery) ([]CommitRecord, error)
}

type FileContent struct{ Path string; Size int64; Content []byte; ContentHash string; Skipped string /* "", "binary", "too_large", "symlink_or_special", "not_allowed" */ }
type ReadReport struct{ Requested, Read, SkippedBinary, SkippedTooLarge, SkippedOther int; Truncated bool }
```

`AgentRelay` (cổng thứ hai, thuộc CR-CV-021 nếu đã có): `Call(ctx, devServerID, method string, params map[string]any) (json.RawMessage, error)`; cài bằng `infrafleetv1.InfraFleetServiceClient.RelayByDevServer` (`ParamsJson`/`ResultJson`), kèm metadata tenant như `withTenantMetadata` của git-gateway. Adapter `internal/adapter/agentrepofs` (mới) chỉ biết tên method agent, các lớp trên không biết.

### 2.2 Ánh xạ sang method agent (Part A, đã đọc code)

| Hàm cổng | Method + tham số | Kết quả dùng | Ghi chú |
|---|---|---|---|
| `ListDir` | `fs.readDir {path: <abs>, depth: 1}` | `entries[{name,path,type,size?}]` | Luôn `depth:1`, tự đệ quy ở backend để áp bộ lọc. Loại mục `type=="file"` mà `size` vắng (symlink/đặc biệt) |
| `ReadFile` (worktree) | `fs.readFile {path: <abs>}` | `{content, encoding:"utf-8"\|"base64", isBinary}` | Chỉ gọi sau khi `ListDir` đã xác nhận file thường và `size ≤ cap` |
| `ReadFile` (commit) | `git.exec {args:["show","<commit>:<rel>"], cwd:<root>, timeout}` | `{stdout}` | Không theo symlink, không đọc ngoài repo. `<commit>` phải là 40/64 hex đã qua `ResolveHead`/`rev-parse` |
| `ResolveHead` | `git.exec {args:["rev-parse","--verify","HEAD"]}` | `stdout` | |
| `BlobIDs` | `git.exec {args:["rev-parse","<c>:<p1>","<c>:<p2>",…]}` | mỗi dòng một oid | Chia lô 100 đường dẫn (giới hạn độ dài dòng lệnh trên Windows) |
| `DirtyPaths` | `git.status {worktreePath, limit:0}` | `entries[]` | `limit:0` nghĩa không giới hạn theo `git-handler-status-ops.ts` (`limit !== 0 && …`). Hình dạng phần tử `entries` chưa đọc kỹ, cần đọc `getStatusOp` trước khi cài |
| `ChangedFiles` | `git.branchCompare {worktreePath, baseRef}` | `{summary{baseOid,headOid,mergeBase,changedFiles,commitsAhead,status}, entries[{path,status,oldPath?,added?,removed?}]}` | `baseRef` không bắt đầu bằng `-`. `status` khác `ready` thì trả lỗi có mã `invalid-base`/`unborn-head`/`no-merge-base` |
| `FileDiff` | `git.branchDiff {worktreePath, baseRef, includePatch:true, filePath, oldPath?}` | mảng `{originalContent, modifiedContent, originalIsBinary, modifiedIsBinary, …}` | Cho hai phía nội dung, không phải patch unified; đủ cho đối chiếu ERD/proto hai commit |
| `Log` | `git.exec {args:["log","--format=…","-z","--no-merges","--since=<n>.days","-n<N>","--", <pathspec>]}` | `stdout` | Xem 2.5 |

Quy ước tham số: mọi đường dẫn gửi agent là **tuyệt đối** (`WorkspaceRoot + "/" + rel`), vì agent nối với `AGENT_WORK_DIR` (mặc định `process.cwd()`, `agent-config.ts:91`) khi tương đối. `git.history` có sẵn (`{items, hasMore, limit}`, mặc định 50, tối đa 200) nhưng chỉ cho lịch sử HEAD, không có đường dẫn từng file; hotspot (CR-CV-037) dùng `git.exec log`.

Method bị cấm gọi từ cổng này: mọi `fs.write*/mkdir/rmdir/copyFile`, `git.exec` có subcommand ngoài `{log, show, rev-parse, diff, status}`, `git.commit/push/pull/fetch/checkout/stage/...`. Cổng chỉ đọc; whitelist đặt ở một bảng hằng trong adapter và có test khẳng định.

### 2.3 An toàn đường dẫn (phía backend)

1. `WorkspaceRoot` lấy từ `repo_bindings`; so khớp theo O4 (CR-CV-012). Client chỉ gửi đường dẫn **tương đối**.
2. `CleanRel(p)`: từ chối rỗng, tuyệt đối, chứa NUL, chứa `..` sau `path.Clean`, bắt đầu bằng `-`. Trả `CODEINTEL_PATH_NOT_ALLOWED`.
3. Chỉ đọc đường dẫn mà **mọi cấp tổ tiên đã xuất hiện trong kết quả `ListDir`** (cache theo phiên đọc): chặn symlink thư mục dẫn ra ngoài vì symlink không bao giờ được đi vào. Đường dẫn cố định (ví dụ `go.mod`) cũng đi qua `ListDir` của thư mục cha.
4. Danh sách đuôi/tên cho phép khi liệt kê file nguồn: `.sql`, `.proto`, `.go`, `.yaml`, `.yml`, `.json`, `.toml`, `.md`, `Dockerfile`, `CODEOWNERS`, `docker-compose*.yml`. Thêm đuôi mới phải sửa hằng và test. File `.env*`, `*.pem`, `*.key`, `id_*`, `*secret*` luôn bị loại (`Skipped:"not_allowed"`) dù đuôi hợp lệ, phù hợp quy ước series "không đưa secret vào kết quả" (CR-CV-035/072 siết thêm).
5. Bỏ qua thư mục khi đệ quy: `.git`, `node_modules`, `vendor`, `dist`, `out`, `.gitnexus`, `.codegraph`, `target`, `.next`, `build`, `coverage` (mặc định, cấu hình được).
6. Chặn đọc qua đường `git show` nếu `<rel>` có ký tự `:` hoặc NUL (tránh tạo rev-spec khác), và nếu không qua `SHELL_METACHARACTERS` của agent thì trả lỗi tham số thay vì cố mã hoá (agent từ chối `<>&|;$\`\\!` trong tham số).

### 2.4 Giới hạn và bỏ file nhị phân

| Hạn mức (`internal/config`) | Mặc định | Hành vi khi vượt |
|---|---|---|
| `REPOFS_MAX_FILE_BYTES` | 1 MiB | `Skipped:"too_large"`; kiểm tra bằng `size` của `readDir` trước khi đọc, không tải 10 MiB rồi mới bỏ |
| `REPOFS_MAX_FILES_PER_CALL` | 2 000 | `ReadReport.Truncated=true`; các file còn lại không đọc |
| `REPOFS_MAX_BYTES_PER_CALL` | 32 MiB | như trên |
| `REPOFS_MAX_DIR_ENTRIES` | 5 000 mỗi thư mục | cắt, `Truncated=true` |
| `REPOFS_MAX_GIT_OUTPUT_BYTES` | 4 MiB | cắt phía backend, `CODEINTEL_OUTPUT_TOO_LARGE` nếu cần toàn bộ; bắt buộc luôn có `-n`/`--since`/pathspec vì agent không giới hạn |
| `REPOFS_CONCURRENCY` | 8 cuộc gọi song song mỗi lần đọc | tôn trọng hạn mức đồng thời theo dev server của CR-CV-013 |
| `REPOFS_CALL_TIMEOUT` | 25 s | nhỏ hơn 30 s của `RequestTimeout`; hết giờ trả `CODEINTEL_TIMEOUT` |

File nhị phân: `isBinary:true` từ agent hoặc đuôi không nằm trong danh sách cho phép thì **không** đưa vào kết quả và chỉ đếm trong `ReadReport`. Nội dung `encoding:"base64"` chỉ xảy ra với ảnh (agent), cổng không giải mã.

### 2.5 `git log` an toàn với Part A và git 2.25

- Chỉ dùng tuỳ chọn có từ trước 2.25 và không dính regex: `log --no-merges --format=%H%x1f%an%x1f%at%x1f%s -z --since=<n>.days -n<N> -- <pathspec>`; `--name-only` khi cần danh sách file (kèm `-z`, không dùng `-c core.quotePath=false`). Tên tác giả đưa ra UI phải qua che email theo CR-CV-013.
- Không dùng `|` (vd `--format=%H|%an`) vì `|` bị `SHELL_METACHARACTERS`; không dùng pathspec `:!` vì `!`. Dùng `:(exclude)path` (có từ 1.9).
- `GitCapabilityCache` (AGENTS.md, `docs/reference/git-compatibility.md`) là cơ chế của agent/Electron; ở đây cổng chỉ dùng lệnh nền 2.25 nên **không cần probe**; nếu sau này thêm tuỳ chọn mới hơn (vd `--since-as-filter`, 2.36) phải có phương án lùi và khoá trạng thái theo `dev_server_id`. Test hợp đồng với git 2.25.5/2.38.1/2.49.1 thuộc CR-CV-070.
- Part B (`relay-ssh`) ngoài phạm vi (D2). Khác biệt đã thấy: tham số `dirPath`/`filePath` thay cho `path`, `fs.readDir` trả mảng phẳng, `git.exec` whitelist 14 subcommand không có `show`/`status`. Adapter đặt sau một giao diện `agentDialect` để CR-CV-006 bổ sung.

### 2.6 Dấu vân tay nội dung và cache

Nguyên tắc: **không lưu mã nguồn thô vào DB** (có thể chứa secret); lưu kết quả đã parse do CR sở hữu từng view, khoá bởi dấu vân tay.

- `ContentHash`: với file sạch (không có trong `DirtyPaths`) là blob oid từ `BlobIDs(HEAD, …)`, đọc nội dung vẫn được bỏ qua nếu consumer đã có kết quả cho oid đó; với file bẩn là `sha256:<hex>` của nội dung vừa đọc (luôn đọc lại). Đúng vì file sạch có nội dung bằng blob ở HEAD, còn file sửa lại vẫn xuất hiện trong `git.status`.
- Giới hạn đã biết: file bị `.gitignore` không có trong `git.status`; các nguồn của CR 031–037 không nằm trong ignore nên chấp nhận.
- Cache nội dung trong bộ nhớ: LRU theo `(tenant, dev_server, contentHash)`, tổng ≤ 64 MiB, không ghi đĩa, không chia sẻ chéo tenant. Cache kết quả parse thuộc CR-CV-022 (`graph_snapshots`) khoá `(repo, commit, view, params_hash)`; khi có file bẩn, `params_hash` gồm băm của tập `(path, ContentHash)` bẩn để không dùng nhầm.
- Mỗi lần đọc ghi `headCommit`, `dirty` (bool) vào kết quả để UI hiện "chưa commit".

### 2.7 Chịu độ trễ

- Số cuộc gọi tới hạn của một lần dựng ERD toàn repo: ~17 `readDir` cho thư mục `migrations/*` + ~300 `readFile` cho `.up.sql` (số file `.up.sql` thực tế 294, xem CR-CV-031) + vài lô `rev-parse`. Với RTT 100 ms và song song 8, ước tính ~4 s; **chưa đo**. Nếu cache oid trúng, chỉ còn các lô `rev-parse`.
- Không dùng `fs.glob` để liệt kê: trần 200 kết quả, `-maxdepth 10`, chỉ khớp tên cuối của pattern (`handleFsGlob`). Không dùng `fs.grep` để dò mẫu: luôn `--ignore-case`, `--max-count` là theo từng file, bản lùi chỉ quét vài đuôi.
- Không có `fs.readFileStream` ở Part A; file lớn bị từ chối bởi trần 1 MiB.
- Lỗi chuyển tiếp: `INFRA_DEV_SERVER_NOT_CONNECTED` (`FailedPrecondition`) ánh xạ thành lỗi `dev server offline` (xem Điều chỉnh hợp đồng: README thiếu mã cho trường hợp này); không tự thử lại quá 1 lần với lỗi mạng; `PathNotFound (-33003)` trả `ErrNotFound`, không phải lỗi hệ thống.

### 2.8 Ánh xạ lỗi

| Nguồn | Mã trả về |
|---|---|
| Vi phạm 2.3 | `CODEINTEL_PATH_NOT_ALLOWED` |
| Tham số sai (`baseRef` bắt đầu `-`, oid không đủ dài) | `CODEINTEL_INVALID_PARAMS` |
| Agent `FILE_TOO_LARGE`, hoặc vượt hạn mức 2.4 khi bắt buộc đủ dữ liệu | `CODEINTEL_OUTPUT_TOO_LARGE` |
| `context deadline exceeded` | `CODEINTEL_TIMEOUT` |
| `-32601 MethodNotFound` | `CODEINTEL_TOOL_UNAVAILABLE` |
| Agent `PathNotFound` | `ErrNotFound` (miền, không phải `CODEINTEL_*`) |
| Khác | `CODEINTEL_TOOL_FAILED`, giữ message đã che đường dẫn tuyệt đối |

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Chặn đường dẫn ở backend, không sửa agent ở CR này | Đọc ở agent không có confinement (mục 1.3). Sửa agent là thay đổi rộng, ngoài phạm vi; ghi vào Câu hỏi mở Q1 |
| Dùng `readDir` làm "nguồn sự thật" về file thường để loại symlink | `size` vắng với symlink là hành vi đã đọc ở code; rẻ hơn gọi `stat` từng file và `stat` không phát hiện được symlink |
| Đọc commit bằng `git show <c>:<p>`, đọc working tree bằng `fs.readFile` | `show` miễn nhiễm symlink; working tree là thứ review sau khi agent code (O7) |
| Dấu vân tay = blob oid cho file sạch | Một lệnh `rev-parse` lô thay cho hàng trăm lần đọc; không phụ thuộc mtime (agent không trả mtime ở `readDir`) |
| Không dùng `fs.glob`/`fs.grep` | Giới hạn và ngữ nghĩa lệch (mục 2.7) |
| Không tái dùng `RelayExecutor` của git-gateway | Lệch tên trường đã thấy (mục 1.8) và nó phục vụ domain khác |
| Whitelist cứng ở adapter | Cùng tinh thần D5 (agent chỉ method hẹp); ở đây là chiều backend tự giới hạn vì agent Part A mở rộng |

## 4. Tiêu chí chấp nhận

- [ ] Cổng `RepoSourceReader` và adapter `agentrepofs` biên dịch; `go vet`, `golangci-lint`/`oxlint` không lỗi; không có file tên `helpers`/`utils`/`common`/`misc`, không thêm `max-lines` disable.
- [ ] `ListDir` không trả mục symlink (type `file` không có `size`), không trả thư mục trong danh sách bỏ qua, cắt ở `REPOFS_MAX_DIR_ENTRIES` và đặt `Truncated`.
- [ ] `ReadFile` từ chối đường dẫn tuyệt đối, `..`, NUL, tiền tố `-`, và đường dẫn có tổ tiên chưa được liệt kê, với `CODEINTEL_PATH_NOT_ALLOWED`; test bằng agent giả trả `fs.readFile` mà không bao giờ được gọi.
- [ ] File lớn hơn `REPOFS_MAX_FILE_BYTES` không bị gọi `fs.readFile`; file `isBinary:true` không xuất hiện trong kết quả, chỉ trong `ReadReport`.
- [ ] `git show <commit>:<path>` đọc được file của commit cũ trên repo thật; `rev-parse` lô trả oid khớp `git ls-tree`.
- [ ] `DirtyPaths` đánh dấu đúng file sửa chưa commit; file sạch trúng cache theo oid, file bẩn luôn đọc lại và có `ContentHash` kiểu `sha256:`.
- [ ] Mọi lỗi `{error:{code,message}}` nhúng trong kết quả relay được phát hiện và ánh xạ theo 2.8.
- [ ] Khi dev server offline trả lỗi nhận biết được, không treo quá `REPOFS_CALL_TIMEOUT`.
- [ ] Không có đường gọi method ghi/`git` ngoài whitelist; test liệt kê bảng whitelist và thất bại nếu thêm method lạ.
- [ ] Không có nội dung file nào được ghi vào DB hoặc log; log chỉ có đường dẫn tương đối và kích thước.
- [ ] Tài liệu ghi rõ nguồn dữ liệu ngoài E1–E5, E8, E10, E12 mà từng consumer cần.

## 5. Kiểm thử

- **Unit:** `CleanRel` (bảng ca xấu); bộ lọc `readDir` (symlink, thư mục bỏ qua, file không có `size`); chia lô `rev-parse`; ánh xạ lỗi; parser `Log` với `-z` và tên tác giả có ký tự đặc biệt; giới hạn tổng.
- **Agent giả (in-process, trả JSON-RPC mẫu lấy từ code):** `fs.readDir`, `fs.readFile`, `git.exec`, `git.branchCompare`, `git.branchDiff`, `git.status`; kèm ca envelope lỗi nhúng, `FILE_TOO_LARGE`, `PathNotFound`, offline, chậm quá timeout.
- **Hợp đồng với agent thật (CR-CV-070):** chạy agent Part A trên repo mẫu có symlink thư mục trỏ ra ngoài, file không ASCII, file nhị phân, file 11 MiB; xác nhận từng quy tắc 2.3/2.4. Chạy `git` 2.25.5, 2.38.1, 2.49.1.
- Chưa chạy bất kỳ test nào ở thời điểm viết.

## 6. Rủi ro và điểm chưa kiểm chứng

- Đường đi chính xác của lỗi agent qua `Exec` → `RelayByDevServer` (envelope nhúng trong `result_json` hay `JSONRPCError`): suy từ comment trong `relay_executor.go`, chưa chạy. Cổng phải xử lý cả hai.
- Hình dạng `entries` của `git.status` Part A chưa đọc kỹ (`getStatusOp`).
- `Size` của `readDir` cho file thường lấy bằng `stat` theo thời điểm liệt kê; có thể lệch nếu file đổi giữa hai bước (chấp nhận, đọc vẫn bị agent chặn ở 10 MiB).
- Agent trên Windows/macOS: đường dẫn và `find`/`rg` khác Linux; tách đường dẫn theo dấu phân cách của `WorkspaceRoot`. Chưa kiểm chứng trên Windows.
- `git.exec` gom stdout bằng `toString()` từng khúc: nhiều byte UTF-8 cắt đôi có thể sinh ký tự lỗi trong `log`; giảm nhẹ bằng `-z` và tên tác giả ASCII hoá ở backend. Chưa kiểm chứng tần suất.
- Số liệu thời gian ở 2.7 là ước tính.
- Chưa kiểm tra `agent.handshake` có công bố `capabilities` chứa `fs`/`git` tới mức `code-intel-service` đọc được (`agent-wire-protocol.ts` có kiểu `AgentCapability`).

## 7. Câu hỏi mở

- **Q1.** Có làm thêm một thay đổi nhỏ ở agent (kiểm tra `workDir`/`workspaceRoot` cho `fs.readDir/readFile/stat`, giống `fs.writeFile`) như lớp phòng thủ thứ hai không? Cần quyết định riêng, vì có thể làm hỏng các tính năng hiện đang dùng đường dẫn tuyệt đối ngoài `workDir`.
- **Q2.** Mã lỗi cho "dev server offline": thêm `CODEINTEL_DEV_SERVER_OFFLINE` vào danh sách README mục 3.3, hay dùng `CODEINTEL_TOOL_UNAVAILABLE`?
- **Q3.** `REPOFS_MAX_FILE_BYTES = 1 MiB` có đủ cho migration/proto lớn nhất? (chưa đo; xem CR-CV-031 để lấy số file lớn nhất).
- **Q4.** Có cần đọc file ở commit tuỳ ý (không phải HEAD) cho `GetContractDiff` (CR-CV-038) qua `git show`, hay `git.branchDiff` là đủ?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (D2, D5, D6, O4, O7, mục 3.3, 6)
- `/opt/repos/orca/docs/research/view-code/08-views-and-review-models.md`, `09-external-inputs-required.md`
- `/opt/repos/orca/specs/agent/api/agent-rpc-catalog-git-fs.md` (đã đối chiếu, có phần cũ)
- `/opt/repos/orca/agent/src/relay/agent-rpc-dispatch-fs.ts`, `agent-rpc-dispatch-git.ts`, `agent-rpc-dispatch-git-status.ts`
- `/opt/repos/orca/agent/src/relay/fs-agent-extensions.ts`, `fs-agent-search-extensions.ts`, `fs-agent-write-extensions.ts`, `fs-handler-file-read.ts`, `fs-handler-utils.ts`
- `/opt/repos/orca/agent/src/relay/agent-git-handler.ts`, `agent-git-exec-validator.ts`, `agent-git-handler-extended.ts`, `agent-git-handler-local-ops.ts`
- `/opt/repos/orca/agent/src/shared/agent-wire-protocol.ts` (`AgentErrorCode`), `agent/src/shared/git-history.ts`, `git-history-types.ts`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/usecase/relay_by_dev_server.go`, `internal/adapter/grpc/server.go` (hàm `RelayByDevServer`), `internal/adapter/devserveragent/{config,frame,client,session}.go`
- `/opt/repos/orca/backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`RelayByDevServerRequest`, `RelayResponse`)
- `/opt/repos/orca/backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go` (mẫu gọi, và các chỗ lệch)
- `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/AGENTS.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`
- CR liên quan: CR-CV-010, 012, 013, 021, 022, 031–037, 070 (xem `../README.md`)
