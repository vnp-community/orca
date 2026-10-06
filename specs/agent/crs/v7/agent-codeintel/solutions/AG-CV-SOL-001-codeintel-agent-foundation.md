# AG-CV-SOL-001: Nền `codeintel.*` trên agent (dispatcher, whitelist, phân giải repo, giới hạn, mã lỗi, capability)

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mọi mục "đã đọc" là đọc code, chưa chạy gì. Không có dòng nào của solution này đã được thực thi.

**CR:** [CR-CV-001](../../../../../../docs/crs/v7/agent-codeintel/CR-CV-001-codeintel-agent-foundation.md)
**Service:** `agent/`, thư mục `agent/src/relay/` (Part A, `agent-rpc-dispatch*.ts`); Part B ở [AG-CV-SOL-006](./AG-CV-SOL-006-relay-ssh-part-b-handlers.md)
**TDD tham chiếu:** [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md) mục 1 (router), 5 (mã lỗi), 9.1 (`makeNotifier`); [v5/04-handshake-session](../../../../tdd/v5/04-handshake-session.md) mục 6 (tools trong handshake), 7 (capabilities); [v5/05-tool-registry](../../../../tdd/v5/05-tool-registry.md) mục 2 (`discoverTools`), 3 (định nghĩa tool); [v5/02-wire-protocol](../../../../tdd/v5/02-wire-protocol.md) mục 1 (hằng, `AgentErrorCode`); [v5/03-connection-modes](../../../../tdd/v5/03-connection-modes.md) mục 3 (`createSession`). API: `specs/agent/api/agent-rpc-catalog-runtime.md`, `specs/agent/api/gaps-and-findings.md` mục 4 (Part A so với Part B)
**Mẫu định dạng:** `specs/agent/crs/v6/agent-capabilities/solutions/AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md`

## 0. Hợp đồng áp dụng

| Nguồn | Mục | Nội dung solution này hiện thực |
|---|---|---|
| `CONTRACT-codeintel-agent-rpc.md` | §1.1–1.3 | chế độ truyền tải, Windows `unsupported_platform`, capability `codeintel`, `codeintel.gitnexus`, `codeintel.codegraph` |
| | §2.1 | `workspaceRoot` bắt buộc, tham số lạ bị từ chối, cấm `args/argv/command/cmd/cwd/env/repo/cypher/shell/timeout/tool`, quy tắc chuỗi và đường dẫn tương đối |
| | §2.2 | phong bì `{sources, headCommit, stale, truncated, totalCount, warnings, perf, data}`, `lineBase` |
| | §2.3 | bảng giới hạn và biến `ORCA_CODEINTEL_*` |
| | §2.4 | `spawn(shell:false)`, đối tượng lệnh có kiểu, `-r`/`-p`, stdout GitNexus qua tệp tạm, danh sách cấm |
| | §2.5 | timeout agent (25 s/20 s) luôn trước timeout Go |
| | §3.1–3.2 | `error.data.code`, bảng mã lỗi do agent sinh |
| | §4.1 | `codeintel.status` (phần nền; trường CR-080 do AG-CV-SOL-080) |
| | §9 mục 1–4, 6 | `TestWhitelistIsClosed`, `TestNoForbiddenSubcommand`, `TestUserValuesNeverStartWithDash`, `TestRepoFlagIsLast`, `TestSpawnNeverUsesShell`, `TestParamsStrict`; phân giải repo; secret; tải |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-02, PQ-03, PQ-13, PQ-18, PQ-19, PQ-20, PQ-21, PQ-23; §7.2 (thứ tự agent); §8.3 mục 1, 5; §9 O-14; §10 (dòng `agent-wire-protocol.ts`, `agent-session-capabilities.ts`, `agent-rpc-dispatch.ts`) | |
| `CONTRACT-codeintel-ui-api.md` | không trực tiếp (agent không biết kênh UI) | |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `agent/src/relay/agent-rpc-dispatch.ts` (`extractTraceFields` `:92-201`, `createRpcDispatcher` `:217-267`, `makeNotifier` `:280-290`, `route()` `:297-382`, `makeError` `:408-419`), `agent-tool-registry.ts` (`resolveToolBinary` `:54-66`, `runToolCommand` `:72-113`, tool `gitnexus` `:184-203`, `codegraph` `:206-224`, `discoverTools` `:342-367`), `agent-session-capabilities.ts` (cả file), `agent-session-handshake.ts` (`:30-85`), `agent-session.ts` (`stop()` `:226-254`), `agent-config.ts` (`loadAgentConfig` `:78-`, `toolEnv` `:81-88`), `shared/agent-wire-protocol.ts` (`AgentErrorCode` `:35-52`, `AgentCapability` `:58`), `shared/git-capability-cache.ts`, `shared/git-worktree-command-capabilities.ts` (`:26`, `:32`), `git-handler.ts` (`readRepoLocation` `:1514-1542`), `agent-git-handler-extended.ts` (`git()` `:42-57`), `agent-rpc-dispatch-misc.ts` (đầu file, `tools/call` `:44-69`), `agent-rpc-dispatch-misc.test.ts` (khuôn `MockWs`), `context.ts` (`registerRoot` no-op `:30-34`), `agent-entry.ts` (`--stdio`/`--detach` `:80-108`), `agent-connection-stdio.ts` (`:262-280`), `agent-logger.ts`, `external-automations-handler.ts` (`:490-525`), `vitest.config.ts`, `package.json` (script `test`), cây `desktop/src/relay/` (`agent-tool-registry.ts` giống hệt bản `agent/` ở 400 dòng đầu).

| Điểm | Hiện trạng thật | Hệ quả cho solution |
|---|---|---|
| Router | `route()` thử lần lượt 15 `dispatchXxxRpc`, hết thì `makeError(rpc.id, MethodNotFound, …)` (`:381`) | thêm đúng một khối `dispatchCodeIntelRpc` ngay trước dòng đó |
| `makeError` | giữ `data` (`:417`) | `error.data.code` đến được Go; Part B thì không (SOL-006) |
| `makeNotifier` | trả `() => void`, bỏ nếu `ws.readyState !== 1` (`:285`) | sink thông báo của SOL-004 dựa vào đây |
| `runToolCommand` | `{cwd, timeout, env}`; mảng chuỗi không cap; hết hạn chỉ `SIGTERM` rồi resolve `exitCode 124` (`:89-96`); `child.stdin?.end()` ngay (`:111`) | cần mở rộng tuỳ chọn tương thích ngược (task 04) |
| `resolveToolBinary` | private, tách `toolPath` theo `':'` (`:55`) | không dùng trên Windows; Windows bị chặn `unsupported_platform` |
| Tool `gitnexus`/`codegraph` | `args` tự do, `cwd` tự do, `timeout 60_000`, `env: config.toolEnv` (`:197-201`, `:219-223`) | giữ tool, nhưng chặn động từ ghi (task 04) |
| `toolEnv` | `{...process.env, PATH, HOME, ANTHROPIC_API_KEY, GITHUB_TOKEN, GH_TOKEN}` (`agent-config.ts:81-88`) | con của codeintel không được thừa kế bí mật (task 03, mục 2.6) |
| `buildCapabilities` | mảng chuỗi tự do, có `checkGitAvailable` quét `config.toolPath.split(':')` (`:22`) | thêm hàm kiểm tồn tại binary, không chạy `--version` |
| `AgentCapability` | union `'pty'\|'fs'\|'git'\|'preflight'` chỉ dùng cho kiểu `AgentHandshakeParams` (grep: không nơi nào khác trong `agent/src`, `desktop/src`) | thêm bốn literal, additive, không đổi hành vi |
| Handshake | `tools: tools.map(t => t.name)` (`agent-session-handshake.ts:70`) | giữ nguyên (PQ-18: backend thêm `Tools`, không phải agent) |
| `registerRoot` | no-op (`context.ts:31-33`) | ranh giới là `workspaceRoot` tự kiểm (contract §9 mục 3) |
| Test | `vitest.config.ts` nhận `src/**/*.test.ts`; chỉ có thư mục `__tests__/` cho một số test cũ; chưa có thư mục `__fixtures__` trong `agent/src/relay` | test đặt cạnh nguồn; fixture ở `agent/src/relay/codeintel/__fixtures__/` (AG-CV-SOL-070) |
| CI | không chạy test của `agent/` (README v7 mục 8 điểm 16/20, theo contract §10) | job `code-intel-contract` thuộc AG-CV-SOL-070; solution này chỉ ghi lệnh vitest cục bộ |
| `desktop/src/relay/` | có `agent-tool-registry.ts`, `agent-config.ts` (`loadAgentConfig()` không tham số), `git-capability-cache` ở `desktop/src/shared/`; không có `agent-rpc-dispatch*.ts` | bản Part A chỉ ở `agent/` (F1 của README feature) |

### Correction relative to CR

| # | CR-CV-001 nói | Thực tế / quyết định |
|---|---|---|
| 1 | `codeintel-errors.ts` có `toJsonRpcError(id, err)` | CR-CV-006 cấm lõi import `makeError`. Đổi thành `toErrorPayload(err)` thuần (`{code, message, data}`); `makeError` chỉ được gọi ở `agent-rpc-dispatch-codeintel.ts` |
| 2 | `toolTimingsMs` | thay bằng `perf` (PQ-19; contract §2.2) |
| 3 | `status` 100% do CR này | các trường `indexScope`, `freshness`, `indexRoot`, `mergeBase`, `dirtySinceIndex`, `changedFilesNotInIndex`, `rootMismatch` thuộc `classifyIndexBasis` của AG-CV-SOL-080; solution này để điểm cắm (`StatusIndexEnricher`) |
| 4 | 10 mã lỗi | contract §3.2 có 17 mã agent; `CodeIntelErrorCode` khai đủ ngay (thêm `CODEINTEL_SYMBOL_NOT_FOUND`, `CODEINTEL_PROFILE_UNKNOWN`, `CODEINTEL_ENV_NOT_READY`, `CODEINTEL_RUN_IN_PROGRESS`, `CODEINTEL_RUN_NOT_FOUND`, `CODEINTEL_RUN_CANCELLED`) để AG-CV-SOL-081 dùng chung một bảng |
| 5 | `CodeIntelRequestContext.config: AgentConfig` | giữ; thêm `deadline` (hạn tuyệt đối) để mọi method tự cắt trước 25 s |
| 6 | kiểm `git rev-parse` bằng `GitCapabilityCache` | đúng; nhưng `readRepoLocation` là phương thức riêng của `GitHandler` (không xuất): viết lại gọn trong file mới, tái dùng `isUnsupportedRevParsePathFormatError`, `hasUnsupportedRevParsePathFormatEcho` (đã xuất ở `shared/git-worktree-command-capabilities.ts`) |

### Lệch giữa CR và hợp đồng

| # | CR-CV-001 | Hợp đồng (thắng) |
|---|---|---|
| 1 | `CODEINTEL_INVALID_PARAMS (reason="not_found")` | `CODEINTEL_SYMBOL_NOT_FOUND` -32602 (PQ-03); `INVALID_PARAMS` chỉ cho tham số sai |
| 2 | `toolTimingsMs` | `perf` (PQ-19) |
| 3 | whitelist GitNexus chỉ `cypher/context/impact/detect-changes` | contract §9 mục 1 liệt kê tập đóng rộng hơn (`query, context, impact, trace, cypher, list, status, detect-changes` + `check --cycles`; CodeGraph thêm `explore`). Solution chỉ cài verb có method dùng; test `TestWhitelistIsClosed` khẳng định tập hiện có là **tập con** của tập hợp đồng. Mâu thuẫn nhỏ: không method nào cần `query/trace/list/status/explore`; ghi ở mục 8 |
| 4 | `error.code` -32000 cho `CODEINTEL_INDEX_MISSING` v.v. | đúng, giữ; thêm `-32602` cho `SYMBOL_NOT_FOUND`, `PROFILE_UNKNOWN`, `RUN_NOT_FOUND` |
| 5 | `status` cho Windows | contract §1.1: mọi method trả `TOOL_UNAVAILABLE reason="unsupported_platform"` (kể cả `status`) dù §4.1 nói "luôn thành công khi agent chạy"; theo §1.1 (cụ thể hơn). Ghi mục 8 |
| 6 | không nói env con | contract §2.4: `config.toolEnv + NO_COLOR=1`; đồng thời README v7 mục 8 điểm 22 coi `toolEnv` chứa toàn bộ `process.env` là vấn đề cho CR-072. Solution chọn lọc biến theo mẫu bí mật cho con codeintel (task 03); cần chủ hợp đồng xác nhận (mục 8, câu 3) |

## 2. Giải pháp

### 2.1 Cây file

```
agent/src/relay/
  agent-rpc-dispatch-codeintel.ts            (mới) dispatchCodeIntelRpc: null nếu method không bắt đầu 'codeintel.'
  codeintel-method-table.ts                  (mới) CODEINTEL_METHODS (một chỗ đăng ký duy nhất)
  codeintel-errors.ts                        (mới) CodeIntelError, bảng mã, toErrorPayload
  codeintel-params-validation.ts             (mới) workspaceRoot, tham số lạ, chuỗi, đường dẫn tương đối, ref git
  codeintel-limits.ts                        (mới) hằng + đọc ORCA_CODEINTEL_*
  codeintel-concurrency-gate.ts              (mới) semaphore 3 toàn agent, gitnexus ≤2, codegraph ≤3, hàng ≤16
  codeintel-command-whitelist.ts             (mới) GitNexusCommand, CodeGraphCommand, buildArgv
  codeintel-child-env.ts                     (mới) buildCodeIntelChildEnv(config)
  codeintel-tool-runner.ts                   (mới) runCodeIntelTool
  codeintel-tool-output-classification.ts    (mới) {"error"} exit 0, JSON cụt, text ANSI của CodeGraph
  codeintel-secret-redaction.ts              (mới) che stderrTail (HOME, gh*_, AKIA…, sk-…, JWT, PEM, scheme://user:pass@)
  gitnexus-registry-reader.ts                (mới) đọc ~/.gitnexus/registry.json
  codeintel-repo-resolution.ts               (mới) resolveCodeIntelRepo
  codeintel-git-exec.ts                      (mới) execFile('git') có timeout/maxBuffer/signal, không phụ thuộc agent-git-handler-extended
  codeintel-tool-detection.ts                (mới) detectCodeIntelTools
  codeintel-result-envelope.ts               (mới) buildCodeIntelResult
  codeintel-head-commit.ts                   (mới) headCommit cache 5 s
  codeintel-status.ts                        (mới) handler codeintel.status + registerIndexProbe
  agent-rpc-dispatch.ts                      (sửa) route(): +1 khối; extractTraceFields(): +1 nhánh
  agent-tool-registry.ts                     (sửa) runToolCommand: tuỳ chọn mới; tool gitnexus/codegraph: chặn động từ ghi
  agent-session-capabilities.ts              (sửa) buildCapabilities: +3 capability có điều kiện
  ../shared/agent-wire-protocol.ts           (sửa) AgentCapability: +4 literal
  (test cạnh nguồn, mục 6)
```

Tên đều theo khái niệm cụ thể (AGENTS.md, không `helpers/utils/common/misc`), không `max-lines` disable (`agent-rpc-dispatch.ts` 419 dòng: chỉ thêm khoảng 8 dòng).

### 2.2 Mã lỗi và tải trọng lỗi (hợp đồng §3.1–3.2)

```ts
// codeintel-errors.ts (mới) — hình dạng chữ ký
export type CodeIntelErrorCode =
  | 'CODEINTEL_INVALID_PARAMS' | 'CODEINTEL_PATH_NOT_ALLOWED' | 'CODEINTEL_TOOL_UNAVAILABLE'
  | 'CODEINTEL_REPO_NOT_REGISTERED' | 'CODEINTEL_INDEX_MISSING' | 'CODEINTEL_SYMBOL_NOT_FOUND'
  | 'CODEINTEL_AMBIGUOUS_SYMBOL' | 'CODEINTEL_TIMEOUT' | 'CODEINTEL_REINDEX_IN_PROGRESS'
  | 'CODEINTEL_OUTPUT_TOO_LARGE' | 'CODEINTEL_TOOL_FAILED' | 'CODEINTEL_PROFILE_UNKNOWN'
  | 'CODEINTEL_ENV_NOT_READY' | 'CODEINTEL_RUN_IN_PROGRESS' | 'CODEINTEL_RUN_NOT_FOUND'
  | 'CODEINTEL_RUN_CANCELLED'

export class CodeIntelError extends Error {
  constructor(readonly code: CodeIntelErrorCode, message: string, readonly data: Record<string, unknown> = {}) 
}
export type CodeIntelErrorPayload = { code: number; message: string; data: { code: CodeIntelErrorCode } & Record<string, unknown> }
export function toErrorPayload(err: unknown): CodeIntelErrorPayload   // lỗi lạ → -32000 'CODEINTEL_TOOL_FAILED' không stack
```

| `data.code` | `error.code` (dùng lại `AgentErrorCode`, không thêm số mới) |
|---|---|
| `INVALID_PARAMS`, `SYMBOL_NOT_FOUND`, `PROFILE_UNKNOWN`, `RUN_NOT_FOUND` | `-32602` (`InvalidParams`) |
| `PATH_NOT_ALLOWED` | `-33002` (`PermissionDenied`) |
| các mã còn lại | `-32000` (`ServerError`) |

`message` ≤ 300 ký tự, không stack, không đường dẫn ngoài `workspaceRoot`; `stderrTail` ≤ 2 KiB đã qua `codeintel-secret-redaction.ts` (contract §3.2 `TOOL_FAILED`, §9 mục 4).

### 2.3 Tham số chặt (hợp đồng §2.1)

`validateCodeIntelParams(params, schema)`: `schema` là bảng khai báo từng tham số (`kind`, biên, mặc định). Quy tắc áp dụng cho mọi method:

- `workspaceRoot` bắt buộc; `_trace` là ngoại lệ duy nhất; khoá lạ → `CODEINTEL_INVALID_PARAMS data.field`.
- Danh sách khoá **cấm** kiểm tra riêng ở test phản chiếu schema: `args, argv, command, cmd, cwd, env, repo, cypher, shell, timeout, tool`.
- Chuỗi từ client: 1..512 ký tự, không NUL/ký tự điều khiển, chuẩn hoá NFKC rồi từ chối nếu bắt đầu bằng `-`, `－` (U+FF0D) hoặc `−` (U+2212).
- Đường dẫn tương đối gốc repo: không tuyệt đối, không `..`, không `\`.
- Ref git (`base`, `head`, `baseRef`): 1..256, `[A-Za-z0-9._/@^~{}+-]`, không bắt đầu `-`.

### 2.4 Whitelist lệnh và chạy tiến trình

Lệnh là đối tượng có kiểu; `codeintel-command-whitelist.ts` là nơi duy nhất sinh argv (không hàm nào nhận `string[]` từ ngoài):

```ts
export type GitNexusCommand =
  | { verb: 'cypher'; query: string }   // chỉ chuỗi đã qua assertReadOnlyCypher (AG-CV-SOL-002)
  | { verb: 'context'; uid: string; limit?: number }
  | { verb: 'context-by-name'; name: string; file?: string; limit?: number }
  | { verb: 'impact'; uid: string; direction: 'upstream'|'downstream'; depth: number; limit: number; includeTests?: boolean }
  | { verb: 'impact-by-name'; name: string; file?: string; kind?: string; direction: 'upstream'|'downstream'; depth: number; limit: number; includeTests?: boolean }
  | { verb: 'detect-changes'; scope: 'unstaged'|'staged'|'all'|'compare'; baseRef?: string }   // chỉ crossCheck (AG-CV-SOL-005)
export type CodeGraphCommand =
  | { verb: 'status' } | { verb: 'query'; search: string; limit: number; kind?: string }
  | { verb: 'callers'|'callees'; symbol: string; limit: number } | { verb: 'impact'; symbol: string; depth: number }
  | { verb: 'files'; filter?: string; maxDepth?: number } | { verb: 'affected'; files: string[]; depth?: number }
  | { verb: 'node'; name?: string; file?: string; offset?: number; limit?: number }
export function buildGitNexusArgv(cmd: GitNexusCommand, registryPath: string): string[]   // -r <registryPath> luôn CUỐI
export function buildCodeGraphArgv(cmd: CodeGraphCommand, projectPath: string): string[]    // -p <projectPath>; status dùng đối số vị trí
```

- `-r`/`-p` đặt **cuối** và giá trị người dùng đi trước nó để `--repo=x`, `-rx` không lọt (TestRepoFlagIsLast: tái hiện `gitnexus impact -r orca handleInvoke "--repo=vnp-workplace"` của contract §9).
- Danh sách cấm (contract §2.4): `analyze, clean, remove, uninstall, publish, setup, index, init, uninit, sync, serve, mcp, wiki, group, daemon, unlock, install, upgrade, telemetry, eval-server, check`. `check --cycles` chỉ mở ở AG-CV-SOL-037. `analyze|sync|index` chỉ ở `codeintel-reindex-commands.ts` (AG-CV-SOL-004).
- `runCodeIntelTool(cmd, binding, opts)` → `{stdout, stderr, exitCode, durationMs, stdoutBytes}`; qua cổng đồng thời, `AbortSignal` hạn method, `spawn(file, argv, {shell:false})`; GitNexus ghi stdout vào tệp tạm `<os.tmpdir()>/orca-codeintel-<uid>/` (`mkdtemp` `0700`, tệp `0600` cờ `wx`, dọn trong `finally`, dọn thư mục `orca-codeintel-*` quá 1 giờ khi khởi động), theo dõi kích thước mỗi 500 ms, vượt 16 MiB → kill + `OUTPUT_TOO_LARGE`; CodeGraph dùng pipe. Quy đổi: `{"error":…}` exit 0 → `TOOL_FAILED` (phân loại `not found|does not exist` → `SYMBOL_NOT_FOUND` do handler quyết, `Write operations` → `write_blocked`), JSON không parse được → `truncated_stdout`, text không bắt đầu `[`/`{` của CodeGraph → `unknown_shape` hoặc `INDEX_MISSING`/`SYMBOL_NOT_FOUND` theo nội dung (CR-003).
- Số đo `perf.cli[].rssPeakKb` của contract §2.2: không có cách đo di động trong Node cho tiến trình con (`getrusage` của con không có); đặt **tuỳ chọn, vắng khi không biết**; không bịa. Chưa kiểm chứng (mục 7).

Mở rộng `runToolCommand` (tương thích ngược; mặc định giữ nguyên để `tools/call` không đổi):

```ts
opts: { cwd: string; timeout: number; env: NodeJS.ProcessEnv;
        maxOutputBytes?: number; stdoutFile?: string; killGraceMs?: number;
        signal?: AbortSignal; detached?: boolean; stdinText?: string }
// ToolResult.meta: { truncated, timedOut, durationMs }
```
(`stdinText` phục vụ `codegraph affected --stdin` của AG-CV-SOL-003; `detached` + `process.kill(-pid)` phục vụ huỷ nhóm tiến trình của AG-CV-SOL-004; thêm cả hai ở đây để chỉ sửa hàm một lần.)

### 2.5 Phân giải repo (hợp đồng §9 mục 2)

Thứ tự trong `resolveCodeIntelRepo(workspaceRoot, config, deps)`: hình dạng → `realpath.native` → `ORCA_CODEINTEL_ALLOWED_ROOTS` → `git rev-parse [--path-format=absolute] --show-toplevel --git-common-dir` (qua `GitCapabilityCache` capability `rev-parse-path-format`; bản dự phòng bỏ cờ và `path.resolve` `git-common-dir` theo cwd) → `toplevel` phải trùng `workspaceRoot` (không → `PATH_NOT_ALLOWED data.hint="workspaceRoot must be a git work tree root"`) → `git worktree list --porcelain` (mục `worktree <path>` đầu = checkout chính; không `-z`) → khớp registry theo `realpath(entry.path) === toplevel` (không khớp tiền tố/tên; trùng → `indexedAt` mới nhất + cảnh báo) → worktree liên kết thì thử checkout chính, khớp ở đó ⇒ `worktreeMismatch:true` → CodeGraph: thư mục chứa `.codegraph/codegraph.db` (toplevel rồi checkout chính) → không có cả hai ⇒ `REPO_NOT_REGISTERED data.hint="run codeintel.reindex"`. Cache 30 s theo `workspaceRoot`; `invalidateRepoBindings()` cho SOL-004. Registry hỏng ⇒ `TOOL_FAILED reason="registry_unreadable"`.

```ts
export type CodeIntelRepoBinding = {
  workspaceRoot: string; toplevel: string; mainCheckoutRoot: string
  linkedWorktree: boolean; worktreeMismatch: boolean
  gitnexus: { name: string; path: string; storagePath: string; entry: GitNexusRegistryEntry } | null
  codegraph: { projectPath: string; hasDatabase: boolean } | null
  warnings: string[]
}
```

Git compat (AGENTS.md "Git Binary Compatibility"): chỉ lệnh baseline 2.25 (`rev-parse --show-toplevel --git-common-dir`, `worktree list --porcelain` Git 2.7); `--path-format` (2.31) có dự phòng; cache theo máy chạy (agent là một host nên một instance; test dùng instance riêng để chứng minh cô lập).

### 2.6 Môi trường tiến trình con

`buildCodeIntelChildEnv(config)`: bắt đầu từ `config.toolEnv`, **loại** mọi biến khớp `/(TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|API_?KEY|PRIVATE|DSN|AUTH|COOKIE|SESSION)/i` (và `SSH_AUTH_SOCK`, `AWS_*`, `GOOGLE_*`), cộng `NO_COLOR=1`; giữ `PATH`, `HOME`, `LANG`, `TMPDIR`, `NODE_*`. Lý do: `gitnexus`/`codegraph` không cần khoá; `toolEnv` thừa kế toàn `process.env`. Phương án an toàn lùi lại: nếu chủ hợp đồng không chấp nhận lọc, đổi một hàm này về `{...toolEnv, NO_COLOR:'1'}`. `quality.*` dùng env bắt đầu từ rỗng ở AG-CV-SOL-081.

### 2.7 Phát hiện công cụ, capability, status, bảng method, dispatcher

- `detectCodeIntelTools(config)`: tìm binary trong `config.toolPath.split(path.delimiter)`, chạy `<binary> --version` (timeout 5 s, cache 60 s); dải `gitnexus >=1.6.0 <2`, `codegraph >=1.4.0 <2` (giả định); ngoài dải ⇒ `supported:false`. Windows ⇒ mọi method `TOOL_UNAVAILABLE reason="unsupported_platform"`.
- `buildCapabilities`: thêm `codeintel`, `codeintel.gitnexus`, `codeintel.codegraph` chỉ theo **sự tồn tại** binary (không chạy `--version` vì cuộc đua 5 s ở `agent-session-handshake.ts:44-54`). `STATIC_CAPABILITIES_FALLBACK` không đổi (contract §1.3).
- `codeintel.status`: tham số `baseRef?` (nhận và kiểm hợp lệ để AG-CV-SOL-080 dùng); phần nền: `binding`, `tools`, `indexes.<tool>` qua probe đăng ký (`registerIndexProbe`, mặc định `{state:'unknown'}`), `sqliteReadAvailable:false` tới khi AG-CV-SOL-003, `host{platform,cores,loadavg1,freeMemBytes}` (`os`), `limits`. **Luôn thành công** khi binding phân giải được; chưa có binding vẫn trả `binding.gitnexus:null`, `codegraph:null`.
- `CODEINTEL_METHODS[method] = { validate, handle, timeoutMs }`; `handle` nạp động handler (`import()` trong thân, lỗi nạp ⇒ `TOOL_FAILED`, không phá phiên). Hạn 25 s (`status`, `reindex*`, `watch`, `overview`…), 55 s `detectChanges`/`structuralFacts` (đăng ký bởi SOL-005/037).
- `dispatchCodeIntelRpc(rpc, config, log, ws, state)`: `null` nếu không bắt đầu `codeintel.`; method lạ trong nhóm ⇒ `-32601` (contract §3.2 cuối); gọi `validate` → `handle` với `ctx{config, log, signal, deadline, notifier}`; `makeError(id, payload.code, payload.message, payload.data)`.
- `extractTraceFields`: nhánh `method.startsWith('codeintel.')` chỉ trả `{workspaceRoot: truncPath(...)}`; không tên symbol/uid/đường dẫn khác.

Ví dụ lỗi chuẩn (contract §7.4): `{"method":"codeintel.symbol","params":{"workspaceRoot":"../../etc",...}}` → `-33002` `CODEINTEL_PATH_NOT_ALLOWED`, từ chối trước `realpath`, không spawn.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do | Bỏ |
|---|---|---|---|
| 1 | Lõi trung lập truyền tải (không import `ws`, `WireState`, `makeError`, `makeNotifier`) | CR-006 sao lõi sang `desktop/src/relay/` | `toJsonRpcError` ở lõi |
| 2 | `codeintel-git-exec.ts` riêng | `git()` của `agent-git-handler-extended.ts` có `process.env` đầy đủ và không có `signal`; cây `desktop/` không có file đó | tái dùng helper |
| 3 | Khai đủ 16 mã lỗi một lần | tránh hai nơi sửa bảng (081 dùng chung) | mỗi CR thêm mã |
| 4 | Capability chỉ theo tồn tại binary | handshake đua 5 s | `--version` ở handshake |
| 5 | Tuỳ chọn `stdinText`, `detached` thêm vào `runToolCommand` ngay | một lần sửa hàm dùng chung với `tools/call` | sửa lại ở CR-003/004 |
| 6 | Chặn động từ ghi ở tool cũ (Q3 của CR) làm có điều kiện | tài liệu hoá không phá client; chưa kiểm chứng ai gọi `tools/call gitnexus` | để nguyên |
| 7 | Lọc bí mật trong env con | ngăn lộ khoá cho tiến trình bên thứ ba | thừa kế toàn bộ |

## 4. Phụ thuộc và thứ tự (task)

```
01 ─┐
02 ─┼─► 05 ─────────────┐
03 ─┤                    │
04 ─┘                    ├─► 09
01 ─► 06 ─► 08 ──────────┤
01 ─► 07 ────────────────┘
```
Rút gọn: 01, 02, 03, 04 song song, làm đầu tiên; 05 cần 01–04; 06 và 07 cần 01; 08 cần 06; 09 (tích hợp) cần 05, 07, 08. 04 sửa file dùng chung với `tools/call` nên làm sớm để bắt hồi quy.

## 5. Tiêu chí chấp nhận

(Dạng đo được; trích CR-001 mục 4 và hợp đồng.)

- [ ] `codeintel.status` trả phong bì đúng §2.2 + `binding`, `tools`, `limits`; không có `args`/lệnh CLI trong vào/ra.
- [ ] `codeintel.nope` ⇒ `-32601`; khoá lạ ⇒ `CODEINTEL_INVALID_PARAMS data.field`; test phản chiếu schema không có khoá cấm.
- [ ] `workspaceRoot` tương đối/NUL/không tồn tại/thư mục con repo/ngoài allowed roots ⇒ `PATH_NOT_ALLOWED` (hoặc `INVALID_PARAMS` cho lỗi hình dạng), **không spawn** (đếm spawn = 0).
- [ ] Registry giả 2 repo: repo kia không bao giờ bị chạm; worktree liên kết ⇒ `linkedWorktree:true`, `worktreeMismatch:true`, `stale:true`.
- [ ] Mọi lần chạy GitNexus có `-r <registryPath>` ở cuối; CodeGraph có `-p` (trừ `status`); `--repo=x`/`-rx` bị từ chối trước spawn.
- [ ] Test quét nguồn: ngoài `codeintel-reindex-*.ts` không có chuỗi `'analyze'|'clean'|'remove'|'sync'|'index'` sinh argv; `TestSpawnNeverUsesShell`.
- [ ] GitNexus > 256 KB (3 000 hàng) không cụt; tệp tạm xoá ở mọi nhánh; thư mục `0700`, tệp `0600`.
- [ ] stdout > 16 MiB ⇒ kill + `OUTPUT_TOO_LARGE`; quá 20 s ⇒ `SIGTERM` rồi `SIGKILL` sau 5 s ⇒ `TIMEOUT`; 4 lệnh đồng thời ⇒ ≤ 3 tiến trình, còn lại chờ hoặc `TIMEOUT reason="queue_wait"`.
- [ ] `{"error":"…"}` exit 0 ⇒ `TOOL_FAILED`; handshake có `codeintel*` khi binary có, không có khi vắng; `STATIC_CAPABILITIES_FALLBACK` không đổi.
- [ ] `tools/call name=gitnexus arguments={args:["analyze"]}` bị từ chối (nếu quyết định 6 chốt); tool khác không đổi hành vi (`agent-tool-registry.test.ts` xanh không sửa).
- [ ] Trace của `codeintel.*` chỉ có `workspaceRoot`; con của codeintel không thấy `ANTHROPIC_API_KEY`/`GITHUB_TOKEN`/`GH_TOKEN`.
- [ ] Không file `helpers/utils/common/misc`, không `max-lines` disable mới.

## 6. Kiểm thử

Chạy trong `/opt/repos/orca/agent` (chưa chạy gì):

| File test (mới) | Lệnh |
|---|---|
| `src/relay/codeintel-errors.test.ts`, `codeintel-params-validation.test.ts` | `pnpm exec vitest run src/relay/codeintel-errors.test.ts src/relay/codeintel-params-validation.test.ts` |
| `src/relay/codeintel-limits.test.ts`, `codeintel-concurrency-gate.test.ts` | `pnpm exec vitest run src/relay/codeintel-limits.test.ts src/relay/codeintel-concurrency-gate.test.ts` |
| `src/relay/codeintel-command-whitelist.test.ts`, `codeintel-child-env.test.ts` | `pnpm exec vitest run src/relay/codeintel-command-whitelist.test.ts src/relay/codeintel-child-env.test.ts` |
| `src/relay/codeintel-tool-runner.test.ts`, `codeintel-tool-output-classification.test.ts`, `codeintel-secret-redaction.test.ts` | `pnpm exec vitest run src/relay/codeintel-tool-runner.test.ts src/relay/codeintel-tool-output-classification.test.ts src/relay/codeintel-secret-redaction.test.ts` |
| `src/relay/__tests__/agent-tool-registry.test.ts` (sửa) | `pnpm exec vitest run src/relay/__tests__/agent-tool-registry.test.ts` |
| `src/relay/gitnexus-registry-reader.test.ts`, `codeintel-repo-resolution.test.ts` | `pnpm exec vitest run src/relay/gitnexus-registry-reader.test.ts src/relay/codeintel-repo-resolution.test.ts` |
| `src/relay/codeintel-tool-detection.test.ts`, `agent-session-capabilities-codeintel.test.ts` | `pnpm exec vitest run src/relay/codeintel-tool-detection.test.ts src/relay/agent-session-capabilities-codeintel.test.ts` |
| `src/relay/codeintel-result-envelope.test.ts`, `codeintel-status.test.ts`, `agent-rpc-dispatch-codeintel.test.ts` | `pnpm exec vitest run src/relay/codeintel-result-envelope.test.ts src/relay/codeintel-status.test.ts src/relay/agent-rpc-dispatch-codeintel.test.ts` |
| Toàn gói | `pnpm test` |

Binary giả: script Node trong thư mục tạm đặt vào `PATH` (`config.toolPath` trong test). Kiểm kiểu: `npx tsc --noEmit` chỉ so sánh trước/sau (v4 ghi 53 lỗi có sẵn ngày 2026-09-09, chưa chạy lại).

## 7. Rủi ro và điểm chưa kiểm chứng

- Mọi hành vi thật của `gitnexus 1.6.9`/`codegraph 1.4.1` (cụt pipe, `--version`, mã thoát khi thiếu `-r`): lấy từ CR, chưa chạy lại; task 05 có bước ghi fixture.
- `rssPeakKb` trong `perf.cli` không có cách đo di động ⇒ vắng.
- Lọc env có thể làm công cụ cần một biến khớp mẫu (ví dụ `*_AUTH*`) hỏng; chưa biết biến nào GitNexus/CodeGraph cần.
- Sửa `runToolCommand` (dùng bởi 7 tool trong `ALL_TOOL_DEFINITIONS`): cần `gitnexus impact`/`context` (`-r /opt/repos/orca`) và test hiện có trước khi merge.
- Windows/WSL/UNC: chưa kiểm chứng (từ chối `unsupported_platform`); macOS `realpath.native` chưa chạy.
- Giới hạn 3 tiến trình chưa đo RAM trên dev server nhỏ (`lbug` ~1,6 GB trên đĩa).
- Telemetry của CodeGraph: chưa kiểm tra `codegraph telemetry status` trên dev server (task 08 ghi bước kiểm).
- CI không chạy test `agent/`: các lệnh vitest ở trên chỉ chạy cục bộ tới khi AG-CV-SOL-070 thêm job.

## 8. Câu hỏi mở

1. `ORCA_CODEINTEL_ALLOWED_ROOTS` mặc định rỗng có đủ (contract §2.1 chốt rỗng) hay đọc từ `AGENT_WORK_DIR`?
2. Có cho `reindex` đăng ký repo mới (CR Q2)? Hợp đồng §4.10 không cho; SOL-004 theo hợp đồng.
3. Chủ hợp đồng xác nhận lọc env của con codeintel (mục 2.6) hay giữ `toolEnv` nguyên văn (contract §2.4)?
4. Hợp đồng §9 mục 1 cho phép `query, trace, list, status` (GitNexus) và `explore` (CodeGraph) trong tập đóng nhưng không method nào dùng: xác nhận không cài.
5. `ORCA_CODEINTEL_DISABLED=1` trả mã nào? Hợp đồng chỉ nêu biến (§2.4) và không có `reason` tương ứng trong bảng `TOOL_UNAVAILABLE`; AG-CV-SOL-073 quyết. Điểm cắm: `dispatchCodeIntelRpc` kiểm cờ đầu tiên.
6. Windows: `status` có nên vẫn trả được `tools.*.available:false` thay vì lỗi (xung đột §1.1 và §4.1)?

## 9. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-023-infra-fleet-codeintel-transport` | timeout Go 30/90 s, `AgentRPCError` + trailer, `Tools []string`, `GetAgentCapabilities`; agent chỉ cần `-32601` đúng và `error.data.code` |
| BE | `BE-CV-SOL-021-agent-collector` | gọi `codeintel.status` sau mỗi lần nối lại; đọc `capabilities`; chờ G1 (tệp vàng) |
| BE | `BE-CV-SOL-012-target-resolution-and-bindings` | ghép `workspaceRoot` ↔ binding bằng `(tenant, dev_server_id, path_hash)` |
| AG | `AG-CV-SOL-070-golden-fixtures-and-parsers` | fixture vàng, job CI `code-intel-contract` |
| AG | `AG-CV-SOL-072-security-tests-agent`, `AG-CV-SOL-073-agent-kill-switch`, `AG-CV-SOL-071-perf-block-and-bench` | test bảo mật/kill-switch/`perf` dựa vào lõi này |
| AG | `AG-CV-SOL-081-quality-runner-core` | dùng `codeintel-errors.ts`, `codeintel-limits.ts`, hook `route()`; nhóm `quality.*` thêm `dispatchQualityRpc` cạnh khối codeintel |
| FE | — | không có việc ở FE |
Thứ tự theo contract §7.2: `001 → 002 → 005`, `001 → 003`, `002 → 004`, `006` sau 001–005; đợt 1.

## 10. Tham chiếu

- Contract: `specs/backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md` §0–§3, §4.1, §9; `CONTRACT-codeintel-proto-and-data-map.md` PQ-02/03/13/18/19/20/21/23, §7.2, §8.2, §8.3, §10.
- CR: `docs/crs/v7/agent-codeintel/CR-CV-001-codeintel-agent-foundation.md`, `README.md` (F1–F10); `docs/crs/v7/README.md` mục 2 (D1, D5), 3.2, 8.
- Code: các file ở mục 1; `guides/reference/git-compatibility.md` (baseline 2.25).
