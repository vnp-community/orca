# BE-CV-SOL-030: Cổng `RepoSourceReader` ở `code-intel-service`: đọc thư mục, file, lịch sử, diff của repo trên dev server qua `fs.*` / `git.*` của agent

> **📋 Proposed.** Chưa triển khai, chưa chạy build/test nào. Điều kiện tiên quyết của cả feature `code-intel-sources` (mọi CR 031–037 đọc file qua cổng này). Không sửa `agent/`, không sửa `infra-fleet-service`.

**CR:** [CR-CV-030](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-030-repo-file-access-gateway.md)
**Service:** `code-intel-service` (mới, do `BE-CV-SOL-010` dựng): `internal/usecase`, `internal/domain`, `internal/config`, `internal/adapter/agentrepofs` (mới), `internal/adapter/infrafleetrelay` (mới, dùng chung với collector của `BE-CV-SOL-021`)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (mục "The dependency rule", "Standard package layout": cổng ở `usecase`, adapter ở `adapter/*`), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (mục "Multi-tenancy"), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (mục "Multi-tenancy isolation", "Input validation & supply chain"), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (mục "gRPC conventions", "Talking to the Dev Server Agent (execution plane)"), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (mục "Resilience patterns"); service: [`infra-fleet-service`](../../../../tdd/services/infra-fleet-service.md)

---

## 0. Hợp đồng áp dụng

| Mã | Nội dung áp dụng | Mục hợp đồng |
|---|---|---|
| PQ-03 | `CODEINTEL_DEV_SERVER_OFFLINE` = `Unavailable` (giải Q2 của CR); `CODEINTEL_OUTPUT_TOO_LARGE` = `FailedPrecondition`; `CODEINTEL_PATH_NOT_ALLOWED`, `CODEINTEL_TIMEOUT`, `CODEINTEL_TOOL_UNAVAILABLE`, `CODEINTEL_TOOL_FAILED`, `CODEINTEL_INVALID_PARAMS` | `CONTRACT-proto-and-data-map` §1 PQ-03; `CONTRACT-ui-api` §2.3 |
| PQ-02 | Mã lỗi đứng đầu `message` (`CODE: message`); cổng chỉ sinh `apperrors.New(kind, "CODEINTEL_X", ...)` | §1 PQ-02 |
| PQ-04 | `RepoRef` suy ra từ `repo_binding` do `selector{project_id, worktree_ref}` phân giải (CR-012); cổng **không** nhận `repo_binding_id`/đường dẫn tuyệt đối từ client | §1 PQ-04 |
| PQ-13 | Go→agent: mặc định 30 s cho `fs.*`/`git.*` (bảng §2.5 của hợp đồng agent chỉ ngoại lệ `agent.execPrompt`, `ai.complete`, `codeintel.*`, `quality.*`); cổng tự hết hạn ở 25 s | §1 PQ-13; `CONTRACT-agent-rpc` §2.5 |
| PQ-14(6) | `MaxCallRecvMsgSize(16 MiB)` ở client `code-intel-service → infra-fleet` thuộc `BE-CV-SOL-021/023`; cổng này giả định đã có | §1 PQ-14 |
| PQ-23 | Biến môi trường tiền tố `CODEINTEL_` (đổi `REPOFS_*` của CR thành `CODEINTEL_REPOFS_*`) | §1 PQ-23, §6.2 |
| PQ-29 | `SourceRef.kind` mở rộng (`migration|proto|wscompat|sql|code|...`) do `BE-CV-SOL-020`; cổng đặt `kind` cho từng nguồn | §2.1 hàng 2 |
| §8.3 | Mục 4, 5 (mọi truy vấn có `tenant_id`; chỉ đường chạm agent trong whitelist) | §8.3 |
| H8, H9, H10 | Không mã nguồn trong log/lỗi/DB; chịu độ trễ SSH 50–200 ms; mọi khoá cache gồm tenant | §0 |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `agent/src/relay/fs-agent-extensions.ts` (`handleFsReadDir`, `readDirRecursive` dòng 71–99), `agent/src/relay/agent-git-handler.ts` (dòng 41–70 `ALLOWED_GIT_SUBCOMMANDS` có `show`, `rev-parse`, `log`, `status`, `diff`; `SHELL_METACHARACTERS = /[&|;$\`<>\\!]/` dòng 70; `git.exec` đọc `params.args`, `params.cwd`, `timeout ≤ 60_000`, dòng 75–125), `agent/src/relay/agent-rpc-dispatch-git.ts` (dòng 59–113: `git.branchCompare`, `git.commitCompare`, `git.branchDiff`, `git.commitDiff`, `git.checkIgnored`), `agent/src/relay/agent-rpc-dispatch-git-status.ts` (`git.status`), `agent/src/relay/git-handler-status-ops.ts` (`getStatusOp`, dòng 59–110), `agent/src/relay/git-status-output-parser.ts`, `backend-go/services/infra-fleet-service/internal/usecase/relay_by_dev_server.go`, `…/adapter/devserveragent/client.go` (`Exec` dòng 424–449), `backend-go/services/git-gateway-service/internal/adapter/grpcclient/{tenant_forwarding.go, relay_executor.go}` (dòng 85–160), `backend-go/services/api-gateway/internal/adapter/wscompat/channels_accounts.go` (mẫu gọi `RelayByDevServer`, dòng 136–160), `backend-go/common/{dbcapability/capability.go, apperrors/apperrors.go}`; kích thước file lớn nhất (`find`/`ls -S`): proto 72 KB (`infrafleet.proto`), Go không-test 72 KB, migration `.up.sql` 7,7 KB.

Xác nhận đúng: `code-intel-service` **chưa có** (`ls backend-go/services` không có), `proto/orca/codeintel/` chưa có; `fs.readDir` đặt `size` chỉ khi `entry.isFile()` nên symlink và mục đặc biệt có `size: undefined` (dòng 80–85); `git.exec` whitelist có `show`, `rev-parse`, `log`, `diff`, `status` nhưng không có `ls-tree`/`cat-file`/`merge-base`/`for-each-ref`; cờ `-c` đứng trước subcommand bị từ chối; `git.exec` gom stdout bằng `chunk.toString()` không giới hạn dung lượng; `git.status` trả `{entries[{path,status,area,…}], conflictOperation, head?, branch?, didHitLimit?, statusLength?}` và luôn dùng `--untracked-files=all` (file chưa theo dõi nằm trong `entries` với `area:'untracked'`, `git-status-output-parser.ts:119`).

### Correction relative to CR-CV-030

| # | CR nói | Mã/hợp đồng thật | Xử lý |
|---|---|---|---|
| C1 | Biến cấu hình `REPOFS_*` | PQ-23: tiền tố `CODEINTEL_` cho mọi biến của `code-intel-service` | `CODEINTEL_REPOFS_*` (mục 2.D) |
| C2 | "Hình dạng `entries` của `git.status` chưa đọc kỹ" | Đã đọc: `entries[].path` (tương đối `worktreePath`), `status`, `area ∈ staged|unstaged|untracked`; thêm `head` (có thể thay `rev-parse HEAD`); `-c core.quotePath=false` do chính agent chèn nội bộ, nên đường dẫn không ASCII **không** bị trích dẫn ở `git.status` (khác `git.exec log`) | `DirtyPaths` đọc `entries[].path`, gộp `staged|unstaged|untracked`; dùng `head` làm đáp án nhanh của `ResolveHead` khi đã gọi `git.status` |
| C3 | Lỗi `PathNotFound (-33003)` → `ErrNotFound`, `INFRA_DEV_SERVER_NOT_CONNECTED` → offline | Hợp đồng agent §3.4: `AgentRPCError` + mã `CODEINTEL_*` chỉ áp cho method `codeintel.*`/`quality.*`. Với `fs.*`/`git.*`, `RelayByDevServer.Execute` vẫn bọc mọi lỗi `Exec` thành `INFRA_AGENT_EXEC_FAILED` (`relay_by_dev_server.go`, dòng cuối); nguyên nhân (`-33003`, `FILE_TOO_LARGE`) **không ra khỏi infra-fleet** qua gRPC. Chỉ phân biệt được: (a) phong bì `{error:{code,message}}` **nhúng trong `result_json`** (git-gateway đã gặp, `relay_executor.go:128–148`), (b) tiền tố `INFRA_DEV_SERVER_NOT_CONNECTED` / `INFRA_DEV_SERVER_NOT_FOUND` trong `status.Message`, (c) `DeadlineExceeded`/`Unavailable` | Bảng ánh xạ lỗi mục 2.F dựa vào (a)(b)(c); mọi lỗi còn lại → `CODEINTEL_TOOL_FAILED`. Ghi vào điểm hợp đồng thiếu G1 (mục 7) |
| C4 | Q3: 1 MiB đủ cho migration/proto lớn nhất? | Lớn nhất đo được: proto 72 KB, Go 72 KB, `.up.sql` 7,7 KB | 1 MiB dư rộng; giữ mặc định, cấu hình được |
| C5 | CR-CV-021 "nếu chưa merge thì CR này tự khai báo `AgentRelay` tối thiểu" | Hợp đồng không giao chủ sở hữu port `AgentRelay` | SOL-030 **tạo** `usecase.AgentRelay` + adapter `infrafleetrelay`; `BE-CV-SOL-021` dùng lại (mục 3, Phụ thuộc chéo) |
| C6 | `git show <commit>:<rel>` "Không đọc ngoài repo" | `show` không có giới hạn dung lượng (agent không chặn, không có `cat-file -s` để biết trước kích thước ở commit cũ) | Chỉ cho đọc ở commit khi đuôi nằm trong allowlist; cắt cứng phía backend ở `MAX_GIT_OUTPUT_BYTES`; xem rủi ro R2 |

## 2. Giải pháp

### A. Cây file (mới, trong `backend-go/services/code-intel-service/`)

```
internal/domain/repo_relative_path.go        # CleanRel, ErrPathNotAllowed (miền thuần)
internal/domain/repo_source_policy.go        # allowlist đuôi/tên, deny-list bí mật, thư mục bỏ qua
internal/usecase/repo_source_reader.go       # RepoRef, SourceRef, FileContent, ReadReport, DirEntry, interface RepoSourceReader
internal/usecase/agent_relay.go              # interface AgentRelay (dùng chung SOL-021)
internal/config/repofs_limits.go             # CODEINTEL_REPOFS_*
internal/adapter/infrafleetrelay/relay_client.go   # AgentRelay qua InfraFleetService.RelayByDevServer (+ metadata tenant)
internal/adapter/agentrepofs/reader.go             # RepoSourceReader: ghép các phần dưới
internal/adapter/agentrepofs/method_whitelist.go   # bảng hằng method/subcommand cho phép + test
internal/adapter/agentrepofs/list_dir.go           # fs.readDir depth 1, lọc symlink, thư mục bỏ qua
internal/adapter/agentrepofs/read_file.go          # fs.readFile (worktree) / git show (commit), ancestor rule
internal/adapter/agentrepofs/git_read_ops.go       # ResolveHead, BlobIDs, DirtyPaths, ChangedFiles, FileDiff
internal/adapter/agentrepofs/git_log_parser.go     # Log với -z, định dạng %x1f
internal/adapter/agentrepofs/agent_envelope_errors.go  # phát hiện phong bì lỗi nhúng, ánh xạ mã
internal/adapter/agentrepofs/content_cache.go      # LRU theo (tenant, dev_server, contentHash)
internal/adapter/agentrepofs/testdata/agent-rpc/*.json   # mẫu JSON-RPC ghi từ mã agent
```

Cấm `helpers`, `utils`, `common`, `misc`; không `max-lines` disable (AGENTS.md).

### B. Cổng usecase (chữ ký; khớp CR 2.1 nhưng bỏ `repo_binding_id`)

```go
// RepoRef do use case dựng từ repo_binding (đã qua kiểm quyền CR-013); không bao giờ từ client.
type RepoRef struct{ TenantID, DevServerID, WorkspaceRoot string }
type SourceRef struct{ Kind string /* "worktree" | "commit" */; Commit string }

type RepoSourceReader interface {
    ResolveHead(ctx context.Context, r RepoRef) (string, error)
    DirtyPaths(ctx context.Context, r RepoRef, scope []string) (map[string]struct{}, error)
    ListDir(ctx context.Context, r RepoRef, relDir string) ([]DirEntry, ReadReport, error)
    ReadFile(ctx context.Context, r RepoRef, s SourceRef, relPath string) (FileContent, error)
    ReadFiles(ctx context.Context, r RepoRef, s SourceRef, relPaths []string) ([]FileContent, ReadReport, error)
    BlobIDs(ctx context.Context, r RepoRef, commit string, relPaths []string) (map[string]string, error)
    ChangedFiles(ctx context.Context, r RepoRef, baseRef string) (BranchCompare, error)
    FileDiff(ctx context.Context, r RepoRef, baseRef, relPath string) (DiffPair, error)
    Log(ctx context.Context, r RepoRef, q LogQuery) ([]CommitRecord, error)
}

type AgentRelay interface { // thuộc cổng usecase; cài đặt: adapter/infrafleetrelay
    Call(ctx context.Context, devServerID, method string, params map[string]any) (json.RawMessage, error)
}
```

`ListDir` trả thêm `ReadReport` (CR trả `[]DirEntry` đơn thuần nhưng cần `Truncated`). `FileContent.Skipped ∈ ""|binary|too_large|symlink_or_special|not_allowed|not_found`. `ContentHash`: blob oid cho file sạch, `sha256:<hex>` cho file bẩn (CR 2.6).

### C. Ánh xạ method agent (đã đối chiếu mã)

| Hàm cổng | Method + tham số | Ghi chú đã kiểm |
|---|---|---|
| `ListDir` | `fs.readDir {path:<abs>, depth:1}` → `result.entries[{path,name,type,size?}]` | symlink/đặc biệt: `type:"file"` + không `size` (`fs-agent-extensions.ts:80–85`); loại bỏ |
| `ReadFile` worktree | `fs.readFile {path:<abs>}` | chỉ sau khi `ListDir` xác nhận file thường, `size ≤ cap` |
| `ReadFile` commit | `git.exec {args:["show","<oid40|64>:<rel>"], cwd:<root>, timeout:25000}` | `<rel>` qua `CleanRel` + không chứa `:`; đuôi trong allowlist |
| `ResolveHead` | `git.exec {args:["rev-parse","--verify","HEAD"], cwd}` | |
| `BlobIDs` | `git.exec {args:["rev-parse","<c>:<p1>",…]}` | lô ≤ 100 đường dẫn |
| `DirtyPaths` | `git.status {worktreePath:<root>, limit:0}` | gộp `entries[].path` (cả `untracked`); `didHitLimit` luôn false khi `limit:0` |
| `ChangedFiles` | `git.branchCompare {worktreePath, baseRef}` | `baseRef` không bắt đầu `-`; `summary.status != ready` → lỗi có mã `invalid-base|unborn-head|no-merge-base` ánh xạ `CODEINTEL_INVALID_PARAMS reason=` |
| `FileDiff` | `git.branchDiff {worktreePath, baseRef, includePatch:true, filePath, oldPath?}` | hai phía nội dung; `originalIsBinary|modifiedIsBinary` → bỏ |
| `Log` | `git.exec {args:["log","--no-merges","--format=%H%x1f%an%x1f%at%x1f%s","-z","--since=<n>.days","-n<N>","--",<pathspec>]}` | không `|`, `!`, `:!`; dùng `:(exclude)`; chỉ lệnh có từ trước git 2.25 |

Mọi đường dẫn gửi agent là tuyệt đối (`WorkspaceRoot + "/" + rel`; POSIX only, O-14). Method **cấm**: `fs.write*`, `fs.mkdir`, `fs.rmdir`, `fs.copyFile`, mọi `git.exec` ngoài `{log, show, rev-parse, diff, status}`, mọi `git.commit|push|pull|fetch|checkout|stage…`. Bảng hằng ở `method_whitelist.go`; test khẳng định đúng tập này (§8.3-5).

### D. An toàn đường dẫn và hạn mức

1. `CleanRel(p)`: từ chối rỗng, tuyệt đối, NUL, `..` sau `path.Clean`, bắt đầu `-`, chứa `\`, ký tự điều khiển → `CODEINTEL_PATH_NOT_ALLOWED`. Chuẩn hoá NFC rồi so khớp (tránh U+FF0D như hợp đồng agent §2.1).
2. **Quy tắc tổ tiên**: chỉ đọc đường dẫn mà mọi cấp tổ tiên đã xuất hiện (type `directory`) trong kết quả `ListDir` của phiên đọc; đường dẫn cố định (`go.mod`) cũng đi qua `ListDir` của thư mục cha. Phiên đọc = struct `readSession` (cache `ListDir` theo thư mục) sống trong một lời gọi use case.
3. Allowlist đuôi/tên: `.sql .proto .go .yaml .yml .json .toml .md Dockerfile CODEOWNERS docker-compose*.yml`; deny-list (`.env*`, `*.pem`, `*.key`, `id_*`, `*secret*`) thắng allowlist → `Skipped:"not_allowed"`. Thư mục bỏ qua: `.git node_modules vendor dist out .gitnexus .codegraph target .next build coverage` (cấu hình được).
4. Hạn mức (`CODEINTEL_REPOFS_*`, mặc định như CR): `MAX_FILE_BYTES=1 MiB`, `MAX_FILES_PER_CALL=2000`, `MAX_BYTES_PER_CALL=32 MiB`, `MAX_DIR_ENTRIES=5000`, `MAX_GIT_OUTPUT_BYTES=4 MiB`, `CONCURRENCY=8`, `CALL_TIMEOUT=25s`. Giá trị sai bị bỏ + log cảnh báo (cùng quy ước hợp đồng agent §2.3). `CONCURRENCY` phải nhỏ hơn hạn mức đồng thời theo dev server của `BE-CV-SOL-013-agent-call-gate-and-quotas` (đi qua cổng đó nếu đã có).

### E. Cache và dấu vân tay (không lưu mã nguồn thô vào DB)

LRU trong bộ nhớ, khoá `(tenantID, devServerID, contentHash)`, tổng ≤ 64 MiB, không ghi đĩa; `tenantID` luôn nằm trong khoá để không chéo tenant (H10). Cache kết quả parse thuộc `BE-CV-SOL-022-snapshot-cache` (`graph_snapshots`, `params_hash` gồm tập `(path, ContentHash)` của file bẩn). Mỗi lần đọc trả `headCommit` và `dirty bool` cho consumer.

### F. Ánh xạ lỗi (điều chỉnh theo C3)

| Nguồn quan sát được | Mã `CODEINTEL_*` | `apperrors.Kind` |
|---|---|---|
| Vi phạm mục D | `CODEINTEL_PATH_NOT_ALLOWED` | `KindInvalidArgument` (xác nhận với chủ sở hữu `BE-CV-SOL-010`; hợp đồng agent dùng `PermissionDenied` ở infra-fleet, nhưng đây là lỗi tham số tầng service) |
| `baseRef` bắt đầu `-`, oid không đủ dài, `summary.status` lỗi | `CODEINTEL_INVALID_PARAMS` | `KindInvalidArgument` |
| Phong bì nhúng `-32601` | `CODEINTEL_TOOL_UNAVAILABLE` | `KindFailedPrecondition` |
| Phong bì nhúng `-33003` (PathNotFound) | `domain.ErrNotFound` (miền, không `CODEINTEL_*`) | `KindNotFound` |
| Phong bì nhúng message `FILE_TOO_LARGE`, hoặc vượt hạn mức khi bắt buộc đủ dữ liệu | `CODEINTEL_OUTPUT_TOO_LARGE` | `KindFailedPrecondition` |
| `INFRA_DEV_SERVER_NOT_CONNECTED` trong `status.Message` | `CODEINTEL_DEV_SERVER_OFFLINE` | `KindUnavailable` (thêm bởi SOL-010, PQ-03(8)) |
| `INFRA_DEV_SERVER_NOT_FOUND` | `CODEINTEL_NO_DEV_SERVER` (miền CR-012) | `KindFailedPrecondition` |
| `context.DeadlineExceeded`, `status.DeadlineExceeded` | `CODEINTEL_TIMEOUT` | `KindDeadlineExceeded` |
| Còn lại | `CODEINTEL_TOOL_FAILED`, message đã che đường dẫn tuyệt đối | `KindInternal` |

Thử lại: tối đa 1 lần với lỗi mạng (`Unavailable` không phải OFFLINE); không retry khi lỗi tham số. Chờ nối lại 20 s thuộc collector (`BE-CV-SOL-021`), không phải cổng (PQ-13).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Chặn đường dẫn ở backend, không sửa agent | `fs.readDir/readFile/stat/glob/grep` Part A không có confinement (chỉ `fs.writeFile`, README v7 mục 8 điểm 1); Q1 (lớp thứ hai ở agent) để mở |
| `readDir` làm nguồn sự thật về file thường | `size` vắng cho symlink là hành vi đã đọc; `fs.stat` theo symlink nên không phát hiện được |
| Commit đọc bằng `git show`, worktree bằng `fs.readFile` | `show` miễn nhiễm symlink; review đọc working tree (O7) |
| Dấu vân tay = blob oid cho file sạch | Một `rev-parse` lô thay hàng trăm lần đọc; không phụ thuộc mtime |
| Không tái dùng `RelayExecutor` của git-gateway | Lệch tên trường (CR 1.8) và phục vụ miền khác |
| Không `fs.glob`/`fs.grep` | Trần 200 kết quả, ngữ nghĩa lệch (CR 2.7) |
| `AgentRelay` tạo ở SOL này, chia sẻ với SOL-021 | Giữ một client `RelayByDevServer`; tránh hai bản gắn metadata tenant khác nhau |
| Lỗi ánh xạ chỉ từ cái quan sát được | Mã agent không ra khỏi infra-fleet với `fs.*`/`git.*` (C3); không bịa mã |

## 4. Lệch giữa CR và hợp đồng

| # | CR-CV-030 | Hợp đồng | Theo |
|---|---|---|---|
| L1 | `REPOFS_*` | PQ-23 `CODEINTEL_` | Hợp đồng |
| L2 | Q2 mã offline mở | PQ-03(3) `CODEINTEL_DEV_SERVER_OFFLINE` = `Unavailable` | Hợp đồng |
| L3 | `-32601` → `TOOL_UNAVAILABLE` cho mọi method | §3.3: `CODEINTEL_AGENT_UNSUPPORTED` chỉ cho `codeintel.*`/`quality.*` | Giữ `TOOL_UNAVAILABLE` cho `fs.*/git.*` (hợp đồng không cấm; ghi lại ở G1) |
| L4 | `repo_binding_id` ngầm trong `RepoRef` | PQ-04 `selector` | `RepoRef` dựng từ binding đã phân giải; cổng không có RPC |
| L5 | Part B (`agentDialect`) | O-5: chỉ Part A ở MVP | Chỉ Part A; tách `agentDialect` là phụ (không làm) |

## 5. Phụ thuộc chéo khu vực

| Khu vực | Solution đối ứng | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-010-scaffold-code-intel-service` | module, `apperrors.KindUnavailable`, config nền (§7 thứ tự: 010→011→012) |
| BE | `BE-CV-SOL-012-target-resolution-and-bindings` | cấp `workspace_root`, `dev_server_id`, `worktree_ref` cho `RepoRef` |
| BE | `BE-CV-SOL-013-agent-call-gate-and-quotas` | hạn mức đồng thời theo dev server |
| BE | `BE-CV-SOL-021-agent-collector`, `BE-CV-SOL-023-infra-fleet-codeintel-transport` | dùng chung `AgentRelay`; `MaxCallRecvMsgSize` 16 MiB |
| BE | `BE-CV-SOL-022-snapshot-cache` | tiêu thụ `ContentHash`/`dirty` làm `params_hash` |
| BE | `BE-CV-SOL-031/032/033/034/035/036/037/038` | mọi solution đọc file qua cổng này (S1) |
| BE | `BE-CV-SOL-070-collector-golden-contract` | hợp đồng với agent thật và git 2.25.5/2.38.1/2.49.1 |
| AG | — (không đổi agent) | Q1: lớp phòng thủ thứ hai ở agent là quyết định riêng, chưa có CR |
| FE | — | cổng không lộ ra UI |

## 6. Tiêu chí chấp nhận

- [x] `RepoSourceReader` và `agentrepofs` biên dịch; `go vet`, `golangci-lint` sạch; không tên `helpers/utils/common/misc`; không `max-lines` disable.
- [x] `ListDir` không trả symlink (file không `size`) và thư mục bỏ qua; cắt ở `MAX_DIR_ENTRIES` và `Truncated=true`.
- [x] `CleanRel`/ancestor rule: từ chối tuyệt đối, `..`, NUL, tiền tố `-`, `\`, tổ tiên chưa liệt kê; test bằng agent giả mà `fs.readFile` **không bao giờ** được gọi.
- [x] File > `MAX_FILE_BYTES` không bị gọi `fs.readFile`; `isBinary:true` chỉ đếm trong `ReadReport`.
- [x] `git show <oid>:<path>` đọc được file commit cũ; `BlobIDs` khớp `git ls-tree` (kiểm bằng git thật ở CI).
- [x] `DirtyPaths` có cả file `untracked`; file sạch trúng cache theo oid, file bẩn luôn đọc lại với `sha256:`.
- [x] Mọi `{error:{code,message}}` nhúng trong `result_json` được phát hiện (cả `code` số) và ánh xạ theo F.
- [x] Offline trả `CODEINTEL_DEV_SERVER_OFFLINE`; không treo quá `CALL_TIMEOUT`.
- [x] Bảng whitelist method khớp test; thêm method lạ làm test thất bại.
- [x] Không có nội dung file trong DB/log/lỗi; log chỉ đường dẫn tương đối + kích thước.
- [x] Cache nội dung cô lập tenant (hai tenant, cùng `contentHash`, không trúng chéo).

## 7. Kiểm thử, rủi ro, câu hỏi mở

**Kiểm thử.** Unit: `CleanRel` (bảng ca xấu), bộ lọc `readDir`, chia lô `rev-parse`, parser `Log` (`-z`, tên tác giả đặc biệt, UTF-8 cắt đôi), ánh xạ lỗi, giới hạn tổng. Agent giả in-process trả JSON-RPC mẫu từ mã thật (`testdata/agent-rpc/`) cho `fs.readDir`, `fs.readFile`, `git.exec`, `git.status`, `git.branchCompare`, `git.branchDiff`; kèm phong bì lỗi nhúng, `FILE_TOO_LARGE`, `-33003`, offline, chậm quá timeout. Không có DB ở SOL này, nên **không cần** ma trận hai dialect; cô lập tenant kiểm bằng test cache và test metadata tenant đi theo mọi cuộc gọi. Hợp đồng agent thật + git 2.25.5/2.38.1/2.49.1: `BE-CV-SOL-070`. Lệnh dự kiến: `cd backend-go && go test ./services/code-intel-service/internal/domain/... ./services/code-intel-service/internal/adapter/agentrepofs/...` (chưa chạy).

**Rủi ro / chưa kiểm chứng.**
- R1: đường đi chính xác của lỗi agent qua `Exec` → `RelayByDevServer` (nhúng trong `result_json` hay `JSONRPCError`): suy từ comment `relay_executor.go:128–148`; chưa chạy. Cổng xử lý cả hai.
- R2: `git show` ở commit cũ không có trần dung lượng phía agent; chỉ chặn được sau khi nhận (cắt ở 4 MiB). File lớn bị agent buffer trong RAM.
- R3: `git.exec` gom stdout bằng `toString()` từng khúc: byte UTF-8 cắt đôi có thể sinh ký tự lỗi trong `log`; giảm bằng `-z` và ASCII hoá tên tác giả. Tần suất chưa kiểm chứng.
- R4: Windows/macOS: tách đường dẫn theo dấu phân cách của `WorkspaceRoot`; chỉ POSIX ở MVP (O-14).
- R5: số cuộc gọi cho ERD toàn repo (~17 `readDir` + ~300 `readFile` + vài lô `rev-parse`) với RTT 100 ms, song song 8 ≈ 4 s là **ước tính**, chưa đo.

**Điểm hợp đồng thiếu (báo chủ hợp đồng).** G1: hợp đồng không nói cách ánh xạ lỗi của `fs.*`/`git.*` qua infra-fleet (CR-023 chỉ phủ `codeintel.*`/`quality.*`); G2: không giao chủ sở hữu port `AgentRelay`.

**Câu hỏi mở.** Q1 (CR): có thêm kiểm tra `workDir` ở agent cho `fs.readDir/readFile/stat` không (có thể hỏng đường dẫn tuyệt đối đang dùng). Q4 (CR): `GetContractDiff` đọc file ở commit tuỳ ý bằng `git show` hay chỉ `git.branchDiff` (liên quan `BE-CV-SOL-038-contract-diff`).

## 8. Tham chiếu

- `docs/crs/v7/code-intel-sources/CR-CV-030-repo-file-access-gateway.md`, `docs/crs/v7/README.md` (D2, D5, D6, O4, O7, mục 8 điểm 1)
- `specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (PQ-02, 03, 04, 13, 14, 23, 29; §8.3), `CONTRACT-codeintel-agent-rpc.md` (§2.5, §3.4), `CONTRACT-codeintel-ui-api.md` (§2.3)
- `agent/src/relay/{fs-agent-extensions,agent-git-handler,agent-rpc-dispatch-git,agent-rpc-dispatch-git-status,git-handler-status-ops,git-status-output-parser}.ts`
- `backend-go/services/infra-fleet-service/internal/usecase/relay_by_dev_server.go`, `…/adapter/devserveragent/client.go`
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/{tenant_forwarding.go,relay_executor.go}`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_accounts.go`
- `guides/reference/git-compatibility.md`, `AGENTS.md`
