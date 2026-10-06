# CR-CV-001 — Nền `codeintel.*` trên agent: đăng ký method, whitelist, phân giải repo, giới hạn, mã lỗi

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-001 |
| **Tên** | Thêm nhóm RPC `codeintel.*` vào agent TypeScript (Part A): dispatcher riêng, whitelist lệnh GitNexus/CodeGraph, phân giải `workspaceRoot` → repo đã đăng ký, giới hạn tài nguyên, phát hiện công cụ, mã lỗi `CODEINTEL_*`, `capabilities` trong handshake |
| **Loại** | Feature (nền cho các CR trích xuất) |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | Không |
| **Mở khoá** | CR-CV-002, CR-CV-003, CR-CV-004, CR-CV-005, CR-CV-006; phía Go: CR-CV-021, CR-CV-023 |
| **Tác động** | `agent/src/relay/agent-rpc-dispatch.ts` (thêm 1 nhánh trong `route()` và 1 nhánh trong `extractTraceFields`), `agent/src/relay/agent-tool-registry.ts` (mở rộng tuỳ chọn của `runToolCommand`), `agent/src/relay/agent-session-capabilities.ts`, các file mới `agent/src/relay/agent-rpc-dispatch-codeintel.ts` và `agent/src/relay/codeintel-*.ts` (liệt kê ở 2.1) |

---

## 1. Bối cảnh và vấn đề

Các khẳng định dưới đây do người soạn đọc code ngày 2026-10-05 (đường dẫn tương đối `agent/src/relay/`).

1. **Agent đã có hai tool CLI thô.** `agent-tool-registry.ts:185` (`gitnexus`) và `:205` (`codegraph`) nhận `args: string[]` tự do và `cwd` tự do, chạy qua `runToolCommand` (`:72-113`) với timeout cứng 60 s. Qua `tools/call` (`agent-rpc-dispatch-misc.ts`, case `'tools/call'`) một client có thể chạy `gitnexus analyze`, `gitnexus clean`, `gitnexus remove`. Backend Go chưa gọi `tools/call` ở đâu (README v7 mục 1; chưa kiểm chứng lại bằng grep toàn repo trong CR này).
2. **`runToolCommand` không giới hạn đầu ra.** stdout/stderr gom vào mảng chuỗi không cap (`:84-100`); khi quá hạn chỉ `child.kill('SIGTERM')` rồi resolve với `exitCode: 124`, không leo thang `SIGKILL`, không diệt cả cây tiến trình. Hàm `resolveToolBinary` (`:54`) là private và tách `toolPath` theo dấu `:` (POSIX), nên không dùng được trên Windows (`agent-config.ts` `buildToolPath` cũng chỉ sinh đường dẫn POSIX).
3. **Không có "workspace root đã đăng ký" trên agent.** README v7 mục 3.2 giả định `workspaceRoot` "đã nằm trong workspace root đăng ký". Thực tế: `fs.*` nhận mọi đường dẫn tuyệt đối (`fs-agent-extensions.ts` `handleFsReadDir`: `isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)`, không kiểm tra thư mục gốc); `RelayContext.registerRoot` là no-op có chủ ý (`context.ts:26-33`, tài liệu `docs/relay-fs-allowlist-removal.md`); `validateWorktreePath` (`git-handler.ts:172`) chỉ áp cho `git worktree add`. Do đó "tập đường dẫn được phép" phải do `codeintel.*` tự định nghĩa (mục 2.4), không có cơ chế để tái dùng.
4. **GitNexus có registry toàn cục nhiều repo.** `~/.gitnexus/registry.json` là mảng JSON, mỗi phần tử có `name`, `path`, `storagePath`, `indexedAt`, `lastCommit`, `remoteUrl`, `stats`, `branch` (đọc trực tiếp file trên máy khảo sát: 12 repo). Thiếu `-r` thì CLI ném `Multiple repositories indexed...` (đã chạy thử). `-r` nhận cả tên lẫn đường dẫn tuyệt đối (đã chạy thử `gitnexus cypher -r /opt/repos/orca ...` trả kết quả đúng). Lưu ý: `gitnexus list` chỉ in văn bản, không có JSON.
5. **Chạy trong worktree liên kết trả về index của checkout chính.** Trong `/opt/repos/orca/.claude/worktrees/dev-process-9beda3` (worktree liên kết, HEAD `d819812`), `gitnexus status` in `Repository: /opt/repos/orca`; `codegraph status --json` trả `projectPath: /opt/repos/orca` kèm `worktreeMismatch: {worktreeRoot, indexRoot}`. Worktree đó có thư mục `.codegraph/` nhưng chỉ chứa `.gitignore`, không có DB. Hệ quả: index không phải của worktree đang review (xem 2.4 và mục 6).
6. **Handshake hiện chỉ mang tên tool.** `agent-session-handshake.ts:70` gửi `tools: tools.map((t) => t.name)` (chuỗi tên, không phải đối tượng; README v7 ghi `tools[]` là danh sách tên đúng như vậy), và `capabilities` từ `buildCapabilities` (`agent-session-capabilities.ts:58`), bị đua với timeout 5 s rồi rơi về `STATIC_CAPABILITIES_FALLBACK` (`:105`). Phía Go đọc `capabilities` như `[]string` tự do (`adapter/agentwsserver/server.go:80`).
7. **Đường vận chuyển.** Khung tối đa 16 MiB (`MaxMessageSize`, `adapter/devserveragent/frame.go:24`); timeout mặc định mỗi lệnh 30 s trừ `agent.execPrompt` (`client.go:412`). `Client.Exec` giải mã kết quả thành `map[string]any` (`client.go:444`) nên kết quả `codeintel.*` luôn phải là **object JSON**, không phải mảng. Lỗi JSON-RPC giữ nguyên `Data` trong `JSONRPCError` (`jsonrpc.go:27-31`).
8. **Chế độ `--stdio` dùng chung Part A.** `agent-entry.ts:80-88` + `agent-connection-stdio.ts` tạo `createSession` (cùng `createRpcDispatcher`) cho `node agent.js --stdio` mà Go dùng cho `relay-ssh`. Vì vậy mọi method thêm vào Part A tự có mặt qua SSH do Go khởi tạo (xem CR-CV-006).

9. **`gitnexus` cắt cụt stdout khi đi qua pipe (phát hiện khi chạy thử, ảnh hưởng thiết kế).** `gitnexus cypher` cho truy vấn 3 000 hàng: chuyển hướng ra tệp cho 411-436 KB JSON hợp lệ; qua pipe shell cho đúng 65 536 byte (JSON cụt, `exit 0`); qua `child_process.spawn` với `stdio: 'pipe'` (đúng kiểu `runToolCommand`) cho 146 176 byte. Nguyên nhân chưa xác định (nhiều khả năng tiến trình gọi `process.exit` trước khi pipe xả hết). `codegraph query -j -l 200` qua pipe cho đủ 134 449 byte nên không bị. Hệ quả: `runToolCommand` hiện tại **không dùng được** cho đầu ra lớn của GitNexus; phải ghi stdout ra tệp tạm (2.5).

Vấn đề cần giải: cung cấp cho backend một bề mặt **hẹp, có kiểu, có giới hạn** để hỏi code-intelligence, không cần và không được phép gửi `args` hay tên repo tự do.

## 2. Giải pháp đề xuất

### 2.1 Cấu trúc file (mới)

Tất cả dưới `agent/src/relay/`. Tên theo khái niệm cụ thể (AGENTS.md: cấm `helpers`/`utils`/`common`/`misc`); không thêm `max-lines` disable.

| File (mới) | Nội dung |
|---|---|
| `agent-rpc-dispatch-codeintel.ts` | `dispatchCodeIntelRpc(rpc, config, log, ws, state): Promise<JsonRpcResponse \| null>`. Trả `null` nếu `rpc.method` không bắt đầu bằng `codeintel.`. Mẫu theo `agent-rpc-dispatch-misc.ts`: `import()` động handler, bọc `try/catch`, trả `makeError` |
| `codeintel-method-table.ts` | Bảng `CODEINTEL_METHODS`: tên method → `{ validate, handle, timeoutMs }`. Một chỗ duy nhất liệt kê method; CR-CV-002..005 thêm dòng vào đây |
| `codeintel-errors.ts` | `class CodeIntelError`, kiểu `CodeIntelErrorCode`, `toJsonRpcError(id, err)` |
| `codeintel-limits.ts` | Hằng số giới hạn (2.5), đọc override từ biến môi trường |
| `codeintel-repo-resolution.ts` | `resolveCodeIntelRepo(workspaceRoot)` (2.4) |
| `gitnexus-registry-reader.ts` | Đọc và kiểm tra `~/.gitnexus/registry.json` |
| `codeintel-command-whitelist.ts` | Kiểu `GitNexusCommand`, `CodeGraphCommand` (union phân biệt) và hàm dựng argv (2.3) |
| `codeintel-tool-runner.ts` | `runCodeIntelTool(command, binding, opts)`: dựng argv từ whitelist, gọi `runToolCommand`, cắt đầu ra, quy đổi lỗi |
| `codeintel-tool-detection.ts` | `detectCodeIntelTools(config)`: có binary không, phiên bản, có được hỗ trợ không (cache 60 s) |
| `codeintel-result-envelope.ts` | `buildCodeIntelResult(...)` dựng phần đầu chung `{sources, headCommit, stale, truncated, totalCount, data}` |
| `codeintel-concurrency-gate.ts` | Semaphore giới hạn số tiến trình công cụ chạy đồng thời |
| `codeintel-status.ts` | Handler `codeintel.status`; gọi các probe chỉ mục do CR-CV-002/003 đăng ký |

Sửa file có sẵn (nhỏ):

| File | Thay đổi |
|---|---|
| `agent-rpc-dispatch.ts` | Trong `route()` (hiện `:297-382`) thêm, ngay trước dòng `return makeError(... MethodNotFound ...)` (`:381`), một khối `const fromCodeIntel = await dispatchCodeIntelRpc(rpc, config, log, ws, state); if (fromCodeIntel !== null) return fromCodeIntel`. Trong `extractTraceFields` (`:92`) thêm nhánh `method.startsWith('codeintel.')` chỉ ghi `workspaceRoot` (cắt 60 ký tự) và tên method, **không** ghi tham số khác (tránh lộ tên symbol/đường dẫn nhạy cảm vào trace) |
| `agent-tool-registry.ts` | Mở rộng `runToolCommand` bằng tuỳ chọn không bắt buộc (2.5): `maxOutputBytes`, `killGraceMs`, `signal`. Mặc định giữ nguyên hành vi hiện tại để `tools/call` không đổi |
| `agent-session-capabilities.ts` | `buildCapabilities` thêm `codeintel`, `codeintel.gitnexus`, `codeintel.codegraph` (2.7) |

### 2.2 Hợp đồng RPC của lớp nền

Mọi method nhận `workspaceRoot` (string) và không nhận gì khác ngoài tham số đã khai báo trong từng CR. **Tham số lạ bị từ chối** (`CODEINTEL_INVALID_PARAMS`, `data.field`), không bị bỏ qua im lặng. Trường `_trace` (đã được `extractResume` dùng) được phép.

Phần đầu kết quả (README v7 mục 3.2), do `buildCodeIntelResult` dựng:

```jsonc
{
  "sources": [
    { "tool": "gitnexus", "version": "1.6.9", "indexedAt": "2026-10-05T05:55:17.289Z", "commit": "d8198127b6bd14a3bf02dda1762ee13b7f3848ed" },
    { "tool": "codegraph", "version": "1.4.1", "indexedAt": "2026-10-05T12:33:43.367Z", "commit": null }
  ],
  "headCommit": "1b0c760935…",       // git rev-parse HEAD tại workspaceRoot; null nếu không đọc được
  "stale": true,                      // xem 2.6
  "truncated": false,
  "totalCount": 0,                    // tổng trước khi cắt; null nếu không biết
  "data": { }
}
```

Các trường bổ sung (không phá hợp đồng): `warnings: string[]` (ví dụ cảnh báo parse của CR-CV-002, `worktreeMismatch`); `toolTimingsMs: {gitnexus?: number, codegraph?: number}` cho CR-CV-071. `commit` của CodeGraph là `null` vì `codegraph status --json` không trả commit (đã đọc đầu ra; chưa kiểm chứng nguồn khác).

`codeintel.status` (method duy nhất do CR này sở hữu đầy đủ):

```jsonc
// request
{ "jsonrpc":"2.0", "id": 7, "method":"codeintel.status", "params": { "workspaceRoot": "/opt/repos/orca" } }
// result.data
{
  "binding": {
    "workspaceRoot": "/opt/repos/orca",
    "repoRoot": "/opt/repos/orca",          // đường dẫn đã đăng ký trong registry GitNexus
    "linkedWorktree": false,
    "worktreeMismatch": false,              // true khi index thuộc checkout chính, không phải workspaceRoot
    "gitnexus": { "name": "orca", "path": "/opt/repos/orca", "storagePath": "/opt/repos/orca/.gitnexus" },
    "codegraph": { "projectPath": "/opt/repos/orca", "hasDatabase": true }
  },
  "tools": {
    "gitnexus":  { "available": true,  "version": "1.6.9", "supported": true,  "binary": "/usr/bin/gitnexus" },
    "codegraph": { "available": true,  "version": "1.4.1", "supported": true,  "binary": "/home/ubuntu/.local/bin/codegraph" }
  },
  "indexes": {
    "gitnexus":  { "state": "stale", "indexedCommit": "d8198127b6…", "indexedAt": "…", "stats": { "files": 20174, "nodes": 247556, "edges": 644157, "communities": 10252, "processes": 300 } },
    "codegraph": { "state": "ready", "indexedAt": "…", "stats": { "files": 15773, "nodes": 295910, "edges": 959095 }, "pendingChanges": { "added": 0, "modified": 0, "removed": 0 } }
  },
  "sqliteReadAvailable": true,
  "limits": { "toolTimeoutMs": 20000, "methodTimeoutMs": 25000, "toolMaxOutputBytes": 16777216, "resultMaxBytes": 8388608, "maxConcurrentTools": 3 }
}
```

`indexes.<tool>.state` ∈ `missing | building | ready | stale | unknown`. Phần `indexes.gitnexus` do probe của CR-CV-002 điền, `indexes.codegraph` do CR-CV-003; CR này cung cấp probe mặc định trả `{"state":"unknown"}` để `status` chạy được trước khi hai CR kia xong.

### 2.3 Whitelist lệnh: kiểu có cấu trúc, không có API argv tự do

Nguyên tắc: **không có hàm nào trong module codeintel nhận `string[]` argv từ bên ngoài.** Mỗi lệnh là một đối tượng có kiểu; `codeintel-command-whitelist.ts` là nơi duy nhất sinh argv.

```ts
// codeintel-command-whitelist.ts (mới) — chỉ minh hoạ hình dạng
export type GitNexusCommand =
  | { verb: 'cypher'; query: string }                                   // query đã qua codeintel-cypher-guard (CR-CV-002)
  | { verb: 'context'; uid: string; limit?: number; content?: boolean }
  | { verb: 'context-by-name'; name: string; file?: string; limit?: number; content?: boolean }
  | { verb: 'impact'; uid: string; direction: 'upstream' | 'downstream'; depth: number; limit: number; summaryOnly?: boolean }
  | { verb: 'impact-by-name'; name: string; file?: string; kind?: string; direction: 'upstream' | 'downstream'; depth: number; limit: number }
  | { verb: 'detect-changes'; scope: 'unstaged' | 'staged' | 'all' | 'compare'; baseRef?: string }  // CR-CV-005, chỉ để đối chiếu

export type CodeGraphCommand =
  | { verb: 'status' }
  | { verb: 'query'; search: string; limit: number; kind?: string }
  | { verb: 'callers' | 'callees'; symbol: string; limit: number }
  | { verb: 'impact'; symbol: string; depth: number }
  | { verb: 'files'; filter?: string; maxDepth?: number }
  | { verb: 'affected'; files: string[]; depth?: number }
  | { verb: 'node'; name?: string; file?: string; offset?: number; limit?: number }
```

Quy tắc dựng argv:

- GitNexus luôn thêm `-r <registryPath>` (đường dẫn tuyệt đối lấy từ registry, **không** từ client); CodeGraph luôn thêm `-p <projectPath>` (riêng `status` dùng đối số vị trí `codegraph status <projectPath> -j`, vì `codegraph status --help` không có `-p`; đã chạy `codegraph status /opt/repos/orca -j` từ `/tmp` và được kết quả đúng; thư mục chưa khởi tạo trả `{"initialized":false,...,"lastIndexed":null}`).
- Mọi giá trị người dùng (tên symbol, uid, file, từ khoá tìm) được kiểm tra: độ dài ≤ 512, không chứa NUL hoặc ký tự điều khiển, **không bắt đầu bằng `-`** (chặn việc biến giá trị thành cờ). Chưa kiểm chứng việc hai CLI có hỗ trợ `--` kết thúc tuỳ chọn; vì vậy dùng quy tắc "không bắt đầu bằng `-`" thay vì dựa vào `--`.
- Cờ `--json`/`-j` bật cho mọi lệnh CodeGraph có hỗ trợ; `node` không có JSON nên đọc text.
- Biến môi trường tiến trình con: `config.toolEnv` cộng `NO_COLOR=1`. `codegraph status` (dạng text) vẫn in mã ANSI khi đi qua pipe (đã thấy `[1m`, `[36m` trong đầu ra), vì vậy chỉ dùng `--json` và, với `node`, loại ANSI bằng regex sau khi đọc.
- **Cấm tuyệt đối** trong mọi đường codeintel đọc: `analyze`, `clean`, `remove`, `uninstall`, `publish`, `setup`, `index`, `init`, `uninit`, `sync`, `serve`, `mcp`, `wiki`, `group`, `daemon`, `unlock`, `install`, `upgrade`, `telemetry`, `eval-server`, `check`. Hai lệnh làm mới (`analyze`, `sync`/`index`) chỉ tồn tại trong kiểu `ReindexCommand` của CR-CV-004, ở file riêng, không xuất qua `codeintel-command-whitelist.ts`.
- Test bắt buộc: bảng cố định "verb → argv mong đợi" (snapshot) và một test phản chứng quét mã nguồn `codeintel-*.ts` để chắc chắn không có chuỗi `'analyze'`/`'clean'`/`'remove'` ngoài `codeintel-reindex-*.ts`.

### 2.4 Phân giải repo từ `workspaceRoot` (O4)

`resolveCodeIntelRepo(workspaceRoot: string): Promise<CodeIntelRepoBinding>` thực hiện đúng thứ tự:

1. **Kiểu và hình dạng:** là string tuyệt đối (POSIX `/…` hoặc `X:\…` trên Windows; dùng `path.isAbsolute` của nền tảng đang chạy), không chứa NUL, độ dài ≤ 4096. Sai → `CODEINTEL_INVALID_PARAMS`.
2. **Chuẩn hoá:** `fs.realpath.native(workspaceRoot)` (giải symlink, chuẩn hoá hoa/thường trên macOS/Windows). Không tồn tại hoặc không phải thư mục → `CODEINTEL_PATH_NOT_ALLOWED` (không tiết lộ lỗi `ENOENT` chi tiết).
3. **Danh sách cho phép tuỳ chọn:** nếu biến môi trường `ORCA_CODEINTEL_ALLOWED_ROOTS` (danh sách ngăn bởi `path.delimiter`) được đặt, đường dẫn đã chuẩn hoá phải nằm trong một gốc. Mặc định không đặt = không giới hạn, **cùng mức tin cậy với `fs.*`/`git.exec`** (xem 1.3). Đây là điểm README v7 cần điều chỉnh.
4. **Là thư mục làm việc git:** chạy `git rev-parse --show-toplevel --git-common-dir` qua `GitCapabilityCache` với capability `rev-parse-path-format` đã có (`agent/src/shared/git-capability-cache.ts:9`), bản ưu tiên thêm `--path-format=absolute`, bản dự phòng bỏ cờ này và tự `path.resolve` (mẫu: `git-handler.ts` `readRepoLocation`, `:1514-1542`; `--path-format` cần Git 2.31, baseline là 2.25 theo `guides/reference/git-compatibility.md`). `toplevel` (realpath) phải bằng `workspaceRoot` đã chuẩn hoá, nếu không → `CODEINTEL_PATH_NOT_ALLOWED` kèm `data.hint = "workspaceRoot must be a git work tree root"`.
5. **Checkout chính của worktree liên kết:** `git worktree list --porcelain` (có từ Git 2.7); mục `worktree <path>` **đầu tiên** là checkout chính. Không dùng `-z` (Git 2.36). Nếu `toplevel` khác mục đầu → `linkedWorktree = true`.
6. **Khớp registry GitNexus** (`gitnexus-registry-reader.ts`): đọc `~/.gitnexus/registry.json` (đường dẫn `path.join(config.toolEnv.HOME, '.gitnexus', 'registry.json')`; biến môi trường để đổi vị trí của GitNexus: chưa kiểm chứng, không dựa vào). So `realpath(entry.path)` với `toplevel`; không khớp và `linkedWorktree` thì thử với checkout chính. Khớp ở checkout chính → `worktreeMismatch = true`. Không khớp cả hai → GitNexus "không có chỉ mục" cho repo này.
7. **CodeGraph:** gốc dự án là thư mục chứa `.codegraph/codegraph.db`, thử `toplevel` rồi checkout chính. Có thể lấy thêm từ `codegraph status --json` (`projectPath`, `worktreeMismatch.indexRoot`; CR-CV-003).
8. Nếu **cả hai** công cụ đều không có chỉ mục: `CODEINTEL_REPO_NOT_REGISTERED` (`data.hint = "run codeintel.reindex"` nếu cả hai binary có; CR-CV-004 cho phép tạo mới ở repo chưa đăng ký hay không là Câu hỏi mở Q2).

Kết quả được cache theo `workspaceRoot` 30 s (registry và worktree hiếm đổi); bị huỷ khi `codeintel.indexChanged` hoặc khi `reindex` kết thúc.

Registry hỏng hoặc không phải mảng → `CODEINTEL_TOOL_FAILED` với `data.reason = "registry_unreadable"`. Nhiều mục trùng `path` → chọn mục có `indexedAt` mới nhất và thêm cảnh báo.

### 2.5 Giới hạn tài nguyên

`codeintel-limits.ts` (mặc định; ghi đè qua biến môi trường `ORCA_CODEINTEL_*`, giá trị không hợp lệ bị bỏ qua kèm log cảnh báo):

| Giới hạn | Mặc định | Lý do |
|---|---|---|
| Timeout một lần chạy CLI | 20 000 ms | dưới 30 s mặc định của Go (`execTimeoutForMethod` trả 0 cho mọi method khác `agent.execPrompt`; CR-CV-023 có thể nâng) |
| Timeout toàn bộ một method | 25 000 ms | để agent tự trả `CODEINTEL_TIMEOUT` trước khi Go cắt kết nối chờ |
| Đầu ra tối đa của một tiến trình (stdout) | 16 MiB, vượt thì kill và `CODEINTEL_OUTPUT_TOO_LARGE` | bảo vệ bộ nhớ agent |
| Kích thước JSON kết quả cuối | 8 MiB | dưới khung 16 MiB (`frame.go:24`); vượt thì cắt dần mảng theo thứ tự ưu tiên của từng CR và đặt `truncated: true`; nếu vẫn vượt → `CODEINTEL_OUTPUT_TOO_LARGE` |
| stderr lưu lại | 64 KiB | chỉ để chẩn đoán |
| Số tiến trình công cụ chạy đồng thời toàn agent | 3 | ba lần gọi `gitnexus cypher` song song đã chạy được trên máy khảo sát; mỗi tiến trình mở DB 1,6 GB (`lbug`) nên giới hạn bộ nhớ |
| Hàng đợi chờ slot | tối đa 16, chờ tối đa 10 s, quá thì `CODEINTEL_TIMEOUT` (`data.reason = "queue_wait"`) | tránh tích tụ |
| Độ dài giá trị chuỗi từ client | 512 | 2.3 |

**Đầu ra GitNexus đi qua tệp tạm, không qua pipe (1.9).** `runCodeIntelTool` với `tool: 'gitnexus'` mở một tệp trong thư mục riêng `<os.tmpdir()>/orca-codeintel-<uid>/` (tạo bằng `mkdtemp`, quyền `0700`; tệp `0600`, cờ `wx`), spawn với `stdio: ['ignore', fd, 'pipe']`, đọc tệp sau khi tiến trình thoát rồi xoá (cả khi lỗi/timeout, trong `finally`). Trong lúc chạy, theo dõi kích thước tệp mỗi 500 ms; vượt `maxOutputBytes` thì kill và trả `CODEINTEL_OUTPUT_TOO_LARGE` (tránh đầy đĩa). Khi agent khởi động, dọn các thư mục `orca-codeintel-*` cũ hơn 1 giờ. Sau khi đọc, kiểm tra JSON có parse được; nếu không (đầu ra cụt) trả `CODEINTEL_TOOL_FAILED (reason = "truncated_stdout")`, không để dữ liệu cụt đi tiếp. CodeGraph dùng pipe thông thường.

Mở rộng `runToolCommand` (backward compatible, các tuỳ chọn mới đều không bắt buộc):

```ts
// agent-tool-registry.ts — chữ ký đề xuất
export function runToolCommand(
  binary: string, args: string[],
  opts: { cwd: string; timeout: number; env: NodeJS.ProcessEnv;
          maxOutputBytes?: number;   // (mới) vượt thì SIGTERM, resolve với meta.truncated = true
          stdoutFile?: string;       // (mới) ghi stdout ra tệp này thay vì pipe (GitNexus, 1.9); stdout trả về rỗng
          killGraceMs?: number;      // (mới) sau SIGTERM chờ rồi SIGKILL; mặc định 5000
          signal?: AbortSignal }     // (mới) huỷ chủ động (CR-CV-004)
): Promise<ToolResult>
```

`ToolResult.meta` (đã tồn tại, `Record<string, unknown>`) mang `{ truncated, timedOut, durationMs }`. Không đổi giá trị mặc định nên `tools/call` giữ nguyên hành vi.

Giới hạn riêng từng method (số nút, trang, ...) nằm ở CR sở hữu method; lớp nền chỉ áp trần chung ở trên.

### 2.6 `stale` và `headCommit`

`headCommit` = `git rev-parse HEAD` tại `workspaceRoot` (cache 5 s, vì chỉ là một lệnh git rẻ). `stale` của một nguồn = `indexedCommit !== headCommit` **hoặc** `worktreeMismatch`. `stale` của kết quả = OR của các nguồn thực sự được dùng. Quy tắc tính `indexedCommit` thuộc probe từng công cụ (CR-CV-002: `lastCommit` trong registry; CR-CV-003: CodeGraph không trả commit, nên chỉ dùng `pendingChanges` và `lastIndexed`; xem CR-CV-003 2.1).

### 2.7 Phát hiện công cụ và `capabilities`

`detectCodeIntelTools(config)`: với mỗi công cụ, xác định binary bằng cùng tập đường dẫn `config.toolPath` mà `discoverTools` dùng (`agent-tool-registry.ts:342`), rồi chạy `<binary> --version` (đo 0,06-0,07 s trên máy khảo sát) với timeout 5 s; cache 60 s. Phiên bản được kiểm bằng so sánh semver: **hỗ trợ GitNexus `>=1.6.0 <2`, CodeGraph `>=1.4.0 <2`** (đã thử `1.6.9` và `1.4.1`; dải rộng hơn là giả định, ghi ở mục 6). Ngoài dải: `supported: false`, các method trích xuất trả `CODEINTEL_TOOL_UNAVAILABLE` với `data.reason = "unsupported_version"`.

Trên Windows: `resolveToolBinary` không xử lý `PATHEXT`/`.cmd`, và `spawn` với `shell:false` không chạy được shim `.cmd`; MVP ghi `CODEINTEL_TOOL_UNAVAILABLE (data.reason = "unsupported_platform")` khi `process.platform === 'win32'` (chưa kiểm chứng trên Windows, mục 6).

`capabilities` trong `agent.handshake` (chuỗi tự do phía Go): thêm `codeintel` khi ít nhất một binary có trong `toolPath`, `codeintel.gitnexus` / `codeintel.codegraph` theo từng binary. Chỉ kiểm tra sự tồn tại binary (rẻ, đồng bộ với `discoverTools`), **không** chạy `--version` trong đường handshake (đã bị đua với timeout 5 s). `STATIC_CAPABILITIES_FALLBACK` **không** thêm `codeintel*` (không biết chắc binary có không). `tools` trong handshake giữ nguyên là danh sách tên của `discoverTools`.

### 2.8 Mã lỗi

`codeintel-errors.ts` quy đổi mọi lỗi thành JSON-RPC error. `error.code` là số (dùng `AgentErrorCode` có sẵn trong `agent/src/shared/agent-wire-protocol.ts`); khoá phân biệt cho backend là `error.data.code` (chuỗi), đúng README v7 mục 3.3.

| `data.code` | `error.code` | Khi nào | `data` bổ sung | `retryable` |
|---|---|---|---|---|
| `CODEINTEL_INVALID_PARAMS` | `-32602` (`InvalidParams`) | tham số thiếu/sai kiểu/vượt biên/lạ | `field`, `reason` | không |
| `CODEINTEL_PATH_NOT_ALLOWED` | `-33002` (`PermissionDenied`) | `workspaceRoot` không hợp lệ, không phải gốc worktree, ngoài `ORCA_CODEINTEL_ALLOWED_ROOTS` | `hint` | không |
| `CODEINTEL_TOOL_UNAVAILABLE` | `-32000` | không có binary, phiên bản ngoài dải, nền tảng chưa hỗ trợ | `tool`, `reason` | không |
| `CODEINTEL_REPO_NOT_REGISTERED` | `-32000` | không khớp registry GitNexus và không có DB CodeGraph | `hint` | không |
| `CODEINTEL_INDEX_MISSING` | `-32000` | repo có công cụ nhưng chỉ mục của công cụ cần dùng chưa có | `tool`, `hint` | không |
| `CODEINTEL_AMBIGUOUS_SYMBOL` | `-32000` | `status:"ambiguous"` (CR-CV-002) | `candidates[]` (`uid,name,kind,filePath,line`) | không |
| `CODEINTEL_TIMEOUT` | `-32000` | quá 20 s/25 s hoặc chờ slot | `tool`, `reason`, `elapsedMs` | có |
| `CODEINTEL_REINDEX_IN_PROGRESS` | `-32000` | đang `analyze`/`sync` cho repo này (CR-CV-004) | `jobId` | có |
| `CODEINTEL_OUTPUT_TOO_LARGE` | `-32000` | stdout > 16 MiB hoặc kết quả > 8 MiB sau khi cắt tối đa | `bytes`, `limit` | không |
| `CODEINTEL_TOOL_FAILED` | `-32000` | exit code khác 0, `{"error": ...}` trong stdout (xem dưới), JSON hỏng | `tool`, `exitCode`, `stderrTail` (≤ 2 KiB, đã loại đường dẫn HOME) | không |

**Điểm quan trọng khi đọc lỗi công cụ:** `gitnexus cypher` báo lỗi truy vấn bằng `{"error": "..."}` trên stdout và **thoát với mã 0** (đã chạy thử: truy vấn ghi `CREATE`, bảng không tồn tại, tham số `$id` không có đều thoát 0). Vì vậy `runCodeIntelTool` phải kiểm tra nội dung stdout, không chỉ `exitCode`. Thiếu `-r` thì Node ném lỗi ra stderr (đã thấy, mã thoát khác 0 chưa được ghi lại; chưa kiểm chứng mã chính xác).

`CodeIntelError` không bao giờ chứa stack hay đường dẫn ngoài `workspaceRoot`/registry path trong `message` gửi đi; chi tiết đầy đủ chỉ vào log agent (`AgentLogger`).

### 2.9 Vì sao KHÔNG mở lại tool `gitnexus`/`codegraph` có `args` tự do ra backend

1. **`args` tự do = chạy lệnh ghi.** `gitnexus analyze` ghi lại `AGENTS.md`, `CLAUDE.md` và cài skill trong repo nếu không có `--index-only` (`gitnexus analyze --help`; `git status` của chính repo này đang có `M AGENTS.md`, `M CLAUDE.md`); `clean`/`remove` xoá chỉ mục. Một client chỉ cần quyền gọi `tools/call` là gây hư hại.
2. **Không có chọn repo an toàn.** Tool cũ không truyền `-r`; với 12 repo trong registry, `cwd` không đủ để GitNexus chọn repo. Mở `args` thì client tự điền `-r` và đọc được chỉ mục repo khác (rủi ro lộ dữ liệu liên dự án, xem CR-CV-072).
3. **Đầu ra không giới hạn và là text.** Không cap stdout (1.2); `detect-changes` chỉ in tối đa 15 symbol (CR-CV-005); `cypher` trả markdown (CR-CV-002).
4. **Không có kiểu.** Backend Go sẽ phải hiểu argv của hai CLI và chịu sự thay đổi phiên bản của chúng.

Trung thực về ranh giới bảo mật: agent đã có `tools/call name=shell` (`bash -c` tuỳ ý, `agent-tool-registry.ts` tool `shell`), `shell.exec` và `git.exec`. Whitelist `codeintel.*` **không làm agent an toàn hơn về tổng thể**; nó đảm bảo (a) đường code-intel của backend/UI không cần và không dùng đường tuỳ ý, và (b) chính sách phía backend (CR-CV-013, 072) có thể chặn `tools/call`, `shell.*` cho vai trò chỉ-xem-code mà vẫn dùng được `codeintel.*`. Quyết định: **giữ** tool `gitnexus`/`codegraph` cũ (có thể có client khác dùng; chưa kiểm chứng) nhưng đề xuất hai việc phụ, nằm trong CR này: (i) `tools/call` từ chối `name ∈ {gitnexus, codegraph}` khi `args[0]` thuộc tập cấm ghi ở 2.3 (xoá khả năng chạy `analyze/clean/remove` qua đường cũ), (ii) tài liệu hoá rằng `code-intel-service` không được gọi `tools/call`. Nếu bước (i) bị coi là phá tương thích, chuyển thành Câu hỏi mở Q3.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Dispatcher riêng `agent-rpc-dispatch-codeintel.ts`, bảng method ở `codeintel-method-table.ts` | Mẫu của các `agent-rpc-dispatch-*.ts`; `agent-rpc-dispatch.ts` đã 419 dòng, chỉ thêm vài dòng |
| Kiểu lệnh có cấu trúc thay vì kiểm tra argv bằng danh sách | Không có đường nào để chèn cờ; test snapshot đơn giản |
| `-r <đường dẫn tuyệt đối từ registry>` thay vì tên repo | Đã chạy thử chấp nhận đường dẫn; tránh nhập nhằng khi có alias trùng (`--allow-duplicate-name` của `analyze`) |
| Đọc `registry.json` thay vì phân tích `gitnexus list` | `list` chỉ in text (đã chạy); file là JSON ổn định hơn. Rủi ro: định dạng file nội bộ của GitNexus (mục 6) |
| Worktree liên kết gắn vào checkout chính kèm cờ `worktreeMismatch` | Cả hai CLI tự làm vậy (1.5); UI cần biết để cảnh báo, không cần ngừng dịch vụ |
| Kết quả luôn là object | `Client.Exec` giải mã vào `map[string]any` (1.7) |
| Thêm `capabilities` chỉ theo sự tồn tại binary | Handshake bị đua với timeout 5 s (1.6) |
| Lỗi có `data.code` chuỗi, số `error.code` dùng lại bộ có sẵn | Backend phân nhánh theo chuỗi; không thêm số mới vào `AgentErrorCode` |
| `workspaceRoot` phải là gốc worktree | Chặn trỏ vào `/` hoặc thư mục con tuỳ ý; một quy tắc duy nhất cho mọi method |

## 4. Tiêu chí chấp nhận

- [ ] Gọi `codeintel.status` với `workspaceRoot` hợp lệ trên agent Part A trả đúng phần đầu chung (2.2), `binding`, `tools`, `limits`; không có trường `args`/tên lệnh CLI ở bất kỳ đâu trong đầu vào hay đầu ra.
- [ ] Method `codeintel.*` không tồn tại trả `-32601` (`MethodNotFound`); tham số lạ trả `CODEINTEL_INVALID_PARAMS` với `data.field`.
- [ ] `workspaceRoot` tương đối, chứa NUL, không tồn tại, là thư mục con của repo, hoặc ngoài `ORCA_CODEINTEL_ALLOWED_ROOTS` đều trả `CODEINTEL_PATH_NOT_ALLOWED` (hoặc `INVALID_PARAMS` cho lỗi hình dạng) và **không** spawn bất kỳ tiến trình CLI nào.
- [ ] Repo chưa có trong registry và không có DB CodeGraph trả `CODEINTEL_REPO_NOT_REGISTERED`; repo trong registry khác không bao giờ bị chạm (test với registry giả 2 repo).
- [ ] Worktree liên kết của một repo đã đăng ký trả `linkedWorktree: true`, `worktreeMismatch: true`, `stale: true`.
- [ ] Mọi lần chạy GitNexus có `-r <registryPath>`; mọi lần chạy CodeGraph có `-p <projectPath>` (trừ `status` dùng đối số vị trí). Test snapshot argv xanh.
- [ ] Không có hàm xuất khẩu nào của `codeintel-*.ts` (trừ `codeintel-reindex-*.ts`, CR-CV-004) sinh argv chứa `analyze`, `clean`, `remove`, `sync`, `index`; test quét nguồn xanh.
- [ ] Truy vấn GitNexus có đầu ra > 256 KB (ví dụ 3 000 hàng `CALLS`) trả JSON đầy đủ, `row_count` khớp số hàng đã parse; không có đường nào dùng pipe cho `gitnexus`.
- [ ] Tệp tạm được xoá sau mỗi lần chạy (thành công, lỗi, timeout, huỷ) và thư mục tạm có quyền `0700`.
- [ ] Stdout vượt 16 MiB: tiến trình bị kill, trả `CODEINTEL_OUTPUT_TOO_LARGE`, không tăng bộ nhớ quá ngưỡng đã cấu hình trong test.
- [ ] Tiến trình quá 20 s bị `SIGTERM` rồi `SIGKILL` sau 5 s, trả `CODEINTEL_TIMEOUT`.
- [ ] `{"error":"..."}` với exit code 0 từ `gitnexus cypher` được quy đổi thành `CODEINTEL_TOOL_FAILED`.
- [ ] 4 lệnh `codeintel.*` đồng thời: tối đa 3 tiến trình chạy cùng lúc, lệnh còn lại chờ hoặc `CODEINTEL_TIMEOUT (queue_wait)`.
- [ ] `agent.handshake` có `codeintel`, `codeintel.gitnexus`, `codeintel.codegraph` khi binary có trong `toolPath`; không có khi vắng; `STATIC_CAPABILITIES_FALLBACK` không đổi.
- [ ] `tools/call name=gitnexus args=["analyze"]` bị từ chối (nếu chấp nhận 2.9(i)); `tools/call` các tool khác không đổi hành vi.
- [ ] Trace của `codeintel.*` không chứa tham số ngoài `workspaceRoot`.
- [ ] Không có tên file `helpers`/`utils`/`common`/`misc`; không có `max-lines` disable mới; `pnpm lint` và `pnpm test` trong `agent/` xanh.

## 5. Kiểm thử

Vitest (`agent/vitest.config.ts` nhận `src/**/*.test.ts`). Chưa chạy gì ở thời điểm viết CR.

| File test (mới) | Nội dung |
|---|---|
| `agent/src/relay/agent-rpc-dispatch-codeintel.test.ts` | Định tuyến: method lạ → `null`/`MethodNotFound`; tham số lạ; hình dạng phản hồi JSON-RPC; mock `MockWs` như `agent-rpc-dispatch-misc.test.ts` |
| `agent/src/relay/codeintel-errors.test.ts` | Bảng quy đổi mã (2.8), `data` bị lọc đường dẫn HOME |
| `agent/src/relay/codeintel-command-whitelist.test.ts` | Snapshot argv từng verb; giá trị bắt đầu bằng `-`/NUL/quá dài bị từ chối; test quét nguồn cấm `analyze/clean/remove` |
| `agent/src/relay/codeintel-tool-runner.test.ts` | Dùng script Node giả làm binary (qua `PATH` trong thư mục tạm): đầu ra lớn, timeout + SIGKILL, `{"error"}` exit 0, stderr cắt; có kiểm tra không rò tiến trình mồ côi; chế độ `stdoutFile` với script ghi 5 MB rồi `process.exit(0)` ngay (mô phỏng lỗi cụt pipe), tệp tạm bị xoá, quyền thư mục |
| `agent/src/relay/gitnexus-registry-reader.test.ts` | Registry hợp lệ, rỗng, hỏng, trùng `path`, đường dẫn symlink |
| `agent/src/relay/codeintel-repo-resolution.test.ts` | Dùng repo git thật trong thư mục tạm: gốc, thư mục con, worktree liên kết (`git worktree add`), không phải git; hai nhánh `rev-parse-path-format` (bản ưu tiên và bản dự phòng bằng cách giả `git` echo cờ lạ, theo mẫu `git-handler-worktree-git-capabilities.test.ts`) |
| `agent/src/relay/codeintel-tool-detection.test.ts` | Có/không binary, ngoài dải phiên bản, cache 60 s, nền tảng win32 giả |
| `agent/src/relay/codeintel-concurrency-gate.test.ts` | Giới hạn 3, hàng đợi 16, thời gian chờ |
| `agent/src/relay/agent-session-capabilities-codeintel.test.ts` | `buildCapabilities` có/không `codeintel*`; fallback không đổi |
| `agent/src/relay/__tests__/agent-tool-registry.test.ts` (sửa) | Tuỳ chọn mới của `runToolCommand`; hành vi mặc định không đổi; `tools/call` từ chối động từ ghi |

Git compatibility: test chạy với Git baseline 2.25 nếu CI hiện có ma trận; cần đối chiếu với job "git-compat" đang có (chưa kiểm chứng tên job) trước khi thêm.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Định dạng `~/.gitnexus/registry.json` là nội bộ của GitNexus.** Đã đọc ở 1.6.9; không có tài liệu cam kết. CR-CV-070 phải có fixture và test hợp đồng với phiên bản; nếu đọc lỗi thì quy về `CODEINTEL_TOOL_FAILED (registry_unreadable)`.
- **Vị trí registry có thể bị đổi bằng biến môi trường của GitNexus** (chưa kiểm chứng tên biến). Nếu agent chạy dưới user khác (systemd) với `HOME` khác, registry sẽ khác; `toolEnv.HOME` đã được `loadAgentConfig` đặt cố định.
- **Index của worktree liên kết thuộc checkout chính.** Với việc review code do agent viết trong worktree riêng, `worktreeMismatch`/`stale` sẽ là trạng thái phổ biến, và số dòng trong chỉ mục không khớp tệp ở worktree. `gitnexus analyze --branch <name>` ("per-branch index slot") và `analyze` chạy riêng trong worktree có thể giải được; **chưa kiểm chứng** và thuộc CR-CV-004/Q2.
- **Telemetry của CodeGraph** (`codegraph telemetry` tồn tại, mặc định chưa kiểm tra): mỗi lần spawn trên dev server có thể gửi dữ liệu; chưa kiểm chứng. Nên kiểm tra `codegraph telemetry status` trên dev server thật và, nếu bật, đặt biến môi trường tắt (tên biến chưa kiểm chứng).
- **Windows**: chưa kiểm chứng; MVP chặn bằng `unsupported_platform`. macOS: `realpath.native` chuẩn hoá hoa/thường, chưa chạy thử.
- Dải phiên bản `>=1.6.0 <2` / `>=1.4.0 <2` là giả định (chỉ đã thử 1.6.9 và 1.4.1).
- `ORCA_CODEINTEL_ALLOWED_ROOTS` mặc định không đặt: nếu coi agent là ranh giới tin cậy chứ không phải backend, mặc định này quá rộng; xem Q1.
- Giới hạn "3 tiến trình" chưa đo trên dev server nhỏ (RAM). `lbug` ~1,6 GB trên đĩa nhưng mức RAM khi mở chưa đo.
- Nguyên nhân cụt stdout của `gitnexus` (1.9) chưa rõ và có thể khác theo phiên bản Node/GitNexus; kiểm thử hợp đồng (CR-CV-070) phải có ca đầu ra lớn. Tệp tạm trên dev server dùng chung có rủi ro rò dữ liệu nếu `os.tmpdir()` là thư mục chia sẻ; vì vậy thư mục `0700` theo uid.
- Sửa `runToolCommand` dùng chung với `tools/call`: tác động được đánh giá bằng đọc code (các handler `claude_code`, `gh`, `git`, `gitnexus`, `codegraph`, `docker`, `shell` đều gọi hàm này với tham số cũ); cần chạy `gitnexus impact`/test hiện có (`agent/src/relay/__tests__/agent-tool-registry.test.ts`) trước khi merge.

## 7. Câu hỏi mở

- **Q1.** Mặc định `ORCA_CODEINTEL_ALLOWED_ROOTS` rỗng (parity với `fs.*`) có đủ không, hay agent nên tự đọc gốc workspace từ cấu hình đã có (`AGENT_WORK_DIR`, thư mục cha của nó)? Đề xuất mặc định hiện tại: rỗng, ranh giới thật là registry GitNexus + phân quyền backend.
- **Q2.** Có cho `codeintel.reindex` đăng ký repo mới vào registry (chạy `analyze` lần đầu) không, hay chỉ làm mới repo đã có? Liên quan 2.4(8) và CR-CV-004.
- **Q3.** Chặn `analyze/clean/remove` trong `tools/call` có phá client hiện có không (chưa kiểm chứng ai gọi)?
- **Q4.** `codeintel.status` có nên chạy `git status --porcelain` để báo working tree bẩn không? Chi phí trên repo lớn chưa đo; hiện bỏ ra, CR-CV-005 tự tính.
- **Q5.** README v7 cần thêm các method `codeintel.watch`, `codeintel.reindexCancel`, `codeintel.reindexStatus` (CR-CV-004) và cho phép `percent: null`; xem mục "Điều chỉnh hợp đồng" trong báo cáo của feature.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 2 D1, D5; mục 3.2, 3.3; mặc định O4)
- `/opt/repos/orca/docs/research/view-code/02-local-mcp-interaction.md`, `03-command-and-data-flow.md`, `07-architecture-decisions.md`
- `/opt/repos/orca/agent/src/relay/agent-rpc-dispatch.ts` (`route()` `:297`, `extractTraceFields` `:92`, `makeError` `:408`, `makeNotifier` `:280`)
- `/opt/repos/orca/agent/src/relay/agent-rpc-dispatch-misc.ts` (mẫu dispatcher, `tools/call`)
- `/opt/repos/orca/agent/src/relay/agent-tool-registry.ts` (`runToolCommand` `:72`, `resolveToolBinary` `:54`, tool `gitnexus` `:185`, `codegraph` `:205`, `discoverTools` `:342`)
- `/opt/repos/orca/agent/src/relay/agent-session-handshake.ts` (`:63-70`), `/opt/repos/orca/agent/src/relay/agent-session-capabilities.ts` (`:58`, `:105`)
- `/opt/repos/orca/agent/src/relay/git-handler.ts` (`readRepoLocation` `:1514`), `/opt/repos/orca/agent/src/shared/git-capability-cache.ts`, `/opt/repos/orca/agent/src/shared/git-worktree-command-capabilities.ts`
- `/opt/repos/orca/agent/src/relay/context.ts` (`registerRoot` no-op), `/opt/repos/orca/agent/src/shared/agent-wire-protocol.ts` (`AgentErrorCode`)
- `/opt/repos/orca/agent/src/relay/agent-entry.ts` (`--stdio` `:80`), `/opt/repos/orca/agent/src/relay/agent-connection-stdio.ts`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go` (`:412`, `:424-449`), `jsonrpc.go`, `frame.go`
- `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/AGENTS.md`
- `/opt/repos/orca/specs/agent/api/gaps-and-findings.md` (Part A/B), `/opt/repos/orca/specs/agent/api/compliance-audit-2026-08-15.md` §1
