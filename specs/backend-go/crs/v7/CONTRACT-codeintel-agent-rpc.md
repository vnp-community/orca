# CONTRACT: Hợp đồng JSON-RPC giữa backend (code-intel-service qua infra-fleet-service) và agent cho `codeintel.*` và `quality.*` (v7)

> **Nguồn sự thật cho mọi thứ `agent/` (TypeScript) nhận và gửi trong series v7 "Xem code và kiểm soát chất lượng".** Các `AG-CV-SOL-*` hiện thực đúng file này; các `BE-CV-SOL-*` (`infra-fleet-service`, `code-intel-service`) chỉ gọi/nhận những gì có ở đây. Cần đổi thì sửa file này trước.
> Bộ ba: file này, [`CONTRACT-codeintel-ui-api.md`](./CONTRACT-codeintel-ui-api.md), [`CONTRACT-codeintel-proto-and-data-map.md`](./CONTRACT-codeintel-proto-and-data-map.md) (**mục 1 "Phán quyết" ở file proto là nơi tập trung các mâu thuẫn đã xử lý; file này trích `PQ-xx`**).
> CR gốc: `docs/crs/v7/agent-codeintel/` (001–006), `quality-signals/` (080–084, 091), `code-intel-graph-pipeline/` (021, 023), `code-intel-sources/` (037), `quality-rollout/` (070–072). README v7 mục 8 thắng mục 3; phán quyết thắng cả CR khi hai CR mâu thuẫn.
> **Trạng thái: 📋 Proposed.** Không dòng nào đã chạy trên hệ thống; mọi mẫu Cypher, hình dạng đầu ra GitNexus 1.6.9 / CodeGraph 1.4.1 / oxlint / tsc / vitest / go vet / golangci-lint / buf / opa lấy từ CR và **chưa được chạy lại**. Phần ghi "(mới)" là đề xuất của hợp đồng này.

## 0. Bằng chứng đã đọc trong code thật (2026-10-06)

| Khẳng định | Nơi đọc |
|---|---|
| Agent nói JSON-RPC 2.0 hai chiều trên cùng WS; khung 13 byte `[TYPE u8][SEQ u32][ACK u32][LENGTH u32][PAYLOAD]`; `0x01` = Regular, `0x09` = KeepAlive (5 s), timeout 20 s | `agent/src/shared/agent-wire-protocol.ts` |
| Mã lỗi số có sẵn: `-32700/-32600/-32601/-32602/-32000`, `-33001…-33007`, `-33100`, `-33101`; **không có mã số riêng cho `CODEINTEL_*`** (CR-001 dùng lại các mã này) | cùng file `AgentErrorCode` |
| `AgentCapability = 'pty' \| 'fs' \| 'git' \| 'preflight'` là union cố định, nhưng `buildCapabilities` trả mảng **chuỗi tự do** (`'fs'`, `'fs.watch'`, `'git.exec'`, `'agent.exec'`…) | `agent-wire-protocol.ts`; `agent/src/relay/agent-session-capabilities.ts` |
| `agent.handshake` params: `agentVersion, platform, arch, nodeVersion, capabilities[], agentToken?, devServerId, tools: string[]` (`tools.map(t => t.name)` — **chỉ tên**) | `agent/src/relay/agent-session-handshake.ts` |
| Go `inboundHandshakeParams` chỉ đọc `AgentToken, DevServerID, Platform, Arch, NodeVersion, AgentVersion, Capabilities` — **không đọc `tools`** | `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go` |
| `route()` thử lần lượt `dispatchGitRpc … dispatchHiddenTargetRpc`, hết thì `MethodNotFound`; `makeError(id, code, message, data?)` **giữ `data`**; `makeNotifier(ws, state)` gửi `{jsonrpc:'2.0', method, params}` không `id` | `agent/src/relay/agent-rpc-dispatch.ts` |
| Tool `gitnexus`/`codegraph` hiện có: `args` tự do, `timeout 60_000`, `env: config.toolEnv`; chỉ Part A (`agent-rpc-dispatch-misc.ts`) | `agent/src/relay/agent-tool-registry.ts` |
| Go `JSONRPCError{Code int, Message string, Data json.RawMessage}`; `Client.Exec` trả `map[string]any` (**kết quả agent phải là object JSON**); `-32601` → `domain.ErrAgentMethodNotFound` | `…/adapter/devserveragent/jsonrpc.go`, `client.go` |
| `execTimeoutForMethod` chỉ ngoại lệ `agent.execPrompt`; mặc định `RequestTimeout = 30 s` | `…/devserveragent/client.go:412`, `config.go:57` |
| `RelayByDevServer` bọc **mọi** lỗi `Exec` thành `apperrors.New(KindInternal, "INFRA_AGENT_EXEC_FAILED", …, err)`; cause không ra client; offline → `INFRA_DEV_SERVER_NOT_CONNECTED` ngay (không xếp hàng) | `…/usecase/relay_by_dev_server.go` |
| `apperrors.ToGRPCStatus` gửi `Code + ": " + Message`, nguyên nhân gốc chỉ log; chỉ 8 `Kind` | `backend-go/common/apperrors/apperrors.go` |
| Khung tối đa 16 MiB (`MaxMessageSize`) | `…/devserveragent/frame.go:24` |

Chưa kiểm chứng (ghi trong CR): hành vi thật của `gitnexus 1.6.9` / `codegraph 1.4.1`, mọi con số hiệu năng, Windows/WSL, Part B thật ở `desktop/src/relay/relay.ts`.

---

## 1. Truyền tải, handshake, năng lực

### 1.1 Chế độ và phạm vi
- Chỉ `direct-websocket` (agent dial `wss://<gateway>/agent`) là phạm vi MVP (D2). `relay-ssh` do Go khởi tạo chạy **cùng mã Part A** qua `node agent.js --stdio|--detach|--connect` nên các method dưới đây có sẵn; việc kiểm thử là của CR-006. `relay-websocket` ngoài phạm vi.
- **Part B** (`RelayDispatcher`, `desktop/src/relay/relay.ts`): **ngoài phạm vi v7 MVP**; để backlog rõ ràng với label `post-mvp`. Lý do: cần `AgentRelay` chưa có chủ port (C5). Khi Part B được phê duyệt (CR-006), đăng ký cùng bảng method `CODEINTEL_METHODS`; khác biệt ở mục 8. `quality.*` **không có** ở Part B ở v7.
- Windows: `codeintel.status` trả `{compatibility: {status: "incompatible", reason: "unsupported_platform"}, ...}` — không crash, không "luôn thành công". Các method đọc khác trả `CODEINTEL_TOOL_UNAVAILABLE reason:"unsupported_platform"`.

### 1.2 Phiên bản giao thức
Giao thức giữ nguyên `AGENT_PROTOCOL_VERSION = '1'`. Việc thêm method/capability là **additive**; backend phát hiện bằng `capabilities`, không bằng số phiên bản.

### 1.3 Handshake và năng lực (`capabilities[]`, chuỗi)

| Capability | Thêm khi | Nguồn |
|---|---|---|
| `codeintel` | ≥ 1 trong hai binary `gitnexus`, `codegraph` có trong `toolPath` (chỉ kiểm tồn tại, **không** chạy `--version` trong handshake: đua timeout 5 s) | CR-001 §2.7 |
| `codeintel.gitnexus`, `codeintel.codegraph` | từng binary có | CR-001 |
| `quality` | dispatcher `quality.*` đã đăng ký **và** ≥ 1 profile `ready` | CR-081 §2.1 |

- `STATIC_CAPABILITIES_FALLBACK` **không** thêm `codeintel*`/`quality` (fallback chỉ khi thăm dò quá 5 s).
- `tools[]` giữ nguyên là mảng tên (`gitnexus`, `codegraph` khi binary có). Backend **không** dựa vào `tools[]` cho logic mới (Go hiện không lưu); CR-023 thêm `Tools []string` vào `HandshakeInfo` và RPC `GetAgentCapabilities` (PQ-18).
- Dải phiên bản công cụ hỗ trợ (giả định, chưa kiểm): GitNexus `>=1.6.0 <2` (đã thử 1.6.9), CodeGraph `>=1.4.0 <2` (đã thử 1.4.1); marker lược đồ: GitNexus `meta.json.schemaVersion ∈ {5}`, CodeGraph `extractionVersion ∈ {24}`, `schema_versions ∈ [1,8]` (CR-070). Ngoài dải: `supported:false`, method đọc trả `CODEINTEL_TOOL_UNAVAILABLE reason="unsupported_version"`.
- `-32601` cho method `codeintel.*`/`quality.*` ⇒ agent không hỗ trợ (infra-fleet đổi thành `CODEINTEL_AGENT_UNSUPPORTED`).

---

## 2. Quy ước chung của mọi method

### 2.1 Tham số
- **`workspaceRoot` (string) bắt buộc ở mọi method** `codeintel.*` và `quality.*` (PQ-21). Kiểm: tuyệt đối theo nền tảng chạy (`path.isAbsolute`), không NUL, ≤ 4096 ký tự; `fs.realpath`; phải là **gốc của git worktree** (`git rev-parse --show-toplevel` realpath trùng, nếu không → `CODEINTEL_PATH_NOT_ALLOWED` `data.hint="workspaceRoot must be a git work tree root"`); tuỳ chọn env `ORCA_CODEINTEL_ALLOWED_ROOTS` (ngăn bởi `path.delimiter`, mặc định rỗng = không giới hạn).
- **Tham số lạ bị từ chối** `CODEINTEL_INVALID_PARAMS` (`data.field`), không bỏ qua im lặng. Ngoại lệ duy nhất: `_trace` (`params._trace.id`, như `extractResume`).
- **Không bao giờ** nhận `args`, `argv`, `command`, `cmd`, `cwd`, `env`, `repo`, tên repo GitNexus, `cypher`, `shell`, `timeout`, `tool`. Test phản chiếu schema `validate` của mọi method khẳng định điều này (CR-072 §2.2).
- Chuỗi từ client (tên symbol, uid, file, từ khoá): 1..512 ký tự, không NUL/ký tự điều khiển, **không bắt đầu bằng `-`** (kể cả U+FF0D, U+2212 sau chuẩn hoá NFKC).
- Đường dẫn trong tham số (`center.file`, `filter`…) là **tương đối gốc repo**, không `..`, không tuyệt đối, không `\`.

### 2.2 Phong bì kết quả (mọi method đọc)
Kết quả **luôn là object JSON** (Go giải mã vào `map[string]any`):

```jsonc
{
  "sources": [ { "tool": "gitnexus", "version": "1.6.9", "indexedAt": "2026-10-05T05:55:17.289Z",
                 "commit": "d8198127b6bd14a3bf02dda1762ee13b7f3848ed", "lineBase": 1 },
               { "tool": "codegraph", "version": "1.4.1", "indexedAt": "…", "commit": null, "lineBase": 1 } ],
  "headCommit": "1b0c760935…",      // git rev-parse HEAD tại workspaceRoot; null nếu không đọc được
  "stale": true,                      // OR các nguồn đã dùng: indexedCommit !== headCommit || rootMismatch/worktreeMismatch || pendingChanges > 0
  "truncated": false,                 // dữ liệu bị cắt theo giới hạn của method
  "totalCount": 9039,                 // tổng trước khi cắt; null nếu không biết
  "warnings": ["index_commit_differs_from_head"],   // tuỳ chọn; chuỗi mã ổn định
  "perf": { "totalMs": 1840, "queueWaitMs": 12, "cliCalls": 1,
            "cli": [ { "tool": "gitnexus", "command": "impact", "ms": 1790, "stdoutBytes": 1019, "rssPeakKb": 830232 } ],
            "parseMs": 6, "truncated": false },   // chỉ cho backend ghi metric/span; KHÔNG ra UI, KHÔNG vào cache snapshot
  "data": { }                         // theo từng method (mục 4)
}
```
- `perf` **thay** `toolTimingsMs` của CR-001 (cùng ý; PQ-19); `command` chỉ là hằng whitelist (`cypher|context|impact|query|status|callers|callees|files|affected|detect-changes`).
- `sources[].lineBase` luôn `1` (agent đã +1 với GitNexus; PQ-20). `commit` của CodeGraph luôn `null` (không có commit trong `status`).
- `stale` **không phải lỗi**; `truncated` **không phải lỗi**.
- Phong bì áp cho: `status, overview, processes, process, subgraph, impact, symbol, routes, detectChanges, structuralFacts, codegraphSearch, files`. **Không áp** cho `reindex, reindexStatus, reindexCancel, watch` (kết quả là `{...}` trực tiếp, mục 4.10–4.13) và `quality.*` (kết quả là object theo mục 5).
- `SymbolRef` do agent trả **đã chuẩn hoá** (PQ-20): xem 2.6.

### 2.3 Giới hạn (override bằng env `ORCA_CODEINTEL_*`; giá trị sai bị bỏ + log cảnh báo)

| Giới hạn | Mặc định |
|---|---|
| Timeout một lần chạy CLI | 20 000 ms (SIGTERM, rồi SIGKILL sau `killGraceMs` 5000) |
| Timeout toàn method | 25 000 ms (agent tự trả `CODEINTEL_TIMEOUT` trước khi Go cắt) |
| Timeout `detectChanges`, `structuralFacts` | 55 000 ms (env `ORCA_CODEINTEL_DETECT_TIMEOUT_MS` hạ về 25 000 nếu infra-fleet chưa nâng) |
| Stdout tối đa một tiến trình | 16 MiB (vượt: kill + `CODEINTEL_OUTPUT_TOO_LARGE`) |
| JSON kết quả cuối | 8 MiB (cắt dần mảng theo ưu tiên của method + `truncated:true`; vẫn vượt → `CODEINTEL_OUTPUT_TOO_LARGE`) |
| stderr lưu | 64 KiB |
| Tiến trình công cụ đồng thời toàn agent | 3 (`gitnexus` ≤ 2/dev server, `codegraph` ≤ 3 theo CR-071) |
| Hàng đợi slot | ≤ 16 mục, chờ ≤ 10 s, quá → `CODEINTEL_TIMEOUT` `data.reason="queue_wait"` |
| Cache repo resolve | 30 s theo `workspaceRoot` (huỷ khi `indexChanged`/reindex xong); phát hiện công cụ 60 s; `headCommit` 5 s |
| Cache kết quả ngắn hạn | 60 s, ≤ 64 mục, ≤ 32 MiB (LRU, singleflight); khoá `(registryPath, indexedAt|lastCommit, method, hash(paramsChuẩnHoá))`; không cache lỗi và `symbol` có `source` |

### 2.4 Môi trường tiến trình con và an toàn thực thi
- **GitNexus/CodeGraph** (CR-001 §2.3): chỉ `spawn(file, argv, {shell:false})`; mỗi lệnh là đối tượng có kiểu (`GitNexusCommand`, `CodeGraphCommand`), **không** có API argv tự do; GitNexus luôn `-r <registryPath tuyệt đối từ ~/.gitnexus/registry.json>`, CodeGraph luôn `-p <projectPath>`; stdout GitNexus đi qua **tệp tạm** `<os.tmpdir()>/orca-codeintel-<uid>/` (thư mục `0700`, tệp `0600`, cờ `wx`) vì pipe cắt cụt.

  **Env allowlist cho GitNexus/CodeGraph** — env con được xây từ rỗng (không `config.toolEnv`), chỉ bổ sung các biến trong allowlist cứng dưới đây. Lý do: `config.toolEnv` chứa toàn bộ `process.env` kể cả `ANTHROPIC_API_KEY`, `GITHUB_TOKEN` (README v7 §8 điểm 22); tiến trình con không cần LLM key hay SCM token.

  Allowlist GitNexus/CodeGraph: `PATH`, `HOME`, `USER`, `LOGNAME`, `TMPDIR`, `TEMP`, `TMP`, `LANG`, `LC_ALL`, `LC_CTYPE`, `TZ`, `SHELL`, `NO_COLOR=1` (ghi đè cứng). Cho phép thêm `GIT_*` **chỉ** các biến an toàn: `GIT_EXEC_PATH`, `GIT_TEMPLATE_DIR`, `GIT_CONFIG_NOSYSTEM` — **không** `GIT_SSH*`, `GIT_ASKPASS`, `GIT_CREDENTIAL*`. **Loại** mọi biến khớp `/(TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|API_?KEY|PRIVATE|DSN|AUTH|COOKIE|SESSION|ORCA_|AWS_|GOOGLE_|AZURE_|GITHUB_|GH_|AGENT_|SSH_AUTH|ANTHROPIC)/i`. `ORCA_CODEINTEL_ALLOWED_ENV` (ngăn bởi `path.delimiter`) có thể mở rộng danh sách; giá trị lạ bị bỏ + log cảnh báo.

- **Cypher**: mẫu hằng, khe `{{ten}}` thay bằng 4 bộ mã hoá (`cypherInt`, `cypherString`, `cypherStringList`, `cypherKindList`); `assertReadOnlyCypher` (≤ 16 KiB, một câu, bắt đầu `MATCH `, cấm `CREATE|MERGE|DELETE|SET|REMOVE|DROP|ALTER|COPY|DETACH|CALL|LOAD|INSTALL|ATTACH|EXPORT|IMPORT|FOREACH|UNWIND`). Công cụ **không** có tham số ràng buộc nên "tham số hoá" thực chất là thay chuỗi đã mã hoá (CR-002 §1.1). Mẫu chưa chạy là điều kiện tiên quyết merge (CR-002 §2.1).
- **Quality** (CR-081 §2.8): env con **bắt đầu từ rỗng** (không `config.toolEnv`); cho phép `PATH` (`qualityToolPath` = `toolPath` + `~/go/bin` + `$(go env GOPATH)/bin` + `/usr/local/go/bin` + `~/.local/share/pnpm`), `HOME, USER, LOGNAME, LANG, LC_ALL, TZ, TMPDIR, SHELL`, cố định `CI=1, NO_COLOR=1, FORCE_COLOR=0, TERM=dumb`, Go (`GOFLAGS`, `GOTOOLCHAIN=local`, `GOMAXPROCS`, `GOMEMLIMIT`, và `GOCACHE/GOPATH/GOMODCACHE` chỉ khi đã đặt), Node (`NODE_OPTIONS` từ profile, `PNPM_HOME`, `npm_config_cache`). **Loại** mọi biến khớp `/(TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|API_?KEY|PRIVATE|DSN|AUTH|COOKIE|SESSION)/i` kể cả khi profile xin qua `env.allowExtra`; đặc biệt `ANTHROPIC_API_KEY, GITHUB_TOKEN, GH_TOKEN, AGENT_TOKEN, ORCA_*, SSH_AUTH_SOCK, AWS_*, GOOGLE_*`. Profile quality cần thêm biến phải khai báo qua `env.allowExtra` (agent kiểm từng tên, không đem cả `toolEnv`).
  
  **Chính sách mạng** cho quality runner: `ORCA_QUALITY_NETWORK=allow|block` (mặc định `block` ngoại trừ module từ registry đã cache); profile khai báo `network_policy: "allow"` thì runner kiểm `ORCA_QUALITY_NETWORK` server-side và bỏ qua yêu cầu nếu bị chặn (không lỗi — runner bỏ bước mạng, ghi warning). Giá trị `ORCA_QUALITY_NETWORK` không ra agent qua env (chỉ là policy agent đọc từ config file).
- Tên biến cấu hình agent: `ORCA_CODEINTEL_ALLOWED_ROOTS`, `ORCA_CODEINTEL_ALLOWED_ENV`, `ORCA_CODEINTEL_*` (override giới hạn mục 2.3), `ORCA_CODEINTEL_SQLITE=auto|off`, `ORCA_CODEINTEL_ANALYZE_WORKERS`, `ORCA_CODEINTEL_MAX_REINDEX` (1, tối đa 2), `ORCA_CODEINTEL_REINDEX_TIMEOUT_MS` (45 phút), `ORCA_CODEINTEL_REINDEX=off`, `ORCA_CODEINTEL_DISABLED=1` (tắt **cả** `codeintel.*` **và** `quality.*`, CR-073), `ORCA_CODEINTEL_DETECT_TIMEOUT_MS`, `ORCA_QUALITY_RUN=off`, `ORCA_QUALITY_QUEUE_MAX` (4), `ORCA_QUALITY_RESULT_TTL_MS` (1 h), `ORCA_QUALITY_CGROUP`, `ORCA_QUALITY_ISOLATION`, `ORCA_QUALITY_NETWORK=allow|block`, `ORCA_HEAVY_JOBS` (1), `ORCA_HEAVY_QUEUE_WAIT_MS` (10 phút). Phía backend dùng tiền tố `CODEINTEL_` (PQ-23).

  **Cờ tắt tập trung**: `ORCA_CODEINTEL_DISABLED=1` tắt **toàn bộ** `codeintel.*` và `quality.*` — mọi method trả `CODEINTEL_TOOL_UNAVAILABLE reason:"codeintel_disabled"`. Tên biến backend: `CODEINTEL_ENABLED` / `codeIntelEnabled` (theo hợp đồng PQ-23); không dùng `CODEINTEL_DISABLED` hay `CODE_INTEL_ENABLED`. `ExportReviewReport` (CR-090) thất bại với `CODEINTEL_DISABLED` khi cờ chất lượng tắt.

### 2.5 Timeout (agent so với Go) — bảng chuẩn (PQ-13)

| Method | Timeout agent | Timeout Go (`infra-fleet`) | Ghi chú |
|---|---|---|---|
| `codeintel.status`, `reindex`, `reindexStatus`, `reindexCancel`, `watch` | 25 s | **30 s** (mặc định) | `reindex` trả ngay `jobId`, chạy nền |
| `codeintel.overview`, `processes`, `process`, `subgraph`, `impact`, `symbol`, `routes`, `codegraphSearch`, `files` | 25 s | **90 s** | cold overview ≈ 5–7 s (đo một lần, CR-002) |
| `codeintel.detectChanges`, `structuralFacts` | **55 s** | **90 s** | ngoại lệ ghi trong CR-005 §2.7 |
| `quality.listProfiles` | 40 s (preflight ≤ 10 s) | **45 s** | |
| `quality.run`, `runStatus`, `cancel`, `results`, `coverage` | 10 s | **30 s** | `run` trả ngay `runId`; chạy nền |
| `ai.complete` (ngoài series nhưng ảnh hưởng CR-093) | 120 s (`ai-complete-handler.ts`) | **120 s** | cần ngoại lệ trong `execTimeoutForMethod` |

Quy tắc: agent **luôn hết hạn trước Go** (`CODEINTEL_TIMEOUT`); khi Go cắt, lệnh vẫn có thể chạy tiếp trên agent (`session.go` chỉ `dropPending`), nên agent phải tự huỷ cây tiến trình khi hết hạn nội bộ. Collector thêm 5 s dự phòng (95 s). Chờ nối lại dev server (20 s) là việc của collector, không phải agent (PQ-13).

### 2.6 `SymbolRef` do agent trả (PQ-20)

```jsonc
{ "key": "method:agent/src/relay/context.ts:RelayContext.registerRoot",   // "<kind>:<filePath>:<qualifiedName||name>", kind thường
  "kind": "function|method|type|value|file|folder|route|component|namespace|import|cluster|flow|doc",
  "nativeKind": "Method",                    // nhãn gốc (Struct, type_alias…); chuỗi mở
  "name": "registerRoot", "qualifiedName": "RelayContext.registerRoot",   // `::`→`.`, bỏ `#n`, NFC
  "filePath": "agent/src/relay/context.ts",  // tương đối gốc repo, dấu '/'
  "startLine": 31, "endLine": 33,            // 1-based
  "language": "typescript",
  "gitnexusId": "Method:agent/src/relay/context.ts:RelayContext.registerRoot#1",
  "codegraphId": "method:3f2a…",             // hash, KHÔNG ổn định giữa lần index, không dùng làm khoá bền
  "signature": null, "isExported": null, "docstring": null, "ordinal": null }
```
Quy tắc khoá: bỏ tiền tố `<Label>:` và `<filePath>:`, bỏ hậu tố `#<n>` (lưu `ordinal`), CodeGraph `::`→`.`; `Section` → `doc:<filePath>:L29:Features`; cluster/flow → `cluster::comm_6117`, `flow::proc_0_checkspanel`; file/folder → `file:<path>:<tên cuối>`. Va chạm trong cùng kết quả: thêm `#<arity>`, vẫn trùng thì `#L<startLine>` (+ `warnings:["key_collision"]`). Ánh xạ kind chuẩn:

| `kind` | GitNexus nhãn | CodeGraph kind |
|---|---|---|
| `function` | `Function` | `function` |
| `method` | `Method`, `Constructor` | `method` |
| `type` | `Struct, Class, Interface, Enum` | `struct, class, interface, enum, type_alias` |
| `value` | `Const, Variable, Property` | `constant, variable, property, field, enum_member` |
| `file` / `folder` | `File` / `Folder` | `file` / — |
| `route` | `Route` | `route` |
| `component` | — | `component` |
| `namespace` | — | `namespace` |
| `import` | — (GitNexus dùng cạnh `IMPORTS`) | `import` (67 865 nút trên Orca; **không** vào `SymbolGraph` mặc định) |
| `cluster` / `flow` / `doc` | `Community` / `Process` / `Section` | — |
Kind lạ → `value` + giữ `nativeKind` + `warnings:["unknown_native_kind"]`. Hai nguồn cùng `key` mà `startLine` lệch > 2 → `warnings:["sources_disagree"]` (backend quyết, CR-020).

---

## 3. Mã lỗi (một bảng duy nhất)

### 3.1 Vị trí
JSON-RPC response lỗi: `error: { code: <số>, message: <chuỗi ≤ 300, không stack, không đường dẫn ngoài workspaceRoot>, data: { code: "CODEINTEL_X", ...bổ sung } }`. `error.code` số **dùng lại** `AgentErrorCode` (không thêm số mới). Thông báo (notification) **không bao giờ** là lỗi.

### 3.2 Mã do **agent** sinh

| `data.code` | `error.code` | Khi nào | `data` bổ sung | Retry |
|---|---|---|---|---|
| `CODEINTEL_INVALID_PARAMS` | -32602 | thiếu/sai kiểu/vượt biên/tham số lạ/ref không phân giải | `field`, `reason` (`unresolved_ref`, `base_required`, `no_merge_base`, `invalid_scope`…) | không |
| `CODEINTEL_PATH_NOT_ALLOWED` | -33002 | `workspaceRoot` không hợp lệ/không phải gốc worktree/ngoài allowed roots; reindex worktree liên kết `trigger=manual` | `hint` (`reindex_linked_worktree_unsupported`, …) | không |
| `CODEINTEL_TOOL_UNAVAILABLE` | -32000 | không binary, phiên bản ngoài dải, nền tảng chưa hỗ trợ, tắt cứng | `tool`, `reason` (`unsupported_version`, `unsupported_platform`, `reindex_disabled`, `quality_disabled`, `schema_version_unsupported`) | không |
| `CODEINTEL_REPO_NOT_REGISTERED` | -32000 | không khớp registry GitNexus và không có DB CodeGraph | `hint` (`run codeintel.reindex`) | không |
| `CODEINTEL_INDEX_MISSING` | -32000 | repo có công cụ nhưng chỉ mục cần dùng chưa có / chưa khởi tạo | `tool`, `hint` | không |
| `CODEINTEL_SYMBOL_NOT_FOUND` | -32602 | symbol/process/flow không có trong chỉ mục (PQ-03) | `kind` (`symbol|process`), `id?` | không |
| `CODEINTEL_AMBIGUOUS_SYMBOL` | -32000 | GitNexus `status:"ambiguous"` | `candidates[]` ≤ 10: `{uid,name,kind,filePath,line}`; `impact` thêm `score,impactedCount,risk` | không |
| `CODEINTEL_TIMEOUT` | -32000 | quá 20 s/lần CLI hoặc timeout method, hoặc chờ slot | `tool?`, `reason?` (`queue_wait`), `elapsedMs` | có |
| `CODEINTEL_REINDEX_IN_PROGRESS` | -32000 | đang `analyze`/`sync` cho repo; method đọc công cụ đang bị ghi | `jobId`, `state`, `stage`, hoặc `reason:"queue_full"` | có |
| `CODEINTEL_OUTPUT_TOO_LARGE` | -32000 | stdout > 16 MiB hoặc kết quả > 8 MiB | `bytes`, `limit` | không |
| `CODEINTEL_TOOL_FAILED` | -32000 | exit ≠ 0, `{"error"}` ở stdout (exit 0), JSON hỏng | `tool`, `exitCode`, `stderrTail` ≤ 2 KiB (đã che), `reason` (`registry_unreadable`, `truncated_stdout`, `write_blocked`, `unknown_shape`, `unexpected_columns`, `index_not_updated`, `format_drift`), `retryable?` | tuỳ `retryable` |
| `CODEINTEL_PROFILE_UNKNOWN` | -32602 | `quality.run` với tên profile/suite lạ (không spawn gì) | `available[]` | không |
| `CODEINTEL_ENV_NOT_READY` | -32000 | **mọi** bước của run không sẵn sàng; coverage thiếu provider; tool không tương thích | `missing[]` (`{check, reason, hint, built?, required?}`), `reason` (`coverage_provider_missing`, `tool_incompatible`, `network_policy`, `go_modules_unavailable`), `missingTools[]` | không |
| `CODEINTEL_RUN_IN_PROGRESS` | -32000 | worktree đang có run hoặc hàng đợi đầy | `runId`, `reason` (`worktree_busy|queue_full`) | có |
| `CODEINTEL_RUN_NOT_FOUND` | -32602 | `runId` lạ hoặc không thuộc `workspaceRoot` | — | không |
| `CODEINTEL_RUN_CANCELLED` | -32000 | thao tác trên run đã huỷ không có ý nghĩa (ví dụ lấy `coverage` của run huỷ) | `runId` | không |
| `CODEINTEL_QUALITY_RUN_INTERRUPTED` | -32000 | agent khởi động lại khi run còn `running`; run không tự chạy lại | `runId`, `reason:"agent_restart"` | có (sau reconnect) |

`reason` ở trên là **giá trị chuỗi ổn định** (test hợp đồng khẳng định). `stale`, `truncated` là cờ trong kết quả, không phải lỗi. Method không tồn tại: `-32601` `MethodNotFound`.

**Hàng đợi method đầy**: trả `CODEINTEL_TIMEOUT data={reason:"queue_wait", queueSize:<n>}`; tên env timeout hàng đợi: `ORCA_CODEINTEL_QUEUE_WAIT_MS` (mặc định 10 000 ms). `ORCA_CODEINTEL_QUEUE_MAX` (mặc định 16, tối đa 32) kiểm soát kích thước hàng đợi.

### 3.3 Mã do **Go** sinh từ lỗi agent (agent KHÔNG bao giờ trả các mã này)
`CODEINTEL_AGENT_UNSUPPORTED` (infra-fleet khi `-32601` với `codeintel.*`/`quality.*`), `CODEINTEL_DEV_SERVER_OFFLINE` (collector hết chờ 20 s), `CODEINTEL_RESULT_INVALID` (collector giải mã thất bại), `CODEINTEL_TIMEOUT` (khi Go cắt trước), `CODEINTEL_OUTPUT_TOO_LARGE` (`ResourceExhausted` > 12 MiB). Các mã tầng service/gateway (`DISABLED`, `NOT_AUTHORIZED`, `RATE_LIMITED`, `VERSION_CONFLICT`, `UNAVAILABLE`, `RESPONSE_TOO_LARGE`, …) ở `CONTRACT-codeintel-ui-api.md` §2.3.

### 3.4 Chuyển sang gRPC/`apperrors` ở `infra-fleet-service` (CR-023; PQ-02)
Trong `RelayByDevServer.Execute`/`Relay.Execute`, **chỉ khi** `method` bắt đầu `codeintel.` hoặc `quality.`: `devserveragent.Client.Exec` trả `*domain.AgentRPCError{Code int, Message string, Data json.RawMessage}` (thay `*JSONRPCError` thô; `Unwrap()` → `ErrAgentMethodNotFound` khi `-32601`); use case đọc `Data` thành `{code string}`; nếu `code` ∈ **danh sách cho phép** (bảng 3.2 + `CODEINTEL_AGENT_UNSUPPORTED`) thì `apperrors.New(kind, code, cắt(Message, 300), err)`, ngược lại giữ `INFRA_AGENT_EXEC_FAILED`. Ánh xạ `Kind`:

| `data.code` | `apperrors.Kind` → gRPC |
|---|---|
| `TOOL_UNAVAILABLE, INDEX_MISSING, REPO_NOT_REGISTERED, REINDEX_IN_PROGRESS, OUTPUT_TOO_LARGE, AGENT_UNSUPPORTED, ENV_NOT_READY, RUN_IN_PROGRESS, RUN_CANCELLED` | `KindFailedPrecondition` |
| `PATH_NOT_ALLOWED` | `KindPermissionDenied` |
| `INVALID_PARAMS, AMBIGUOUS_SYMBOL, PROFILE_UNKNOWN` | `KindInvalidArgument` |
| `SYMBOL_NOT_FOUND, RUN_NOT_FOUND` | `KindNotFound` |
| `TIMEOUT` (và `context.DeadlineExceeded` của infra-fleet) | `KindDeadlineExceeded` |
| `TOOL_FAILED` | `KindInternal` |

Dữ liệu cấu trúc (`candidates`, `jobId`, `runId`, `missing`, `retryable`): **trailer gRPC `x-orca-agent-error-data-bin`** do handler `RelayByDevServer` (và `Relay`) đặt bằng `grpc.SetTrailer` (JSON `error.data`, ≤ 4 KiB, hợp lệ hoá bằng bỏ phần tử, không cắt giữa chuỗi). Collector đọc mã từ tiền tố message (`^CODEINTEL_[A-Z0-9_]+: `) và dữ liệu từ trailer; trailer mất/hỏng → vẫn dùng mã, bỏ `data`. `RelayStream` ngoài phạm vi. Trước CR-023: collector chỉ thấy `INFRA_AGENT_EXEC_FAILED` và suy giảm thành `CODEINTEL_TOOL_FAILED` (không hỏng).
Part B (`RelayDispatcher`) **làm rơi `err.data`** (`dispatcher.ts:485-491` gửi `{code, message}`): CR-006 sửa ≤ 5 dòng ở **cả** `agent/` và `desktop/` (mục 8); phương án dự phòng "mã trong `message` dạng `CODEINTEL_TIMEOUT: …`" **không khuyến nghị** (`candidates`/`jobId` phải nằm trong kết quả).

---
## 4. Method `codeintel.*`

> Ký hiệu: **bb** = bắt buộc; mặc định/giới hạn trong ngoặc. Mọi method nhận thêm `workspaceRoot` (bb). `data` là phần của phong bì mục 2.2. Ví dụ rút gọn; mọi `SymbolRef` theo mục 2.6.

### 4.1 `codeintel.status` (CR-001 §2.2, mở rộng CR-003 §2.1, CR-080 §2.3)
Tham số: `baseRef?` (string, 1..256, `[A-Za-z0-9._/@^~{}+-]`, không bắt đầu `-`; backend truyền nhánh gốc O7 để tính `mergeBase`; mặc định `origin/HEAD`). **Luôn thành công** khi agent chạy (thiếu công cụ/chỉ mục/repo chưa đăng ký không là lỗi).

Field bổ sung (B1): `compatibility` và `warnings[]` ổn định ở cấp trên cùng của `result.data`:
```jsonc
// Bổ sung vào result.data (bên cạnh "binding", "tools", "indexes", ...)
"compatibility": {
  "status": "verified",          // verified | untested | incompatible
  "reason": null,                // null khi verified; chuỗi mã ổn định khi không verified
  "details": null                // chuỗi mô tả tuỳ chọn
},
"warnings": [                    // mảng chuỗi mã ổn định; có thể rỗng
  // giá trị hợp lệ:
  // "tool_version_untested"       — binary có mặt nhưng ngoài dải đã kiểm
  // "index_built_with_old_extraction" — extractionVersion < hiện tại
  // "codeintel_disabled"          — ORCA_CODEINTEL_DISABLED=1 được đặt
  // "overlay_index"               — đang dùng OVERLAY (indexScope=repo_root)
  // "commit_mismatch"             — indexedCommit !== headCommit
]
```
`indexScope`/`freshness` được giao **theo thứ tự**: `indexScope` trả trước trong object JSON, `freshness` liền sau (`indexes.<tool>.indexScope`, `indexes.<tool>.freshness`) theo CR-080 §2.3 (B1).

```jsonc
// result.data
{ "binding": { "workspaceRoot": "/opt/repos/orca", "repoRoot": "/opt/repos/orca",
               "linkedWorktree": false, "worktreeMismatch": false,           // boolean (PQ-19)
               "gitnexus":  { "name": "orca", "path": "/opt/repos/orca", "storagePath": "/opt/repos/orca/.gitnexus" },   // null nếu không khớp
               "codegraph": { "projectPath": "/opt/repos/orca", "hasDatabase": true } },                                 // null nếu không có
  "tools": { "gitnexus":  { "available": true, "version": "1.6.9", "supported": true, "binary": "/usr/bin/gitnexus" },
             "codegraph": { "available": true, "version": "1.4.1", "supported": true, "binary": "/home/ubuntu/.local/bin/codegraph" } },
  "indexes": {
    "gitnexus": { "state": "stale",                 // missing|building|ready|stale|unknown  (sức khoẻ chỉ mục)
                  "indexedCommit": "d8198127b6…", "indexedAt": "2026-10-05T05:55:17.289Z", "branch": "main",
                  "stats": { "files": 20174, "nodes": 247556, "edges": 644157, "communities": 10252, "processes": 300 },
                  "schemaVersion": 5, "storagePath": "/opt/repos/orca/.gitnexus", "indicators": ["wal_missing_shadow_files:4"],
                  "indexRoot": "/opt/repos/orca", "indexScope": "repo_root",      // exact|repo_root|stale|none
                  "freshness": "fresh_base",                                      // fresh|fresh_base|stale|unknown
                  "headCommit": "1b0c760935…", "mergeBase": "d8198127b6…",
                  "dirtySinceIndex": true, "changedFilesNotInIndex": 42, "pendingChanges": null },
    "codegraph": { "state": "ready", "indexedAt": "…", "stats": { "files": 15773, "nodes": 295910, "edges": 959095 },
                   "pendingChanges": { "added": 0, "modified": 0, "removed": 0 },   // chỉ nghĩa khi gốc chỉ mục == workspaceRoot, ngược lại null
                   "backend": "node-sqlite", "journalMode": "wal", "dbSizeBytes": 1257566208, "extractionVersion": 24,
                   "reindexRecommended": false,
                   "rootMismatch": null,                                            // hoặc {worktreeRoot, indexRoot}  (PQ-19: đổi tên từ worktreeMismatch)
                   "indexRoot": "/opt/repos/orca", "indexScope": "exact", "freshness": "fresh",
                   "headCommit": "1b0c760935…", "mergeBase": "…", "dirtySinceIndex": false, "changedFilesNotInIndex": 0 } },
  "sqliteReadAvailable": true,
  "host": { "platform": "linux", "cores": 32, "loadavg1": 3.2, "freeMemBytes": 7700000000 },
  "limits": { "toolTimeoutMs": 20000, "methodTimeoutMs": 25000, "toolMaxOutputBytes": 16777216, "resultMaxBytes": 8388608, "maxConcurrentTools": 3 } }
```
- Phân loại `indexScope`/`freshness` (CR-080 `classifyIndexBasis`, dừng ở dòng đúng đầu tiên): (1) không có chỉ mục → `none`/`unknown`; (2) công cụ không dùng được → `none`/`unknown`; (3) gốc trùng `workspaceRoot` ∧ `indexedCommit==headCommit` ∧ `!dirtySinceIndex` (CodeGraph: `pendingChanges` toàn 0) → `exact`/`fresh`; (4) gốc trùng ∧ (commit khác ∨ bẩn ∨ pending>0) → `stale`/`stale`; (5) gốc khác ∧ `indexedCommit==mergeBase` → `repo_root`/`fresh_base`; (6) gốc khác ∧ commit khác → `stale`/`stale`. Có `lbug.wal.missing-shadow.*` → thêm `indicators` (không lỗi).
- Phong bì: `stale` = OR theo công cụ (`indexedCommit !== headCommit` ∨ `rootMismatch/worktreeMismatch` ∨ pending > 0). Đường dẫn tuyệt đối (`repoRoot`, `indexRoot`, `storagePath`, `binary`) **chỉ cho backend**: gateway **không** chuyển ra UI (che ở service).
- Tiêu chí: `codegraph sync` ở worktree liên kết trả `pendingChanges` của **checkout chính** ⇒ không dùng kết luận `fresh` (README v7 mục 8 điểm 21).

Ví dụ request: `{"jsonrpc":"2.0","id":7,"method":"codeintel.status","params":{"workspaceRoot":"/opt/repos/orca","baseRef":"origin/main"}}`.

### 4.2 `codeintel.overview` (CR-002 §2.6)
| Tham số | Kiểu | Mặc định | Biên |
|---|---|---|---|
| `topN` | int | 200 | 1..500 |
| `maxEdges` | int | 5000 | 1..5000 |
| `edgeKinds` | string[] | `["CALLS","IMPORTS"]` | tập con `CALLS, IMPORTS, ACCESSES, EXTENDS, IMPLEMENTS` |
| `withTopFiles` | bool | true | |
`data`: `{ "nodes": ClusterNode[], "edges": ClusterEdge[] }` với `ClusterNode = {id, label, symbolCount, cohesion, keywords[], topFiles[≤3], dominantLanguage, area}`, `ClusterEdge = {from, to, weight, kinds: {CALLS?: n, IMPORTS?: n}}`. `totalCount` = số cụm thật (9 039 ≠ `stats.communities` 10 252 của registry; chấp nhận, kèm `warnings:["communities_count_mismatch"]`); `truncated` khi `topN < tổng` hoặc `edges == maxEdges`. Thời gian kỳ vọng 5–7 s (một lần đo). Lỗi: `INDEX_MISSING`, `REPO_NOT_REGISTERED`, `TOOL_UNAVAILABLE`, `TIMEOUT`, `OUTPUT_TOO_LARGE`, `REINDEX_IN_PROGRESS`, `TOOL_FAILED`.

### 4.3 `codeintel.processes` / `codeintel.process` (CR-002)
- `processes`: `limit` (1..100, 50), `offset` (≥ 0, 0). `data: { "flows": FlowSummary[] }`, `FlowSummary = {id, label, processType, stepCount, communities: string[], entry: SymbolRef|null, terminal: SymbolRef|null}`; sắp `stepCount DESC, id`. **Lưu ý** (README v7 mục 8 điểm 9): `Process` của GitNexus **không** đi vào use case/adapter DB/gRPC ở backend Go; luồng nghiệp vụ Go do CR-034 dựng tĩnh.
- `process`: `processId` (bb, 1..128). `data: FlowGraph = { "flow": FlowSummary, "steps": [{step, symbol: SymbolRef, cluster: {id,label}|null, filePath, startLine}], "edges": [{fromKey, toKey, kind:"calls", confidence, reason}] }`; ≤ 200 bước (`stepCount` > 200 → `truncated:true`, `warnings:["steps_truncated"]`). Không thấy → `CODEINTEL_SYMBOL_NOT_FOUND` (`data.kind:"process"`).

### 4.4 `codeintel.subgraph` (CR-002, CR-003 §2.2)
| Tham số | Kiểu | Mặc định | Biên |
|---|---|---|---|
| `center` (bb) | đúng **một** khoá: `{"symbol": "<key hoặc gitnexus uid>"}` \| `{"file": "<rel>"}` \| `{"cluster": "<id>"}` | — | |
| `depth` | int | 1 | 1..3 (`file`/`cluster` luôn 1) |
| `kinds` | string[] | `["CALLS","IMPORTS","EXTENDS","IMPLEMENTS"]` | tập con cạnh hợp lệ (`CALLS, IMPORTS, ACCESSES, EXTENDS, IMPLEMENTS, MEMBER_OF, STEP_IN_PROCESS, DEFINES, HANDLES_ROUTE, FETCHES, HAS_METHOD, HAS_PROPERTY, METHOD_IMPLEMENTS, METHOD_OVERRIDES, CONTAINS`) |
| `limit` (số nút) | int | 800 | 1..1500 |
| `source` | `"gitnexus"` \| `"codegraph"` \| `"auto"` | `"auto"` | `codegraph` chỉ cho tầng `calls` khi SQLite khả dụng |
`data: SymbolGraph = { "center": SymbolRef, "nodes": [SymbolRef + {isExported, signature?, cluster?}], "edges": [{fromKey, toKey, kind, confidence, reason, line?, sources: ["gitnexus"|"codegraph"]}], "depth": n }`. Duyệt theo mức (≤ 3 truy vấn), frontier ≤ 300 id/lần; dừng khi nút ≥ `limit` hoặc cạnh ≥ 4 000 (`truncated:true`). Mặc định **loại** nút kind `value`/`Section` (trừ khi là tâm). Cạnh giữ `confidence` (0 = không biết).

### 4.5 `codeintel.impact` (CR-002, mở rộng CR-003/005)
| Tham số | Kiểu | Mặc định | Biên |
|---|---|---|---|
| `target` (bb) | `{"uid": "…"}` \| `{"name": "…", "file"?: "…", "kind"?: "…"}` (hoặc `{"key": "<SymbolRef.key>"}`) | — | |
| `direction` | `upstream` \| `downstream` | `upstream` | |
| `depth` | int | 2 | 1..3 |
| `limit` | int | 300 | 1..300 |
| `includeTests` | bool | false | |
`data: ImpactGraph = { "target": SymbolRef, "direction": "upstream", "risk": "LOW|MEDIUM|HIGH|CRITICAL|UNKNOWN", "impactedCount": n, "levels": [{ "depth": 1, "symbols": [{ "symbol": SymbolRef, "via": "calls", "confidence": 0.9, "direct": true }] }], "affectedFlows": [{ "flowId", "label", "stepCount", "changedStep"?: n }], "affectedModules": [{ "name", "hits", "impact" }], "testsCovering": [SymbolRef], "rawSummary": { "direct", "processes_affected", "modules_affected" } }`. **Không có cạnh** (hạn chế đã biết, PQ-19): GitNexus `byDepth` không có nút cha; `via` = `relationType`, `direct = depth==1`. `affectedFlows` dựng bằng truy vấn riêng (`SYMBOL_FLOWS`) chứ không dùng `affected_processes` của CLI. `testsCovering` lọc theo mẫu tên (`*.test.*`, `*.spec.*`, `_test.go`, `__tests__`, `tests/`) khi `includeTests`, ngược lại `[]` + `warnings:["tests_excluded"]`; nếu CodeGraph SQLite khả dụng thì điền từ `affected` (≤ 200 tệp test). Lỗi riêng: `AMBIGUOUS_SYMBOL` (kèm `candidates`, **không tự chọn**), `SYMBOL_NOT_FOUND`.

### 4.6 `codeintel.symbol` (CR-002, CR-003 §2.7)
| Tham số | Kiểu | Mặc định | Biên |
|---|---|---|---|
| `uid` **hoặc** (`name` + `file`) hoặc `key` | string | bb đúng một dạng | |
| `includeSource` | bool | true | |
| `relationLimit` | int | 20 | 1..50 |
| `includeTrail` | bool | false | chỉ khi CR-003 phát hành; `experimental:true`, text từ `codegraph node` |
`data: { "symbol": SymbolRef, "incoming": { "<loại cạnh thường>": [{uid, name, filePath}] }, "outgoing": { … }, "flows": [{id, label, stepCount, step}], "source": { "text": "…", "startLine": 72, "endLine": 113, "truncated": false } | null, "sourceOmitted": null | "gitignored" | "binary" | "not_requested", "trail"?: {…} }`. Mã nguồn: agent tự đọc `workspaceRoot/filePath` dòng `[startLine .. endLine]` (1-based) **sau khi** kiểm `realpath` nằm trong `workspaceRoot`; `git check-ignore -q -- <path>` mã 0 → `source:null, sourceOmitted:"gitignored"`; NUL trong 8 KiB đầu → `"binary"`; ≤ 200 KiB (cắt theo dòng, `source.truncated:true`); `stale`/`rootMismatch` → `warnings:["source_may_not_match_index"]`. **Backend** (không phải agent) che secret và chặn đường dẫn nhạy cảm (`.env`, `*.pem`… → `contentWithheld:"sensitive_path"`). Không cache khi có `source`.

### 4.7 `codeintel.routes` (CR-002)
`limit` (1..500, 200), `offset` (≥ 0). `data: RouteMap = { "routes": [{id, path, method, filePath, side: "server"|"client", handler: SymbolRef, middleware?: [], responseKeys?: [], errorKeys?: []}], "edges": [{route, handler, kind: "handles_route"|"fetches"}] }`. Cảnh báo `routes_coverage_js_only` khi repo có `backend-go/` (route gRPC/`wscompat` do CR-032). Orca: 92 `HANDLES_ROUTE` + 21 `FETCHES` (đếm theo CR, chưa kiểm).

### 4.8 `codeintel.detectChanges` (CR-005; PQ-19)
| Tham số | Kiểu | Mặc định | Ghi chú |
|---|---|---|---|
| `base` | string | nhánh gốc suy ra | 1..256, `[A-Za-z0-9._/@^~{}+-]`, không bắt đầu `-` |
| `head` | string | không đặt = **cây làm việc** | đặt thì KHÔNG tính thay đổi chưa commit |
| `includeUntracked` | bool | true (chỉ khi không có `head`) | ≤ 200 tệp |
| `withClusters` | bool | false | thêm `affectedClusters` |
| `crossCheck` | bool | false | `gitnexus detect-changes` đối chiếu (3 dòng đầu) |
`base` mặc định **không gọi mạng**: `git symbolic-ref --quiet refs/remotes/origin/HEAD` → thử `origin/main, origin/master, main, master`; không có → `CODEINTEL_INVALID_PARAMS reason="base_required"`. So với **merge-base** (O7). Git whitelist (tất cả ≤ 2.24; `-c core.quotePath=false --no-color --no-ext-diff --no-textconv`): `git diff --raw -z --no-abbrev -M <mb> [<head>]`, `git diff --numstat -z -M …`, `git diff --unified=0 -M …`, `git diff --name-only -z HEAD`, `git ls-files --others --exclude-standard -z`; không `--merge-base`, `merge-tree --write-tree`, `--path-format`.

```jsonc
// result.data (ChangeOverlay thô của agent; backend hợp nhất thành ChangeOverlay đầy đủ ở CR-036)
{ "base": { "ref": "origin/main", "oid": "1b0c76…", "mergeBase": "1b0c76…" },
  "head": { "ref": null, "oid": "1b0c76…", "includesUncommitted": true, "dirtyFileCount": 2 },
  "changedFiles": [ { "path": "agent/src/relay/context.ts", "oldPath": null, "status": "M", "additions": 3, "deletions": 1,
                      "hunks": [ { "oldStart": 30, "oldLines": 1, "newStart": 30, "newLines": 3 } ],
                      "untracked": false, "binary": false, "indexed": true, "driftedFromIndex": false } ],
  "changedSymbols": [ { "symbol": SymbolRef, "change": "modified", "hunkCount": 1, "linesTouched": 3,
                        "confidence": "exact", "containers": ["type:agent/src/relay/context.ts:RelayContext"] } ],
  "unmapped": { "filesWithoutSymbols": ["docs/crs/v7/README.md"], "filesNotIndexed": [], "filesBeyondCap": 0, "unsafePathFiles": [] },
  "affectedFlows": [ { "flowId": "proc_0_checkspanel", "label": "…", "stepCount": 9, "changedSymbolKeys": ["…"], "earliestChangedStep": 1 } ],
  "affectedClusters": null,                       // hoặc [{id,label,changedSymbols}] khi withClusters
  "riskHint": null,                               // hoặc {source:"gitnexus detect-changes", level, files, symbols, processes}
  "index": { "commit": "d8198127b6…", "stale": true, "driftedFileCount": 412, "mappingConfidence": "mixed" } }
```
Enum: `changedFiles[].status` mã raw git (`M|A|D|R|C|T|U`; `A` cho untracked); `change ∈ modified|added|deleted`; `confidence ∈ exact|approximate`; `mappingConfidence ∈ exact|mixed|approximate`. `warnings` (**ở phong bì**, không trong `data`): `index_commit_differs_from_head`, `index_commit_unreachable`, `unborn_head` (→ `changedFiles:[]`), `hunk_file_order_mismatch`, `hunks_unavailable_diff_too_large`, `crosscheck_mismatch` (lệch > 20%), `deadline_partial` (trả một phần + `truncated:true`; `CODEINTEL_TIMEOUT` chỉ khi chưa có `changedFiles`).
Giới hạn: ≤ 2 000 symbol (ưu tiên tệp `additions+deletions` giảm dần), ≤ 1 000 tệp ánh xạ (`filesBeyondCap`), ≤ 5 000 tệp trong `changedFiles`, ≤ 200 luồng, ≤ 200 untracked, ≤ 2 MiB/tệp untracked. Ánh xạ hunk→symbol: symbol trong cùng tệp thoả `s ≤ se ∧ e ≥ ss`, ưu tiên symbol con; `value` chỉ giữ khi không có hàm/lớp chứa; `Section` → `kind:"doc"`. Độ tin cậy từng tệp: `exact` khi `indexedCommit==headOid` và tệp sạch hoặc không thuộc `git diff --name-only <indexedCommit> <headOid>`; còn lại `approximate` (tệp bẩn luôn `approximate`). Lỗi: `INVALID_PARAMS` (`field:"base"`, `reason ∈ unresolved_ref|base_required|no_merge_base`), `TOOL_FAILED`, `TIMEOUT`. **Hạn chế**: `gitnexus detect-changes` chỉ in text (≤ 15 symbol/10 luồng) nên **chỉ dùng cho `crossCheck`**; `codeintel.detectChanges` tự ánh xạ bằng `git diff` + Cypher (mẫu `FILE_SYMBOLS_BATCH`, `MATCH (n)` + `IN` **chưa chạy**).

### 4.9 `codeintel.structuralFacts` (CR-037 §2.2; JSON của `data` là (mới) ở hợp đồng này)
| Tham số | Kiểu | Mặc định | Ghi chú |
|---|---|---|---|
| `kind` (bb) | `layerImports` \| `cycles` \| `importInDegree` \| `fileSizes` \| `unusedExports` | — | |
| `pair` | `usecase->adapter` \| `domain->usecase` \| `domain->adapter` \| `adapter->adapter` | không đặt = cả 4 | chỉ `layerImports` |
| `pathPrefixes` | string[] ≤ 20 | `["backend-go/services/"]` | tương đối gốc repo |
| `limit` | int | 5000 | 1..5000 |
| `offset` | int | 0 | ≥ 0 |
Thực thi (mẫu cố định, chỉ đọc; ~1,7–1,9 s/lệnh): `layerImports` = Cypher `IMPORTS` giữa `File` với cặp đoạn đường dẫn cố định, loại `_test.go`; `cycles` = `gitnexus check --cycles --json -r <repo>` (chỉ cờ này); `importInDegree` = `count(DISTINCT a)` theo `b.filePath`; `fileSizes` = hai truy vấn `Function` và `Method` gộp ở agent; `unusedExports` = `Function` có `isExported` không có cạnh vào (loại `_test.go`, `/cmd/`, `usecasetest`).
`data` theo `kind` (luôn kèm `"kind"`):
```jsonc
{ "kind": "layerImports",   "rows": [ { "pair": "usecase->adapter", "fromFile": "backend-go/services/infra-fleet-service/internal/usecase/ports.go", "toFile": "backend-go/services/infra-fleet-service/internal/adapter/eventbus/publisher.go" } ] }
{ "kind": "cycles",         "cycleCount": 93, "status": "cycles_found|clean", "cycles": [ { "files": ["a.ts","b.ts","a.ts"] } ] }
{ "kind": "importInDegree", "rows": [ { "file": "…", "inDegree": 123 } ] }
{ "kind": "fileSizes",      "rows": [ { "file": "…", "functions": 12, "totalLines": 480, "longest": { "name": "…", "lines": 90 } } ] }
{ "kind": "unusedExports",  "rows": [ { "symbol": SymbolRef } ] }
```
Lỗi: `TOOL_UNAVAILABLE`, `INDEX_MISSING`, `TOOL_FAILED` (`check` lỗi/cắt cụt qua pipe → dùng tệp tạm), `TIMEOUT`, `OUTPUT_TOO_LARGE`. Orca (đo một lần, chưa chạy lại): `cycles` = 93 vòng (backend-go 0), `unusedExports` 13/1231.

### 4.10 `codeintel.reindex` (CR-004, mở rộng CR-080; PQ-16) — trả ngay, chạy nền
| Tham số | Kiểu | Mặc định | Ghi chú |
|---|---|---|---|
| `mode` | `incremental` \| `full` | `incremental` | |
| `tools` | `("gitnexus"\|"codegraph")[]` | mọi công cụ khả dụng+hỗ trợ | thứ tự thực hiện cố định: `codegraph` rồi `gitnexus` |
| `trigger` | `manual` \| `agent_done` \| `head_change` | `manual` | `agent_done` đổi hành vi worktree liên kết |
| `ifStale` | bool | false | `freshness==="fresh"` thì `outcome:"already_up_to_date"`, không spawn |
| `expectHead` | string | — | HEAD hiện tại khác → `outcome:"superseded"`, không chạy |
Lệnh (chỉ ở `codeintel-reindex-commands.ts`, kiểu `ReindexCommand`): GitNexus incremental `gitnexus analyze --index-only <repoRoot>`, full thêm `--force`; CodeGraph incremental `codegraph sync <projectPath> -q`, full `codegraph index <projectPath> -q`. **`--index-only` bắt buộc**; cấm `--embeddings, --skills, --name, --branch, --drop-embeddings`. Env con `GITNEXUS_WORKER_POOL_SIZE` ← `ORCA_CODEINTEL_ANALYZE_WORKERS` (mặc định `max(1, min(4, floor(cpu/2)))`, giả định). **Worktree liên kết**: `trigger:"manual"` → `CODEINTEL_PATH_NOT_ALLOWED hint="reindex_linked_worktree_unsupported"`; `trigger:"agent_done"` → thành công `outcome:"skipped_scope_repo_root"`, `skipped:[{tool, reason:"index_root_is_main_checkout"}]` (không chạy `analyze/sync`).
```jsonc
// result (KHÔNG có phong bì)
{ "jobId": "ri_01J9ZK3Q8M2X",          // ULID
  "state": "running",                    // queued|running|succeeded (khi outcome != "")
  "workspaceRoot": "/opt/repos/orca", "repoRoot": "/opt/repos/orca",
  "mode": "incremental", "tools": ["codegraph", "gitnexus"], "trigger": "manual",
  "startedAt": "2026-10-05T14:00:00.000Z", "estimate": null,   // chưa có số đo thật
  "outcome": "", "skipped": [] }
```
Đồng thời: mỗi repo 1 job (khoá `realpath(repoRoot)`); toàn agent `ORCA_CODEINTEL_MAX_REINDEX` (1, tối đa 2), hàng đợi ≤ 4; trùng repo → `CODEINTEL_REINDEX_IN_PROGRESS data={jobId,state,stage}`; hàng đợi đầy → `data.reason="queue_full"`. Khi `analyze` GitNexus chạy: mọi method đọc GitNexus trả `REINDEX_IN_PROGRESS`; giai đoạn CodeGraph đóng SQLite; method chỉ cần công cụ kia vẫn chạy. Timeout job 45 phút/công cụ → `failed` (`CODEINTEL_TIMEOUT`). Journal `~/.orca/codeintel/jobs/<jobId>.json` (thư mục `0700`, giữ 50 bản, không ghi `message`). Session WS dừng **không** huỷ job. Thành công = exit 0 **và** bước `verify` (probe `indexedAt/lastCommit` đổi hoặc `pendingChanges`→0; nếu `indexedCommit===headCommit` → `succeeded` + `outcome:"already_up_to_date"`; không → `failed TOOL_FAILED reason="index_not_updated"`). Trạng thái job: `queued → running → (succeeded|failed|cancelled)`, thêm `cancelling`, `interrupted` (chỉ từ journal sau khởi động lại). **Chính tả `cancelled`** (không `canceled`).

### 4.11 `codeintel.reindexStatus` (mới, CR-004)
`{ jobId? }`. Kết quả `{ "job": { "jobId", "state", "stage", "percent": 0..100|null, "message", "tool"?: "gitnexus|codegraph", "startedAt", "finishedAt"?: , "error"?: {"code","message"}, "outcome"?: "", "indexHealth"?: "ok|unknown" } | null }` (không `jobId` → job gần nhất; không job → `{"job": null}`).
`stage ∈ preflight | codegraph.sync | codegraph.index | gitnexus.analyze | verify | done`.

### 4.12 `codeintel.reindexCancel` (mới, CR-004)
`{ jobId }` (bb). Idempotent: job đã kết thúc → trả trạng thái cuối, không lỗi. SIGTERM nhóm tiến trình (`detached:true`, `process.kill(-pid)`), sau 10 s SIGKILL; luôn `verify` sau huỷ; DB không rõ → `indexHealth:"unknown"` và `status` `state:"unknown"` tới khi reindex thành công. Kết quả `{ "job": {…như 4.11} }`. **Không có RPC/kênh backend gọi nó ở v7** (điểm mở O-17); chỉ dùng khi `CODEINTEL_AUTOANALYZE_CANCEL_ON_RESUME=true`.

### 4.13 `codeintel.watch` (mới, CR-004)
`{ enabled: bool }` (bb). Idempotent; **backend gọi lại sau mỗi lần agent nối lại** (B4: đây là trách nhiệm của backend collector, không phải agent — sau khi WS reconnect, backend phải gọi `codeintel.watch` + `codeintel.status` trước bất kỳ method nào khác để agent bắt đầu gửi thông báo); tối đa 8 repo đồng thời (vượt → `CODEINTEL_INVALID_PARAMS reason="watch_limit"`); lần đầu bật chỉ ghi nhớ, không phát. Kết quả `{ "enabled": true, "watching": 3 }`. Thăm dò (polling, không `fs.watch`): GitNexus `stat .gitnexus/meta.json` mỗi 10 s (debounce 2 s); CodeGraph `max(mtime)` của `codegraph.db` và `-wal` mỗi 10 s (≤ 1 thông báo/30 s/repo); HEAD `git rev-parse HEAD` mỗi 15 s.

### 4.14 `codeintel.codegraphSearch` và `codeintel.files` (mới, CR-003 §2.2)
- `codegraphSearch`: `search` (bb, 1..256), `limit` (1..50, 10), `kind?` ∈ `function, method, class, interface, struct, enum, type_alias, constant, variable, property, field, route, component, namespace`. `data: { "results": [{ "symbol": SymbolRef, "score": n }] }` (bỏ `kind: import`).
- `files`: `filter?` (đường dẫn tương đối ≤ 512, cấm bắt đầu `-` hoặc chứa `..`), `limit` (1..5000, 2000). `data: { "files": [{ "path", "language", "nodeCount", "size" }] }`, `truncated` khi > `limit`.
- Cả hai chỉ gọi `codegraph query|files` (`-p <root> -j`), exit 0 với lỗi dạng text ANSI (kiểm stdout bắt đầu `[`/`{` trước khi parse); `Symbol "…" not found` → `CODEINTEL_SYMBOL_NOT_FOUND`; `not initialized` → `INDEX_MISSING`. Phát hành ở đợt 4 (P1); backend phải chịu `-32601`.

### 4.15 Phương thức **không công khai**
`codeintel.node` (chỉ nội bộ cho `symbol.includeTrail`, `experimental:true`); mọi lệnh CLI ngoài whitelist.

---

## 5. Method `quality.*` (CR-081, 082, 083)

> Chỉ nhận **tên profile**, không lệnh. Mọi method nhận `workspaceRoot` (bb, PQ-21). Part A (`dispatchQualityRpc`); `relay-ssh`/Part B **không** có `quality.*` (CR riêng sau). Windows và `ORCA_QUALITY_RUN=off` → `CODEINTEL_TOOL_UNAVAILABLE reason="unsupported_platform|quality_disabled"`.

### 5.1 `quality.listProfiles`
```jsonc
{ "profiles": [
    { "id": "ts-lint", "title": "oxlint (TS/JS)", "kind": "lint", "scopes": ["worktree","changed","commitRange"],
      "heavy": false, "ready": true, "missing": [], "definitionHash": "sha256:9f…", "source": "builtin", "display": "oxlint --format json" },
    { "id": "go-lint", "kind": "lint", "ready": false, "scopes": ["worktree","changed"], "heavy": true,
      "missing": [ { "check": "golangci-lint", "reason": "tool_too_old", "built": "go1.22.2", "required": "go1.26.0", "hint": "install a golangci-lint built with Go >= 1.26" } ] } ],
  "suites": [ { "id": "fast", "profiles": ["ts-lint","repo-check-max-lines","go-vet"] } ],
  "host": { "platform": "linux", "cores": 32, "loadavg1": 3.1, "freeMemBytes": 7700000000 },
  "limits": { "maxConcurrentRuns": 1, "queueMax": 4, "runTimeoutMs": 2700000 } }
```
**Không lỗi** khi thiếu môi trường (`ready:false` + `missing[]`). `definitionHash` (đổi khi đổi `argv`) để backend ghim chính sách (CR-085). `display` chỉ để người đọc, không có biến môi trường/đường dẫn ngoài `workspaceRoot`. `kind ∈ lint|typecheck|test|proto|policy|repo-rules|coverage|security|dependency`. `missing[].reason ∈ binary_missing|node_modules_missing|native_runtime_unavailable|go_missing|go_too_old|go_modcache_empty|tool_too_old|tool_incompatible|base_ref_missing|tmp_space_low|home_missing|coverage_provider_missing|network_policy`. Preflight chỉ đọc (cache 60 s; không `pnpm install|rebuild`, `go mod download`, `ensure-native-runtime --runtime=…` — chỉ `--check-only`). **Lọc theo cờ tenant là việc của backend** (agent không biết cờ tenant): profile `security-*`/`dependency-diff` do agent liệt kê, backend ẩn khi `quality_security_scan_enabled` tắt (PQ-01).
Catalog mặc định Orca (đề xuất, chưa chạy): `ts-lint`, `ts-typecheck-{desktop-node,desktop-web,desktop-cli,agent,frontend}`, `ts-unit-{desktop,frontend,agent,backend}`, `repo-check-{max-lines,styled-scrollbars,reliability-gates}`, `go-vet`, `go-test`, `go-lint`, `proto-lint`, `proto-breaking`, `opa-test`, `coverage-go`, `coverage-ts`, `repo-rules`, `repo-rules-scripts`, `security-go-vuln`, `security-deps-osv`, `security-secrets-diff`, `dependency-diff`; suite `fast`, `standard`, `full`. Tên cuối do `AG-CV-SOL-081-quality-profile-catalog-and-preflight` chốt (đề xuất của CR).

**Argv mẫu bổ sung** (B6): `{gitCommonDir}` thay thế `<gitCommonDir>` trong argv profile cần truy cập `--git-dir` của worktree chính (ví dụ `buf breaking --against .git#{gitCommonDir}/..`). `scopeArgv` (B6): vitest related dùng `{relatedFiles}` (mảng đường dẫn tương đối, tối đa 300 tệp) thay vì đường dẫn tuyệt đối; nếu profile không khai báo `scopeArgv`, runner dùng `full-run-filter` theo mặc định.

**`QualityProfileDefinition` bổ sung** (B9):
- `whenChangedPaths: string[]` — glob tương đối gốc repo; nếu khai báo, profile chỉ chạy khi ≥ 1 tệp đổi khớp (CR-085). Cú pháp: `micromatch` standard, không `../`.
- `ciMappings: { checkName: string; provider?: 'github'|'gitlab' }[]` — ánh xạ profile sang CI check name để `CiComparison` đối chiếu (CR-086). Thiếu → `relation: 'no_ci_data'`.

### 5.2 `quality.run` — trả ngay (<1 s), chạy nền
Tham số: `profile` (bb, id profile **hoặc** suite), `scope` (bb, `worktree|changed|commitRange`), `base?` (bb với `changed|commitRange`; khớp `^[0-9a-f]{7,64}$` hoặc ref theo `git check-ref-format --branch`, không bắt đầu `-`). Kết quả:
```jsonc
{ "runId": "qr_01JA0X8E5T", "state": "queued", "profile": "standard", "scope": "changed",
  "steps": [ { "id": "ts-lint", "title": "oxlint (TS/JS)" }, { "id": "go-test:services/project-service", "title": "go test" } ],
  "headCommit": "1b0c760935…", "dirtyFingerprint": "sha256:…", "queuePosition": 0 }
```
Lỗi: `PROFILE_UNKNOWN` (`available[]`), `ENV_NOT_READY` (chỉ khi **mọi** bước không sẵn sàng; nếu chỉ vài bước thiếu thì run vẫn chạy và bước đó `env_not_ready`), `RUN_IN_PROGRESS` (`runId`, `reason`), `INVALID_PARAMS`, `PATH_NOT_ALLOWED`, `TOOL_UNAVAILABLE`. Mỗi worktree một run (khoá `realpath(workspaceRoot)`); khác worktree xếp hàng `queued` (≤ `ORCA_QUALITY_QUEUE_MAX`=4). Các bước chạy **tuần tự**. `scope:"changed"`: `append-files` (≤ 300 tệp/lần, hơn → `full-run-filter` + `scopeWidened`), `full-run-filter` (tsc, buf, opa: chạy đủ, đánh `inScope`), `go-modules` (module từ khối `use (...)` của `backend-go/go.work`; `common/**` hoặc `proto/**` đổi → **mọi** module + `scopeWidened`), `none`. Tham số `trust` **chưa** thêm (CR-081 Q2).
Giới hạn run: `runTimeoutMs` 45 phút; `maxOutputBytes` 32 MiB/bước (64 MiB `go-test`); spawn `shell:false`, `detached`, `nice 10`, cây tiến trình huỷ `SIGTERM → 5 s → SIGKILL`; `vitest --maxWorkers=max(1,cores/4)`, Go `GOMAXPROCS=max(2,cores/4)`; cổng nặng `ORCA_HEAVY_JOBS=1` (chờ ≤ 10 phút rồi bước `skipped reason=gate_timeout`); không chạy cùng `codeintel.reindex` nặng.

### 5.3 `quality.runStatus` `{runId}` (bb)
```jsonc
{ "runId": "qr_…", "state": "queued|running|cancelling|succeeded|failed|cancelled|interrupted",
  "startedAt": "…", "finishedAt": null,
  "skipReason": null,            // null | "scope_empty" | "all_steps_skipped" | "env_not_ready_all"
  "steps": [ { "id": "ts-lint", "status": "passed|findings|failed|timeout|cancelled|skipped|env_not_ready", "exitCode": 1, "durationMs": 21044,
               "findings": { "error": 3, "warning": 12, "info": 0 }, "truncated": false, "envMissing": [],
               "skipReason": null   // null | "scope_empty" | "disabled_by_policy" | "gate_timeout" | "env_not_ready"
             } ],
  "headCommit": "…", "dirtyFingerprint": "sha256:…", "workTreeChangedDuringRun": false, "scopeWidened": false,
  "summary": { "error": 3, "warning": 12, "info": 0, "stepsTotal": 6, "stepsWithFindings": 2, "stepsFailed": 0, "stepsEnvNotReady": 1, "outsideScope": 4, "truncated": false } }
```
Idempotent, đọc từ bộ nhớ hoặc nhật ký `~/.orca/quality/runs/<runId>.json` (≥ 50 bản, thư mục `0700`). `interrupted` = agent khởi động lại khi run còn `running` (không tự chạy lại); khi `state=interrupted`, `results` trả những phát hiện đã parse được trước khi ngắt, `coverage` trả `available:false reason:"run_interrupted"`. Sau khi WS nối lại backend gọi `runStatus` cho mọi run chưa kết thúc. Trạng thái bước: có phát hiện **không** làm run `failed`; run `failed` khi có bước `failed|timeout|env_not_ready` (kết quả **không đầy đủ**, cổng coi `unknown`).

### 5.4 `quality.cancel` `{runId}`
Idempotent, trả `runStatus` hiện tại; chuyển `cancelling`, diệt cây tiến trình (POSIX `kill(-pid)`; Windows `taskkill /T /F` chưa hỗ trợ), kết thúc `cancelled`; giữ các phát hiện đã parse (đánh `partial`). Tiêu chí: sau ≤ 15 s cả nhóm tiến trình biến mất.

### 5.5 `quality.results` `{runId, offset?=0, limit?=500 (1..500), view?="findings", stepId?}`
- `view="findings"`: `{ "view": "findings", "totalCount": 6000, "truncated": true, "outsideScopeCount": 4, "nextOffset": 500|null, "items": [QualityFinding] }` với
```jsonc
// QualityFinding (agent, camelCase)
{ "fingerprint": "9f2c…(32 hex)", "fpVersion": 1, "ruleId": "oxlint/eslint/no-unused-vars", "severity": "error|warning|info",
  "category": "lint|typecheck|test|coverage|complexity|security|dependency|convention|architecture|ai",
  "file": "frontend/src/a.ts", "line": 12, "endLine": 12, "column": 5, "endColumn": 9,   // 1-based, 0 = không có
  "message": "…(≤2 KiB, đã che)", "tool": "oxlint", "toolVersion": "1.71.0", "fixHint": "…", "stepId": "ts-lint", "inScope": true }
```
- `view="steps"`: `items: QualityStepResult[]` = `{id, profileId, status, failureKind, envReason, exitCode, durationMs, tool, toolVersion, definitionHash, errorCount, warningCount, infoCount, totalCount, truncated, outsideScopeCount}`; `failureKind ∈ ""|format_drift|output_too_large|exit_unexpected|parser_error|env`.
- `view="log"` (cần `stepId`): `{ "view":"log", "stepId", "text": "…(≤64 KiB, đã che)", "nextOffset": n|null }`. Quyền xem log: backend quyết (CR-081 Q8 chưa chốt).
Agent giữ kết quả tối đa `ORCA_QUALITY_RESULT_TTL_MS`=1 h và 20 run/worktree; backend **phải** ghi vào `quality_findings` khi nhận `quality.finished`. Ngân sách: ≤ 5 000 phát hiện/bước, ≤ 20 000/run (cắt `error` trước `warning` trước `info`; `inScope=true` trước); `message` ≤ 2 048 byte UTF-8; một trang ≤ 500 mục và ≤ 1 MiB (gRPC 4 MiB). **Fingerprint tính ở agent** (CR-082): `sha256("v1"␀tool␀ruleId␀file␀anchor␀normMessage␀occurrence)[0:32]`, `anchor` = dòng nguồn chuẩn hoá (chỉ **băm**, dòng nguồn không bao giờ ra RPC), không chứa số dòng.

### 5.6 `quality.coverage` `{runId}` (CR-083)
```jsonc
{ "runId": "qr_…", "available": true,
  "report": { "source": "measured|estimated", "language": "go|ts", "mode": "set|count|atomic|v8",
              "headCommit": "…", "baseCommit": "…", "dirty": false,
              "totals": { "stmts": 12000, "covered": 8123, "pct": 0.677 },
              "diff": { "changedExecutable": 120, "covered": 90, "uncovered": 30, "diffCoverage": 0.75,   // null + reason khi mẫu số 0
                        "reason": null, "partial": false, "modules": [], "noTests": [], "excludedFiles": [{"path","reason"}], "unmappedBlocks": 0 },
              "files": [ { "path": "…", "stmts": 100, "covered": 80, "pct": 0.8, "functions": [{"name","line","pct"}], "uncoveredRanges": [[10,14]] } ],
              "truncated": false, "totalCount": 1500, "toolVersions": { "go": "1.26.0" }, "estimatedNote": null } }
```
Run chưa xong → `RUN_IN_PROGRESS`; run không có bước coverage → `{ "runId", "available": false, "reason": "no_coverage_step" }`; thiếu `@vitest/coverage-v8` → `ENV_NOT_READY reason:"coverage_provider_missing"` (không ra số 0). `source:"estimated"` (từ `ChangeOverlay.uncoveredSymbols`) chỉ có `totals.changedSymbolsTested/Untested/Unknown`, **không** có phần trăm câu lệnh. Go: `go test -covermode=set -coverprofile=<tmp>/<module>.out ./...` mỗi module từ `go.work`, không `-coverpkg=./...`, không `-tags=integration`; TS (giai đoạn B): `vitest run … --coverage.provider=v8 --coverage.reporter=json`, cần `@vitest/coverage-v8` cùng phiên bản `vitest` (O12, cần duyệt). Payload ≤ 2 000 tệp và ≤ 1 MiB; `uncoveredRanges` chỉ cho tệp đã đổi hoặc `pct < 0.5`.

---

## 6. Thông báo agent → backend (notification, không `id`; qua `makeNotifier(ws, state)`)

> Gửi qua `codeintel-notification-sink.ts` (đối tượng "notifier hiện hành" = của lần gọi `codeintel.*`/`quality.*` gần nhất ⇒ **thông báo chỉ tới nơi sau khi backend gọi bất kỳ method nào** của nhóm này; backend gọi `codeintel.watch` + `codeintel.status` ngay sau mỗi lần nối lại). WS chưa mở → **bỏ** (không xếp hàng). Part B dùng `dispatcher.notify` tới mọi client. Mọi thông báo mang `workspaceRoot` (PQ-17); backend ánh xạ về binding bằng `(tenant, dev_server_id, path_hash)`. Phía infra-fleet chuyển tới `StreamCodeIntelEvents` (mục 2.3 của file proto).

### 6.1 `codeintel.indexChanged`
```jsonc
{ "jsonrpc": "2.0", "method": "codeintel.indexChanged",
  "params": { "workspaceRoot": "/opt/repos/orca", "tool": "gitnexus",     // gitnexus | codegraph | git
              "commit": "d8198127b6…", "indexedAt": "2026-10-05T05:55:17.289Z",
              "reason": "index",                                           // index | head | reindex
              "headCommit": "1b0c760935…", "stale": true,
              "indexScope": "repo_root", "mergeBase": "d8198127b6…", "trigger": "manual" } }   // 3 trường cuối: CR-080, tuỳ chọn
```
`tool:"git"` + `reason:"head"` chỉ cập nhật `stale`, **không** kích hoạt `analyze`. Sau reindex thành công phát ngay `reason:"reindex"` và huỷ cache ngắn hạn của agent. `lastIndexed` của CodeGraph **không** là tín hiệu (daemon auto-sync không đổi).

### 6.2 `codeintel.reindexProgress`
```jsonc
{ "jsonrpc": "2.0", "method": "codeintel.reindexProgress",
  "params": { "jobId": "ri_01J9ZK3Q8M2X", "workspaceRoot": "/opt/repos/orca",
              "state": "running",                  // queued|running|succeeded|failed|cancelled
              "tool": "gitnexus", "stage": "gitnexus.analyze",
              "percent": null,                     // số 0..100 hoặc null (chỉ số khi dòng khớp /(\d{1,3})%/)
              "message": "Parsing files…", "at": "2026-10-05T14:01:07.220Z",
              "errorCode": null, "outcome": "" } }  // errorCode/outcome khi state kết thúc
```
≤ 1 thông báo/giây/job (luôn phát khi đổi `stage`/`state`); `message` = dòng cuối stdout/stderr, bỏ ANSI, ≤ 200 ký tự, loại `$HOME`. `percent` **không** suy diễn theo thời gian.

### 6.3 `quality.progress` (CR-081 §2.6; thêm `workspaceRoot` — PQ-17)
```jsonc
{ "jsonrpc": "2.0", "method": "quality.progress",
  "params": { "runId": "qr_01JA0X8E5T", "workspaceRoot": "/opt/repos/orca", "stage": "step:ts-lint",
              "stepIndex": 1, "stepCount": 6, "percent": 16, "message": "ts-lint running", "at": "2026-10-06T08:00:12.100Z" } }
```
`percent` = `floor(completedSteps / stepCount * 100)` (số nguyên 0..100, chỉ đổi khi một bước xong) hoặc `null` khi `stepCount=0` hoặc không xác định; ≤ 1/giây/run; `message` ≤ 200 ký tự đã che. (B5: `percent:16` trong ví dụ trước là giá trị ví dụ, không phải định nghĩa — nay đã chốt công thức.)

### 6.4 `quality.finished`
```jsonc
{ "jsonrpc": "2.0", "method": "quality.finished",
  "params": { "runId": "qr_…", "workspaceRoot": "/opt/repos/orca",
              "status": "succeeded",   // succeeded | failed | cancelled | interrupted
              "summary": { "error": 3, "warning": 12, "info": 0, "stepsTotal": 6, "stepsWithFindings": 2, "stepsFailed": 0, "stepsEnvNotReady": 1, "outsideScope": 4, "truncated": false },
              "steps": [ { "id": "ts-lint", "status": "findings", "exitCode": 1, "durationMs": 21044, "truncated": false } ],
              "headCommit": "…", "dirtyFingerprint": "sha256:…", "workTreeChangedDuringRun": false,
              "startedAt": "…", "finishedAt": "…", "errorCode": null } }
```
`status` bao gồm `interrupted` (B5): khi `interrupted`, `summary` chứa những gì đã đếm được, `truncated:true` ở summary. `QualityRun.status` trên backend **cũng** có giá trị `interrupted` (lưu trong DB, không phải chỉ ở agent). Kết quả (`results`, `coverage`) của run `interrupted` hoặc hết TTL: `results` trả `partial:true` + phát hiện đã parse; `coverage` trả `available:false reason:"run_interrupted"|"run_expired"`.

Mất WS: run **tiếp tục** (chỉ bộ phát thông báo bị gỡ). Backend gọi `runStatus` + `results` (phân trang `limit 500` tới `nextOffset:null`) sau `finished` hoặc sau khi nối lại; **nạp findings trước, `Finish` run sau** (idempotent).

---
## 7. Ví dụ trọn vẹn

**7.1 `impact` gặp symbol mơ hồ** (agent → Go; `error.data` bị mất ở Go nếu CR-023 chưa làm):
```jsonc
// request
{ "jsonrpc": "2.0", "id": "41", "method": "codeintel.impact",
  "params": { "workspaceRoot": "/opt/repos/orca", "target": { "name": "runToolCommand" }, "direction": "upstream", "depth": 2, "includeTests": true } }
// response
{ "jsonrpc": "2.0", "id": "41",
  "error": { "code": -32000, "message": "ambiguous symbol 'runToolCommand'",
             "data": { "code": "CODEINTEL_AMBIGUOUS_SYMBOL",
                       "candidates": [ { "uid": "Function:agent/src/relay/agent-tool-registry.ts:runToolCommand", "name": "runToolCommand", "kind": "function",
                                          "filePath": "agent/src/relay/agent-tool-registry.ts", "line": 73, "score": 0.91, "impactedCount": 12, "risk": "HIGH" },
                                        { "uid": "Function:desktop/src/relay/agent-tool-registry.ts:runToolCommand", "name": "runToolCommand", "kind": "function",
                                          "filePath": "desktop/src/relay/agent-tool-registry.ts", "line": 73, "score": 0.88, "impactedCount": 3, "risk": "MEDIUM" } ] } } }
```
Sau CR-023 → gRPC `FailedPrecondition`? **không**: `AMBIGUOUS_SYMBOL` → `InvalidArgument`; message `CODEINTEL_AMBIGUOUS_SYMBOL: ambiguous symbol 'runToolCommand'`; trailer `x-orca-agent-error-data-bin` mang `data`; service dựng message cho UI: `CODEINTEL_AMBIGUOUS_SYMBOL: ambiguous symbol 'runToolCommand' | {"candidates":[…≤10…]}` (PQ-02).

**7.2 `reindex` rồi tiến độ rồi kết thúc**:
```jsonc
{ "jsonrpc":"2.0","id":"52","method":"codeintel.reindex","params":{"workspaceRoot":"/opt/repos/orca","mode":"incremental","trigger":"manual"} }
{ "jsonrpc":"2.0","id":"52","result":{"jobId":"ri_01J9ZK3Q8M2X","state":"running","workspaceRoot":"/opt/repos/orca","repoRoot":"/opt/repos/orca","mode":"incremental","tools":["codegraph","gitnexus"],"trigger":"manual","startedAt":"2026-10-05T14:00:00.000Z","estimate":null,"outcome":"","skipped":[]} }
{ "jsonrpc":"2.0","method":"codeintel.reindexProgress","params":{"jobId":"ri_01J9ZK3Q8M2X","workspaceRoot":"/opt/repos/orca","state":"running","tool":"codegraph","stage":"codegraph.sync","percent":null,"message":"","at":"2026-10-05T14:00:01.100Z"} }
{ "jsonrpc":"2.0","method":"codeintel.indexChanged","params":{"workspaceRoot":"/opt/repos/orca","tool":"codegraph","commit":null,"indexedAt":"2026-10-05T14:02:10.000Z","reason":"reindex","headCommit":"1b0c760935…","stale":false} }
{ "jsonrpc":"2.0","method":"codeintel.reindexProgress","params":{"jobId":"ri_01J9ZK3Q8M2X","workspaceRoot":"/opt/repos/orca","state":"succeeded","tool":"gitnexus","stage":"done","percent":100,"message":"","at":"2026-10-05T14:09:40.000Z","errorCode":null,"outcome":""} }
```

**7.3 `quality.run` → `quality.progress` → `quality.finished` → `results`**:
```jsonc
{ "jsonrpc":"2.0","id":"60","method":"quality.run","params":{"workspaceRoot":"/opt/repos/orca","profile":"fast","scope":"changed","base":"origin/main"} }
{ "jsonrpc":"2.0","id":"60","result":{"runId":"qr_01JA0X8E5T","state":"running","profile":"fast","scope":"changed","steps":[{"id":"ts-lint","title":"oxlint (TS/JS)"}],"headCommit":"1b0c76…","dirtyFingerprint":"sha256:ab…","queuePosition":0} }
{ "jsonrpc":"2.0","method":"quality.progress","params":{"runId":"qr_01JA0X8E5T","workspaceRoot":"/opt/repos/orca","stage":"step:ts-lint","stepIndex":1,"stepCount":1,"percent":null,"message":"ts-lint running","at":"…"} }
{ "jsonrpc":"2.0","method":"quality.finished","params":{"runId":"qr_01JA0X8E5T","workspaceRoot":"/opt/repos/orca","status":"succeeded","summary":{"error":1,"warning":0,"info":0,"stepsTotal":1,"stepsWithFindings":1,"stepsFailed":0,"stepsEnvNotReady":0,"outsideScope":0,"truncated":false},"steps":[{"id":"ts-lint","status":"findings","exitCode":1,"durationMs":9100,"truncated":false}],"headCommit":"1b0c76…","dirtyFingerprint":"sha256:ab…","workTreeChangedDuringRun":false,"startedAt":"…","finishedAt":"…","errorCode":null} }
{ "jsonrpc":"2.0","id":"61","method":"quality.results","params":{"workspaceRoot":"/opt/repos/orca","runId":"qr_01JA0X8E5T","offset":0,"limit":500} }
{ "jsonrpc":"2.0","id":"61","result":{"view":"findings","totalCount":1,"truncated":false,"outsideScopeCount":0,"nextOffset":null,"items":[{"fingerprint":"9f2c…","fpVersion":1,"ruleId":"oxlint/eslint/no-unused-vars","severity":"error","category":"lint","file":"frontend/src/a.ts","line":12,"endLine":12,"column":5,"endColumn":9,"message":"'x' is assigned a value but never used","tool":"oxlint","toolVersion":"1.71.0","stepId":"ts-lint","inScope":true}]} }
```

**7.4 Lỗi hiếm gặp thường bị bỏ sót**: `quality.run` với tên lạ `{"profile":"rm -rf /"}` → `-32602` `CODEINTEL_PROFILE_UNKNOWN` (`data.available`), **không spawn**; `codeintel.symbol` với `workspaceRoot:"../../etc"` → `-33002` `CODEINTEL_PATH_NOT_ALLOWED` (từ chối **trước** `expandTilde`); GitNexus trả `{"error": "..."}` ở stdout với exit 0 → `CODEINTEL_TOOL_FAILED reason="unknown_shape"` hoặc `INVALID_PARAMS`/`SYMBOL_NOT_FOUND` theo nội dung (`not found|does not exist`), `Write operations` → `TOOL_FAILED reason="write_blocked"`.

---

## 8. Part B (`relay-ssh` kiểu cũ / Electron) — chỉ khi O-5 chấp thuận (CR-006)

- **Không có method mới trên dây**; `registerCodeIntelHandlers(dispatcher, config, log)` đăng ký **cùng** `CODEINTEL_METHODS` (dòng gọi duy nhất sau `registerAuthStatusHandlers`, `relay.ts:494` theo CR; chưa kiểm). Lõi `codeintel-*.ts` trung lập truyền tải: **không** import `ws`, `WireState`, `makeNotifier`, `makeError`; `CodeIntelNotificationSink = (method, params) => void` (`dispatcher.notify`).
- Khác biệt buộc xử lý (theo CR-006, chưa kiểm chứng toàn bộ): `error.data`: Part A giữ (`makeError`), **Part B bỏ** `err.data` (`dispatcher.ts` gửi `{code: err.code ?? -32000, message}`) → sửa ≤ 5 dòng ở **cả** `agent/` và `desktop/` để gửi `{code, message, ...(data !== undefined && {data})}`; id Part A chuỗi|số, Part B số; thông báo Part A theo kết nối, Part B tới mọi client; Part B có `rpc.cancel {id}` → `context.signal` (kill tiến trình CLI), Part A không; handshake B (`orca-relay-handshake-ok`) **không** có `capabilities` → backend gọi `codeintel.status`, coi `-32601` = không hỗ trợ; **không** thêm trường vào `relay.status`.
- Kiểm thử bắt buộc: `JSON.stringify(result)` Part A == Part B (trừ `perf`, `startedAt`); thông báo tới client B qua `dispatcher.notify`; script parity `config/scripts/check-codeintel-relay-parity.mjs` (vị trí chưa chốt).
- Quality (`quality.*`) **không** có ở Part B ở v7.

---

## 9. An toàn (tổng hợp, bắt buộc kiểm thử)

1. **Whitelist lệnh**: mỗi lệnh là đối tượng có kiểu; test `TestWhitelistIsClosed` (GitNexus = `query, context, impact, trace, cypher, list, status, detect-changes` + `check --cycles` cho `structuralFacts`; CodeGraph = `query, callers, callees, impact, files, status, node, explore, affected`; `analyze/sync/index` chỉ ở reindex), `TestNoForbiddenSubcommand`, `TestUserValuesNeverStartWithDash`, `TestRepoFlagIsLast` (từ chối `--repo=x`, `-rx` trước spawn — đã tái hiện: `gitnexus impact -r orca handleInvoke "--repo=vnp-workplace"` trả dữ liệu repo khác), `TestReindexArgvIsIndexOnly`, `TestSpawnNeverUsesShell`, `TestParamsStrict`.
2. **Phân giải repo** (CR-001 §2.4, thứ tự): hình dạng → `realpath` → allowed roots → `git rev-parse --show-toplevel --git-common-dir` (qua `GitCapabilityCache`, ưu tiên `--path-format=absolute`, dự phòng bỏ cờ; baseline Git 2.25) → `git worktree list --porcelain` (mục đầu = checkout chính; không `-z`) → khớp `~/.gitnexus/registry.json` theo **đường dẫn chính xác sau `realpath`** (không khớp tiền tố/tên; trùng → chọn `indexedAt` mới nhất + cảnh báo; hỏng → `TOOL_FAILED reason="registry_unreadable"`) → CodeGraph: thư mục chứa `.codegraph/codegraph.db` (thử toplevel rồi checkout chính) → không có cả hai → `REPO_NOT_REGISTERED`. Worktree liên kết: `linkedWorktree:true, worktreeMismatch:true, stale:true`.
3. **Agent không có "workspace root đăng ký"**: `registerRoot` là no-op, `fs.*` nhận mọi đường dẫn; chỉ `fs.writeFile` có kiểm tra SecureFs (`agent/src/relay/fs-agent-write-extensions.ts:43`, theo README v7 mục 8 điểm 1). Ranh giới thật = registry GitNexus + phân quyền backend; `codeintel.*` và `RepoSourceReader` (CR-030) **tự chặn** đường dẫn.
4. **Secret**: không đưa vào kết quả/log/cache; stderr tail đã loại `HOME`, che mẫu (`gh[pousr]_…`, `github_pat_…`, `AKIA…`, `sk-…`, JWT, PEM, `scheme://user:pass@`); mã nguồn chỉ khi mở một symbol; `.env*`, `*.pem`, `*.key`, `id_rsa*` không bao giờ trả; test canary (CR-072 §2.6).
5. **Quality**: profile là dữ liệu của người phát hành agent (L1) hoặc quản trị dev server (L2 `~/.orca/quality/profiles.json`, `0600`, thư mục `0700`, chỉ **thêm** hoặc **vô hiệu hoá**; ghi đè `argv` chỉ với `replace:true` + lý do log); `.orca/quality.json` trong repo (P1) chỉ chọn tập con + hạ `timeoutMs`; **backend không giữ lệnh**. Mẫu thay `argv` chỉ `{bin:<tên>}`, `{files}`/`{files|<mặc định>}`, `{tmp:<tên>}`, `{base}`; tên tệp chèn bị từ chối nếu NUL/điều khiển/bắt đầu `-`/`realpath` thoát worktree. Cô lập mức L0 mặc định; `ORCA_QUALITY_ISOLATION=bwrap` (L1, P1, chưa thử).
6. **Tải**: tiến trình ≤ 3; reindex ≤ 1 (tối đa 2); run ≤ 1/worktree (hàng đợi 4); nặng 1; không chạy `codeintel.reindex` nặng cùng `quality.run` nặng.

---

## 10. Hạn chế đã biết (không phải lỗi hợp đồng)

- `ImpactGraph` không có cạnh; `impact` chỉ một `target` mỗi lần; chuyển tên `affected_clusters` ở hai method (PQ-19).
- `detectChanges` phụ thuộc Cypher `IN`/`MATCH (n)` chưa chạy trên LadybugDB; `gitnexus detect-changes` chỉ text; `head` không đặt = cây làm việc **chưa kiểm chứng** ở `branchCompare` (`git-handler-ops.ts:124`, phải sửa nhận `headRef`, cần `impact` trước khi sửa).
- GitNexus 0-based, CodeGraph 1-based; `communities` 10 252 (meta) ≠ 9 039 (đồ thị); mọi số hiệu năng là một lần đo trên một máy.
- Worktree liên kết **không có `.gitnexus` riêng**: chỉ mục của checkout chính (`repo_root`/`stale`/`OVERLAY`); `codegraph sync` ở đó báo `pendingChanges` của checkout chính; chưa chọn `per_worktree` (CR-080 M1–M7 chưa chạy).
- `node:sqlite` (CR-003) là experimental, cần Node ≥ 22.5, schema nội bộ CodeGraph (`schema_versions` max 8, `extractionVersion` 24); không thêm dependency.
- `gitnexus` stdout bị cắt cụt qua pipe nhưng vẫn `exit 0` (phải đọc từ tệp tạm); `cypher` lỗi trả `{error}` với exit 0; markdown không thoát `|` và gộp xuống dòng (parser theo cột cố định, dòng lệch bị bỏ + cảnh báo); `CALL show_tables()` vẫn chạy được (danh sách cấm của ta).
- Phản hồi lớn: `data` GitNexus phải qua tệp tạm, tối đa 16 MiB stdout / 8 MiB JSON; khung WS 16 MiB; gRPC mặc định 4 MiB (nên collector đặt `MaxCallRecvMsgSize(16 MiB)` — **hiện không nơi nào trong backend-go đặt**, đã grep).
- `toolEnv` của agent chứa toàn bộ `process.env` (kể cả `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`); các tool `gitnexus`/`codegraph` hiện có vẫn chạy với env đó (CR-072 phải xử lý).
- CI hiện **không chạy test của `agent/`** và `pr.yml` trỏ `config/vitest.config.ts` không tồn tại (README v7 mục 8 điểm 16/20): mọi test hợp đồng phía agent cần job CI riêng (CR-070 `code-intel-contract`).
- Đường dẫn script/baseline lệch (README v7 mục 8 điểm 20): `check-*` nằm ở `desktop/config/scripts/`, không ở gốc; profile chạy với `cwd` đúng thư mục chứa `config/`; chưa chạy thử.
- Windows chưa hỗ trợ (`unsupported_platform`); WSL/UNC từ chối.
- `quality.coverage` ở TS phụ thuộc duyệt `@vitest/coverage-v8`; Go cần `go.work` 1.26 (CI 1.25, golangci-lint v1.62.2 build go1.22.2 có thể không chạy).

---

## 11. Việc phải làm ở `agent/` (tóm tắt cho `AG-CV-SOL-*`)

| Việc | Nơi | CR |
|---|---|---|
| Thêm `dispatchCodeIntelRpc`, `CODEINTEL_METHODS`, whitelist, giới hạn, mã lỗi, `buildCodeIntelResult`, phân giải repo | `agent/src/relay/agent-rpc-dispatch-codeintel.ts` (mới) + `codeintel-*.ts` (mới); sửa `route()` trước `MethodNotFound`, `extractTraceFields` | 001 |
| Capability `codeintel*`, mở `AgentCapability` | `agent-session-capabilities.ts`, `shared/agent-wire-protocol.ts` | 001, 081 |
| Trích xuất GitNexus (mẫu Cypher, parser markdown, `SymbolRef`) | `gitnexus-*.ts`, `codeintel-symbol-ref.ts` (mới) | 002 |
| CodeGraph + SQLite read-only | `codegraph-*.ts` (mới) | 003 |
| `reindex*`, `watch`, thông báo, journal | `codeintel-reindex-*.ts`, `codeintel-notification-sink.ts` (mới) | 004, 080 |
| `detectChanges` | `codeintel-detect-changes.ts` (mới); sửa nhỏ `git-handler-ops.ts` `branchCompare` (cần `impact`) | 005 |
| `structuralFacts` + `gitnexus check --cycles` vào whitelist | (mới) | 037 |
| `dispatchQualityRpc`, profile, preflight, runner, parsers, fingerprint, rule pack, coverage | `quality-*.ts` (mới) | 081–084, 091 |
| Fixture vàng + chụp fixture (`agent/scripts/capture-codeintel-fixtures.mjs`) + job CI `code-intel-contract` | `agent/src/relay/codeintel/__fixtures__/…` | 070 |
| Part B (nếu O-5 chấp thuận) + sửa `dispatcher.ts` hai cây | `desktop/src/relay/*` | 006 |

*Kết thúc `CONTRACT-codeintel-agent-rpc.md`.*
