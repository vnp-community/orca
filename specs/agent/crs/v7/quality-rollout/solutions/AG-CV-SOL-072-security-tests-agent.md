# AG-CV-SOL-072: Kiểm thử bảo mật phía `agent/` (whitelist, Cypher, đường dẫn, phân giải repo, env, secret, giới hạn)

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mọi dòng "đã đọc" là đọc code/tài liệu thật; chưa chạy build, test hay công cụ nào. Các phát hiện kiểu "đã tái hiện" (ví dụ `--repo=` làm lộ repo khác) là của người soạn CR-CV-072 ngày 2026-10-05, **tôi chưa chạy lại**.

**CR:** [CR-CV-072](../../../../../../docs/crs/v7/quality-rollout/CR-CV-072-security-tests.md) (P0, Medium) — phần phía `agent/` (mục 2.2, 2.3, 2.4 phía agent, 2.5 phía agent, 2.6 phía agent, 2.10 một phần, 2.11)
**Service:** `agent/`
**Hợp đồng chuẩn tắc:** [`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md) (chính, đặc biệt §2.1, §2.4, §3.2, §9), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md), [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md)
**TDD tham chiếu:** [v5/05-tool-registry](../../../../tdd/v5/05-tool-registry.md), [v5/06-tool-handlers](../../../../tdd/v5/06-tool-handlers.md), [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md), [v5/10-git-handler-extension](../../../../tdd/v5/10-git-handler-extension.md), [v5/11-fs-handler-extension](../../../../tdd/v5/11-fs-handler-extension.md)
**Mẫu định dạng:** `specs/agent/crs/v6/agent-capabilities/solutions/`

## 0. Hợp đồng áp dụng

| Mục hợp đồng | Áp dụng |
|---|---|
| Agent contract §9.1 (`TestWhitelistIsClosed`, `TestNoForbiddenSubcommand`, `TestUserValuesNeverStartWithDash`, `TestRepoFlagIsLast`, `TestReindexArgvIsIndexOnly`, `TestSpawnNeverUsesShell`, `TestParamsStrict`; danh sách whitelist GitNexus/CodeGraph chính xác) | Tập test cốt lõi (task 02, 03) |
| §2.1 (`workspaceRoot` bắt buộc ở mọi method; kiểm: tuyệt đối, không NUL, ≤ 4096, `realpath`, **gốc git worktree**, `ORCA_CODEINTEL_ALLOWED_ROOTS`; tham số lạ bị từ chối `CODEINTEL_INVALID_PARAMS`; không bao giờ nhận `args, argv, command, cmd, cwd, env, repo, cypher, shell, timeout, tool`; chuỗi 1..512, không NUL/điều khiển, **không bắt đầu `-` kể cả U+FF0D, U+2212 sau NFKC**; đường dẫn tham số tương đối gốc repo, không `..`, không `\`) | Test tham số và đường dẫn (task 03, 05) |
| §2.4 (`spawn(file, argv, {shell:false})`; GitNexus luôn `-r <registryPath tuyệt đối>`; CodeGraph luôn `-p`; `assertReadOnlyCypher` ≤ 16 KiB, một câu, bắt đầu `MATCH `, cấm `CREATE\|MERGE\|DELETE\|SET\|REMOVE\|DROP\|ALTER\|COPY\|DETACH\|CALL\|LOAD\|INSTALL\|ATTACH\|EXPORT\|IMPORT\|FOREACH\|UNWIND`; bốn bộ mã hoá `cypherInt/String/StringList/KindList`; danh sách cấm subcommand; env con của quality bắt đầu từ rỗng, loại biến khớp `/(TOKEN\|SECRET\|PASSWORD\|PASSWD\|CREDENTIAL\|API_?KEY\|PRIVATE\|DSN\|AUTH\|COOKIE\|SESSION)/i`) | Test Cypher (task 04), env (task 07) |
| §2.3 (stdout tối đa 16 MiB; JSON 8 MiB; stderr lưu 64 KiB; timeout CLI 20 s, SIGTERM rồi SIGKILL sau `killGraceMs` 5000) | Test giới hạn (task 07) |
| §3.2 (`CODEINTEL_INVALID_PARAMS -32602`, `PATH_NOT_ALLOWED -33002`, `REPO_NOT_REGISTERED`, `OUTPUT_TOO_LARGE`, `TIMEOUT`, `TOOL_FAILED reason=write_blocked|unknown_shape`) | Mã lỗi khẳng định |
| §9.2 (phân giải repo: thứ tự hình dạng → `realpath` → allowed roots → `git rev-parse` → `git worktree list --porcelain` → khớp registry theo **đường dẫn chính xác** sau `realpath`; trùng → `indexedAt` mới nhất + cảnh báo; hỏng → `registry_unreadable`), §9.3 (agent không có "workspace root đăng ký"; `fs.*` nhận mọi đường dẫn), §9.4 (secret, canary), §7.4 (ca hiếm: `profile:"rm -rf /"`, `workspaceRoot:"../../etc"` → `-33002` **trước** `expandTilde`) | Task 05, 06, 07 |
| Proto/data-map: PQ-02 (vị trí mã lỗi), PQ-03 (tập mã), PQ-21 (`workspaceRoot` mọi method `quality.*`), PQ-37(c)(d) (`analyze` luôn `--index-only`; profile tự lọc env), §8.3 mục 5 (test phản chiếu schema `validate` không có `command\|argv\|args\|env\|cwd\|timeout`), O-16 | Task 03, 07 |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `agent/src/relay/agent-tool-registry.ts` (đầy đủ), `agent-config.ts` (`loadAgentConfig`, `toolEnv`), `context.ts` (`expandTilde`, `RelayContext.registerRoot` no-op và chú thích "FS-side allowlist removal"), `fs-agent-extensions.ts` (`handleFsReadFile`), `agent-rpc-dispatch-fs.ts` (`fs.readFile`), `agent-rpc-dispatch-misc.ts` (`tools/call`, `shell.*`), `agent-git-exec-validator.ts` + `.test.ts` (mẫu whitelist/cờ nguy hiểm), `agent-rpc-dispatch-misc.test.ts` (khuôn test dispatcher), `agent/package.json`, `agent/vitest.config.ts`, `deploy/agent/orca-agent.service`.

| Điểm | Hiện trạng thật | Hệ quả |
|---|---|---|
| Mã `codeintel.*`/`quality.*` | **Chưa có** (không có thư mục `codeintel/`; grep tên không có) | Test viết theo hợp đồng; tên module do AG-CV-SOL-001, 002, 004, 081 chốt. Mọi test import qua hằng đường dẫn ở đầu file |
| `runToolCommand` hiện có | `spawn(resolved, args, { cwd, env, stdio:['pipe','pipe','pipe'], shell:false })`; thu `stdout`/`stderr` **không giới hạn**; hết giờ thì `child.kill('SIGTERM')` rồi `resolve` ngay (không đợi, không SIGKILL, không giết cây) | Chỉ đúng một phần yêu cầu `TestSpawnNeverUsesShell`; **không** là mẫu cho `TestOutputCap`/`TestTimeoutKillsProcessTree` — runner mới của `codeintel.*` phải làm khác, test viết cho runner mới |
| Tool `gitnexus`/`codegraph` hiện có | `args` tự do (`params.args.map(String)`), `cwd` tự do, `env: config.toolEnv`, truy cập qua `tools/call` (`agent-rpc-dispatch-misc.ts`); không có whitelist subcommand | **Bề mặt cũ cho phép `gitnexus analyze`, `--repo=...` v.v.** qua `tools/call`, độc lập với `codeintel.*`. Ngoài phạm vi v7 nhưng làm thủng ranh giới D5 nếu có người gọi (mục 6, câu hỏi 1) |
| `shell`/`shell.eval`/`shell.exec` | Tool `shell` chạy `bash -c <command>`; `dispatchMiscRpc` có `shell.eval`/`shell.exec` (đọc phần đầu tệp) | Agent vốn là kênh thực thi lệnh tuỳ ý cho backend đã xác thực. "Ranh giới an toàn" của v7 là phòng thủ chiều sâu và giảm bề mặt cho đường **đọc** mới, không phải chống kẻ đã điều khiển được RPC; ghi trong mục 6 |
| `toolEnv` | `loadAgentConfig`: `{ ...process.env, PATH: toolPath, HOME, ANTHROPIC_API_KEY, GITHUB_TOKEN, GH_TOKEN }` — **toàn bộ `process.env`** | Xác nhận README v7 điểm 22 và hợp đồng §10 |
| Lọc đường dẫn của `fs.*` | `handleFsReadFile`: `isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)`, không kiểm root; `RelayContext.registerRoot` là no-op | Xác nhận CR-072 mục 1.3 và hợp đồng §9.3. `codeintel.*` phải **tự** chặn; v7 không đổi `fs.*` |
| `expandTilde` | `~`, `~/x`, `~\x` → `homedir()` | `workspaceRoot` phải bị từ chối **trước** khi gọi hàm này (§7.4) |
| Mẫu test whitelist | `agent-git-exec-validator.test.ts` dùng `expect(() => ...).not.toThrow()/toThrow()`; `agent-rpc-dispatch-misc.test.ts` dùng `vi.mock`, `MockWs`, `createWireState` | Khuôn test mới theo hai tệp này |
| CI | Không workflow nào chạy test `agent/` | Task 09 thêm job; dựa trên cấu hình vitest của AG-CV-SOL-070 |
| Bộ che secret trong `agent/` | Không có mô-đun che secret riêng cho relay (tìm tên `redact|mask` chỉ ra `preflight-handler.ts`, `agent-hook-server.ts`, `agent-ephemeral-vm-handler.ts`, `ssh-outbound-client.ts` và test; chưa đọc nội dung) | Bộ che stderr/thông điệp lỗi của `codeintel.*` là (mới) của AG-CV-SOL-001; test canary khẳng định nó |

**Correction relative to CR-CV-072:**

| # | CR viết | Hợp đồng / thực tế | Xử lý |
|---|---|---|---|
| 1 | `TestOutputCap`: "dừng đọc ở **8 MiB**, giết tiến trình, `CODEINTEL_OUTPUT_TOO_LARGE`" | Hợp đồng §2.3: **stdout 16 MiB** → kill + `OUTPUT_TOO_LARGE`; **JSON kết quả cuối 8 MiB** cắt dần mảng + `truncated:true`, vẫn vượt → `OUTPUT_TOO_LARGE` | Hai test riêng: stdout 20 MiB bị dừng ở 16 MiB; kết quả 9 MiB bị cắt/`OUTPUT_TOO_LARGE` |
| 2 | Whitelist CodeGraph = `query, callers, callees, impact, files, status, node, explore` (+ `analyze`/`init`/`index` chỉ reindex); GitNexus không nêu `check` | Hợp đồng §9.1: CodeGraph thêm `affected`; GitNexus thêm `check --cycles` **chỉ** cho `structuralFacts kind:"cycles"`; `node` **không công khai** (§4.15) | Test dùng danh sách của hợp đồng; `check` chỉ hợp lệ cùng cờ `--cycles` và chỉ từ đường `structuralFacts` |
| 3 | Danh sách lệnh cấm: `analyze, clean, remove, uninstall, publish, setup, serve, mcp, wiki, uninit, daemon, unlock, install` | Hợp đồng §2.4 thêm `index, init, sync, group, eval-server, telemetry, upgrade, check` | Dùng danh sách hợp đồng |
| 4 | Root "đã đăng ký" (`workspaceRoot` phải thuộc workspace root đăng ký) | Hợp đồng §9.3: agent **không có** root đăng ký; điều kiện thật = gốc git worktree + `ORCA_CODEINTEL_ALLOWED_ROOTS` (mặc định rỗng = không giới hạn) | Test theo hợp đồng; chạy thêm biến thể có `ALLOWED_ROOTS` |
| 5 | Env con: "qua bộ lọc `env` cho phép, không chuyển biến nhạy cảm của agent" (`TestSpawnNeverUsesShell`) | Hợp đồng §2.4: GitNexus/CodeGraph chạy với `config.toolEnv + NO_COLOR=1` (**toàn `process.env`**); chỉ **quality** có env bắt đầu từ rỗng. §10 lại nói "CR-072 phải xử lý" | **Mâu thuẫn trong hợp đồng.** Solution theo hướng chặt (test khẳng định env con của `codeintel.*` không có biến khớp mẫu nhạy cảm) và ghi đây là quyết định chờ duyệt (mục 7, câu hỏi 2) |
| 6 | Hàng đợi quá hạn `CODEINTEL_TIMEOUT` hoặc `CODEINTEL_RATE_LIMITED` | `RATE_LIMITED` không phải mã do agent sinh | Chỉ `TIMEOUT` |
| 7 | `TestWhitelistIsClosed` liệt kê "mọi hằng `argv[0]` trong module" | Hợp đồng §9.1 mô tả "lệnh là đối tượng có kiểu `GitNexusCommand`/`CodeGraphCommand`" | Test duyệt **kiểu** lệnh (bảng `COMMAND_BUILDERS`) thay vì quét văn bản, đồng thời một test quét văn bản mã nguồn các tệp `*.ts` runner để bắt `spawn(` ngoài một điểm duy nhất |

### 1.1 Lệch giữa CR và hợp đồng

Các dòng 1 đến 7 ở bảng trên. Thêm: CR-072 §2.4 yêu cầu bộ vector đường dẫn `path-attack-vectors.json` "đặt cạnh fixture của CR-CV-070 và đọc được từ cả TS lẫn Go"; hợp đồng không nêu nơi đặt. Solution đặt ở `agent/src/relay/codeintel/__fixtures__/path-attack-vectors.json` (theo CR) và báo cho BE-CV-SOL-072.

### 1.2 Phụ thuộc chéo khu vực

| Đối ứng | Quan hệ |
|---|---|
| `BE-CV-SOL-072-security-tests-service-gateway` | Dùng chung `path-attack-vectors.json` (Go đọc từ cây `agent/`); quy tắc "ba nơi cùng kiểm đường dẫn" (agent tự kiểm vì không tin backend). Backend sở hữu: RLS/tenant, ma trận quyền, red-team MCP, `FuzzDecodeCodeIntelArgs` — **không** ở đây |
| `BE-CV-SOL-070-...` | không liên quan trực tiếp; cùng workflow nếu gộp job (câu hỏi 4) |
| `AG-CV-SOL-001, 002, 004, 081` | sở hữu mã bị test; test phải đỏ khi ai đó thêm lệnh/cờ/tham số mới mà quên cập nhật |
| `AG-CV-SOL-070` | tái dùng `scanTextForLeaks` và mẫu token (một nguồn), cấu hình vitest, workflow |
| `FE-CV-SOL-*` | không có việc ở agent (XSS/Mermaid là của frontend) |

## 2. Giải pháp

### 2.1 Cây file

```
agent/src/relay/codeintel/
  spawn-recorder.ts                         (mới, chỉ dùng trong test) bắt argv/options/env của mọi spawn
  spawn-recorder.test.ts                    (mới)
  security-command-whitelist.test.ts        (mới) task 02
  security-params-and-schema.test.ts        (mới) task 03
  security-cypher.test.ts                   (mới) task 04
  security-workspace-root-and-paths.test.ts (mới) task 05
  security-repo-resolution.test.ts          (mới) task 06
  security-process-limits-and-env.test.ts   (mới) task 07 (stdout cap, timeout tree, env, canary stderr)
  security-seeded-fuzz.test.ts              (mới) task 08
  seeded-random.ts                          (mới, test-only) PRNG xác định (mulberry32), không phụ thuộc
  __fixtures__/path-attack-vectors.json     (mới) task 05
  __fixtures__/canary-secrets.json          (mới) task 07: chuỗi canary + dạng che mong đợi
.github/workflows/code-intel-security.yml   (mới hoặc job trong code-intel-contract.yml) task 09
```

### 2.2 Dụng cụ chung: `spawn-recorder.ts` (task 01)

Test-only, **không** nằm trong bundle `agent/out/agent.js` (không được import từ mã sản phẩm; `build.mjs` chỉ bundle từ `agent-entry.ts`, nên miễn là không có import ngược).

```ts
export type RecordedSpawn = { file: string; argv: readonly string[]; options: { shell?: unknown; cwd?: string; env: NodeJS.ProcessEnv; stdio?: unknown } }
export function installSpawnRecorder(vi: typeof import('vitest').vi, opts?: { stdout?: string; exitCode?: number }): {
  calls: RecordedSpawn[]            // chỉ ghi, không chạy tiến trình thật
  reset(): void
}
```
Dùng `vi.mock('node:child_process', ...)` cho `spawn`/`execFile`/`execFileSync`/`exec`, trả `FakeChild` theo khuôn `agent-print-mode-exec.test.ts` (đã đọc ở v6: `FakeChild`, `waitForSpawn()`). Với test cần tiến trình thật (stdout 20 MiB, treo, cây con), dùng CLI giả thật (tệp thực thi tạm), **không** dùng recorder.

Quét mã tĩnh `TestSpawnCallsOnlyInRunner`: đọc văn bản các tệp `agent/src/relay/codeintel/*.ts` (trừ `*.test.ts`, `spawn-recorder.ts`) và `codeintel-*.ts`, `gitnexus-*.ts`, `codegraph-*.ts`, `quality-*.ts`; khẳng định `child_process`/`spawn(`/`execFile(`/`exec(` chỉ xuất hiện ở danh sách tệp cho phép (runner, reindex commands, quality runner, git probe) và **không** có `shell: true`, `exec(`, `execSync(`, `spawnSync(` kèm chuỗi lệnh. Danh sách cho phép là hằng ở đầu test (thêm tệp spawn mới phải sửa test: ý định).

### 2.3 Whitelist, dấu `-`, cờ `-r`/`-p` (task 02)

Dựa vào hợp đồng §9.1 (không chép lại tên). Cách kiểm:

1. **Tập đóng:** import bảng kiểu lệnh (`GitNexusCommand`, `CodeGraphCommand`) từ module của AG-CV-SOL-001/002/003; với **mọi** giá trị hợp lệ do generator tạo từ tham số mẫu, gọi bộ dựng argv và ghi `argv[0]`. Khẳng định tập `argv[0]` == đúng tập hợp đồng (GitNexus: `query, context, impact, trace, cypher, list, status, detect-changes` + `check` chỉ cùng `--cycles`; CodeGraph: `query, callers, callees, impact, files, status, node, explore, affected`). Thêm lệnh mà quên test → đỏ.
2. **Cấm:** với từng method và tham số "xấu" (xem 2.4), bộ dựng không bao giờ ra subcommand thuộc danh sách cấm ngoài `codeintel-reindex-commands.ts`. Reindex: `gitnexus analyze` **luôn** có `--index-only` (không thiếu trong mọi tổ hợp `mode`/`workers`).
3. **`-`:** mọi giá trị người dùng đưa vào argv bị từ chối nếu (sau NFKC) bắt đầu bằng `-`; ca: `"-r"`, `"--repo=vnp-workplace"`, `"-rvnp-workplace"`, `"－r"` (U+FF0D), `"−r"` (U+2212), `" -r"` (khoảng trắng đầu — kiểm hành vi thật: từ chối hay trim, theo hợp đồng "không bắt đầu `-`"); và khi tham số hợp lệ được nhận thì đứng sau `--` nếu công cụ hỗ trợ (CR-072 mục 1.1: `gitnexus impact -r orca -- handleInvoke ...`; hành vi `--` với từng subcommand là **chưa kiểm chứng** cho mọi lệnh ngoài `impact`).
4. **Cờ repo:** `-r <registryPath>`/`-p <projectPath>` do agent đặt; không phần tử argv do người dùng quyết định nằm ngoài vị trí sau `--`; `--repo=x`, `-rx` bị từ chối **trước spawn** (`calls.length === 0`).
5. **Shell:** mọi `spawn` có `options.shell` là `false` hoặc `undefined` (không `true`, không chuỗi).

### 2.4 Tham số nghiêm ngặt và phản chiếu schema (task 03)

- `TestParamsStrict`: cho mỗi method trong `CODEINTEL_METHODS` và `QUALITY_METHODS` (hằng của 001/081), gửi `{workspaceRoot: <hợp lệ>, <khoá lạ>: 1}` với khoá ∈ {`args, argv, command, cmd, cwd, env, repo, cypher, shell, timeout, tool`, `extra`} → `-32602 CODEINTEL_INVALID_PARAMS` và `data.field` bằng khoá đó; **`_trace`** là ngoại lệ duy nhất (§2.1); kiểu sai (mảng thay chuỗi, đối tượng lồng, số thay chuỗi), thiếu `workspaceRoot`, chuỗi 513 ký tự, có NUL/điều khiển.
- **Phản chiếu schema** (CR-072 mục 2.2 / §8.3 mục 5): mỗi method có `validate` (schema) công khai trong bảng method; test duyệt `Object.keys` của schema và khẳng định không có khoá cấm. Nếu AG-CV-SOL-001 không xuất schema duyệt được thì test nhờ hợp đồng buộc: yêu cầu 001 xuất `CODEINTEL_METHOD_SCHEMAS` (câu hỏi 3).
- Hàm bộ đếm: `Object.keys(CODEINTEL_METHODS)` phải khớp danh sách §4–5 của hợp đồng (16 + 6 method; `codeintel.node` **không** có) — chặn việc ai đó thêm method công khai không qua hợp đồng.
- Không method nào trả về `args`/`argv`/`env`/`cwd` trong `data` hay `error.data`.

### 2.5 Cypher (task 04)

Mẫu hằng + bốn bộ mã hoá + `assertReadOnlyCypher` (hợp đồng §2.4). Test:

| Test | Nội dung |
|---|---|
| `every template passes assertReadOnlyCypher` | duyệt toàn bộ mẫu (hằng của 002); mỗi mẫu bắt đầu `MATCH `, một câu, ≤ 16 KiB, không từ cấm (so khớp theo từ, không phân biệt hoa thường, kể cả trong chuỗi mẫu cố định) |
| `forbidden words are rejected even when produced by data` | giá trị người dùng chứa `DETACH DELETE`, `CALL show_tables()`, `LOAD`, `ATTACH`: hoặc bị bộ kiểm hạng tử từ chối, hoặc nằm trong chuỗi **đã thoát** và `assertReadOnlyCypher` chạy trên **chuỗi cuối** vẫn đạt/không đạt đúng (không có từ cấm ngoài literal) — chốt cách xử lý từ cấm nằm *bên trong literal* (câu hỏi 5) |
| `param validators` | `processId` theo ngữ pháp thật (quan sát ở fixture AG-CV-SOL-070); `uid`/`center` theo `SymbolRef.key`; `kinds` ∈ enum hợp đồng §2.6; `limit`/`depth` nguyên có chặn; mọi giá trị khác bị từ chối **trước** nội suy |
| `string escape` | `'`, `"`, `\`, `}`, `)`, `//`, `/*`, xuống dòng, NUL, `' OR 1=1 //`, `'}) MATCH (x) DETACH DELETE x //`: bị từ chối hoặc thoát đúng; chuỗi sinh ra khi chuẩn hoá (thay literal bằng `?`) **bằng** mẫu gốc |
| `single statement` | không `;` ngoài literal; không thêm `MATCH ... RETURN` ngoài mẫu |
| `limit always set` | mọi lệnh `cypher` có `-l <n>` và `LIMIT` trong mẫu |
| `slot substitution is closed` | mọi `{{ten}}` trong mẫu đều có bộ mã hoá khai báo; không còn `{{` trong chuỗi cuối |
| live (đêm, tuỳ chọn, không chạy ở PR) | gửi ca độc tới `gitnexus cypher` thật trên `mini-repo` (chỉ-đọc) và khẳng định `{error}`/`[]` — dùng hạ tầng tầng-live của AG-CV-SOL-070 |

### 2.6 `workspaceRoot` và đường dẫn (task 05)

`path-attack-vectors.json`: `[{ "input": "...", "field": "workspaceRoot"|"file"|"path", "expect": "allow"|"deny", "code": "CODEINTEL_PATH_NOT_ALLOWED"|"CODEINTEL_INVALID_PARAMS", "note": "..." }]`. Ca (CR-072 §2.4, theo hợp đồng):

- `workspaceRoot`: `..`, `../x`, `/`, `/etc`, `~`, `~/.ssh`, tương đối, `""`, chỉ khoảng trắng, NUL, > 4096 ký tự, UTF-8 hỏng → `PATH_NOT_ALLOWED`/`INVALID_PARAMS`; `"../../etc"` → `-33002` và `expandTilde`/`realpath` **không được gọi** (test dùng spy; thứ tự "hình dạng trước, `realpath` sau").
- Cần thư mục tạm: root là symlink ra ngoài; thư mục con là symlink (`repo/link -> /etc`) rồi `file: link/passwd`; root có `/` cuối, `//`; so sánh với root đi kèm `ORCA_CODEINTEL_ALLOWED_ROOTS` (`/tmp/x/orca` so với `/tmp/x/orca-old`: không khớp tiền tố).
- `file`/`path`: `a/../../x`, `a/./../..`, `%2e%2e`, `%252e%252e`, `..%c0%af`, `．．`/`․․` (NFKC), `\` trên POSIX, tuyệt đối, NUL, > 1024 → từ chối. Hành vi của `%2e%2e` là chuỗi literal trên hệ tệp POSIX (không phải `..`); quyết định "từ chối vì nghi ngờ" hay "cho phép như tên tệp" theo hợp đồng "không `..`, không tuyệt đối, không `\`": **từ chối** mọi dạng mã hoá để không phụ thuộc diễn giải (D4).
- `.env`, `*.pem`, `*.key`, `id_rsa*`: không bao giờ trả nội dung (§9.4) — kiểm ở `codeintel.symbol` khi symbol thuộc tệp như vậy (kết hợp canary, task 07).
- Windows (`C:\`, UNC, `\\?\C:\`): hợp đồng trả `unsupported_platform`; test chỉ khẳng định từ chối (`TOOL_UNAVAILABLE`/`PATH_NOT_ALLOWED`), không có dev server Windows để kiểm thật.
- TOCTOU: giảm thiểu bằng dùng đường dẫn đã `realpath`; test chỉ khẳng định đường dẫn truyền cho `spawn` là bản `realpath`; rủi ro còn lại ghi nhận (không kiểm tự động).

Tệp JSON này cũng được BE-CV-SOL-072 đọc: giữ `input` ở dạng chuỗi JSON thuần (không biểu diễn đặc thù TS), dùng `\u` cho ký tự điều khiển.

### 2.7 Phân giải repo (task 06)

Dùng thư mục tạm + registry giả (`HOME` trỏ thư mục tạm có `.gitnexus/registry.json`; agent đọc `~/.gitnexus/registry.json`, §2.4/§9.2) và `git init` thật hoặc `git` giả. Ca:

| Ca | Kỳ vọng |
|---|---|
| Hai mục registry cùng đường dẫn (hoặc cùng tên) | hợp đồng §9.2: "chọn `indexedAt` mới nhất + cảnh báo" — **khác CR-072** ("từ chối mơ hồ"); test theo hợp đồng và khẳng định `warnings` có mã tương ứng; CR-072 lệch → ghi (câu hỏi 6) |
| `workspaceRoot` là thư mục con của repo đã đăng ký (worktree lồng, `/…/orca/.claude/worktrees/x`) có `git rev-parse --show-toplevel` = chính nó | **không** dùng chỉ mục repo cha; không có mục registry khớp chính xác → `INDEX_MISSING`/`REPO_NOT_REGISTERED`, không dữ liệu repo cha |
| Tên gần giống (`orca` / `orca-old`, hoa thường) | khớp theo đường dẫn, không theo tên |
| `-r` lấy từ registry đã phân tích, không từ tham số | `argv` có `-r <realpath>`; không có tham số RPC nào xuất hiện trong `-r` |
| Registry hỏng/không đọc được | `TOOL_FAILED reason="registry_unreadable"` |
| Registry đổi giữa hai lần gọi | `REPO_NOT_REGISTERED`; cache 30 s theo `workspaceRoot` bị huỷ khi phát hiện (hợp đồng §2.3 chỉ nói huỷ khi `indexChanged`/reindex; hành vi khi **registry** đổi là chưa chốt, câu hỏi 7) |
| `codegraph`: `-p` đã `realpath`; symlink trỏ repo khác | so khớp sau `realpath` |
| Lỗi thiếu `-r` của công cụ (stack trace chứa `Available: orca, vnp...`) | stderr **không bao giờ** chuyển nguyên lên backend (`TestToolStderrNeverForwarded`): `stderrTail` đã che, ≤ 2 KiB, loại `HOME`; không chứa tên repo khác |

### 2.8 Giới hạn tiến trình, env và canary (task 07)

CLI giả thật (không recorder):

- `stdout 20 MiB`: agent dừng ở 16 MiB, kill, `CODEINTEL_OUTPUT_TOO_LARGE` (`data.bytes`, `data.limit`); RSS tiến trình test không tăng quá một ngưỡng rộng (giả định, ví dụ +96 MiB; chưa đo).
- `kết quả JSON > 8 MiB`: cắt dần mảng + `truncated:true`; vẫn vượt → `OUTPUT_TOO_LARGE`.
- Treo: SIGTERM, rồi SIGKILL sau `killGraceMs`, kể cả **tiến trình cháu** (CLI giả sinh một con `sleep` trong cùng nhóm tiến trình): sau hạn, không còn pid nào sống (đối chiếu bằng `process.kill(pid, 0)`); `CODEINTEL_TIMEOUT`. Yêu cầu runner đặt `detached: true` + giết theo nhóm (`process.kill(-pid)`) trên POSIX — **chưa kiểm chứng** hành vi của `gitnexus` (có dùng worker con?) nên test dùng CLI giả.
- Env con: tiến trình giả in `process.env`; test đặt `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`, `GH_TOKEN`, `AGENT_TOKEN`, `ORCA_URL`, `SSH_AUTH_SOCK`, `AWS_SECRET_ACCESS_KEY`, `MY_SECRET_THING` vào `process.env` của test; khẳng định **không** có trong env con của `codeintel.*` (hướng chặt, mục "Correction" 5) và cũng không có trong env con của `quality.*` (hợp đồng §2.4, §9.5), kể cả khi profile xin `env.allowExtra`. Cho phép: `PATH`, `HOME`, `NO_COLOR=1`; còn lại theo whitelist của 001/081.
- Canary: tệp cấu hình trong `mini-repo`/thư mục tạm chứa `CANARY-DB-PASS-1`, `CANARY-REDIS-2`, `CANARY-API-3`, khối `-----BEGIN PRIVATE KEY-----` giả, `ghp_`/`AKIA` giả, `https://user:CANARY-URL-4@host`; CLI giả làm `symbol`/lỗi in các chuỗi này ra stdout/stderr; khẳng định `stderrTail`, `message`, log (handler log bắt), `error.data` **không** chứa canary, đường dẫn tuyệt đối ngoài `workspaceRoot`, hay `$HOME`; kết quả `symbol` từ tệp nhạy cảm (`.env`, `*.pem`...) bị từ chối/che. Che theo regex không bắt mọi dạng: giới hạn đã biết, ghi trong tài liệu (CR-072 §2.6).

### 2.9 Fuzz xác định (task 08)

Không thêm `fast-check` (D2 / O12 chưa duyệt): `seeded-random.ts` (mulberry32) với hạt giống cố định (số hạt trong tên test; in hạt khi thất bại). Bất biến cho từng bộ dựng/validator: **hoặc** từ chối với mã có kiểu, **hoặc** argv/Cypher/đường dẫn thoả ngữ pháp cho phép; không bao giờ ném ngoại lệ không bắt. Đầu vào: Unicode ngẫu nhiên, NUL, chuỗi dài, khoảng trắng, `;`, `$()`, backtick, xuống dòng, các ký tự U+FF0D/U+2212, chuỗi bắt đầu bằng `-` sau chuẩn hoá. Số vòng mặc định đặt để tổng ≤ vài giây (ước đoán; chưa đo); biến `ORCA_FUZZ_ITERATIONS` tăng lên khi chạy tay.

### 2.10 CI (task 09)

`code-intel-security` (workflow riêng hoặc job thứ hai trong `code-intel-contract.yml`; cùng cấu hình vitest `vitest.code-intel-contract.config.ts` của AG-CV-SOL-070, mở rộng `include` bằng `security-*.test.ts`): chặn PR. Job live (đêm) chạy ca Cypher độc trên `gitnexus` thật — thừa hưởng hạ tầng của 070. Thêm bước khẳng định số test security ≥ ngưỡng (đếm `vitest list`), tránh "xanh vì không chạy".

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Từ chối thay vì "làm sạch" đầu vào | CR-072 D3 |
| D2 | Không thêm phụ thuộc; fuzz bằng PRNG xác định | O12; CR-072 câu hỏi 1 |
| D3 | Test theo hợp đồng khi lệch CR (bảng Correction) | Hợp đồng thắng |
| D4 | Từ chối mọi dạng mã hoá đường dẫn thay vì giải mã | Cái ta cấp phép phải đúng từng byte cái ta chạy |
| D5 | Env con `codeintel.*` bị lọc (chặt) | README v7 điểm 22 + hợp đồng §10; chờ duyệt vì §2.4 nói `toolEnv` |
| D6 | Test cho runner mới, không dựa vào `runToolCommand` cũ | `runToolCommand` không có trần dung lượng, không SIGKILL, không giết cây |
| D7 | Bộ vector đường dẫn dùng chung với BE | CR-072 D4 |
| D8 | Tầng live (đêm) cho ca Cypher độc trên công cụ thật; PR chỉ dùng stub | Công cụ ngoài không ổn định (như 070 D2) |
| D9 | Không sửa `fs.*` ở v7; chỉ ghi nhận rủi ro | Ngoài phạm vi; `fs.*` đã cố ý bỏ allowlist (chú thích trong `context.ts`) |

## 4. Tiêu chí chấp nhận

- [ ] `TestWhitelistIsClosed`, `TestNoForbiddenSubcommand`, `TestUserValuesNeverStartWithDash`, `TestRepoFlagIsLast`, `TestReindexArgvIsIndexOnly`, `TestSpawnNeverUsesShell`, `TestParamsStrict` xanh; `--repo=vnp-workplace`, `-rvnp-workplace`, U+FF0D, U+2212 bị từ chối **trước** spawn (`calls.length === 0`).
- [ ] Test phản chiếu schema: không method nào nhận `args|argv|command|cmd|cwd|env|repo|cypher|shell|timeout|tool`; số method công khai = hợp đồng; `codeintel.node` không có.
- [ ] Mọi mẫu Cypher chỉ-đọc; vector tiêm bị từ chối hoặc thoát đúng; `-l` luôn có.
- [ ] Bộ vector `path-attack-vectors.json` xanh ở agent (cùng tệp BE đọc); symlink thoát root bị từ chối sau `realpath`; `~` từ chối trước `expandTilde`.
- [ ] Worktree lồng không nhận dữ liệu repo cha; registry trùng → cảnh báo + `indexedAt` mới nhất (theo hợp đồng); stderr thô không bao giờ ra ngoài.
- [ ] stdout 20 MiB bị dừng ở 16 MiB; treo bị giết đúng hạn, không để cháu sống; kết quả > 8 MiB xử lý đúng.
- [ ] Env con không chứa biến nhạy cảm; quét canary sạch (`stderrTail`, `message`, `error.data`, log).
- [ ] Fuzz xác định: bất biến giữ cho mọi hạt cố định; thất bại in hạt.
- [ ] Workflow chặn PR chạy các test trên; có bước khẳng định số test.
- [ ] Không thêm phụ thuộc, không `max-lines` disable.

## 5. Kiểm thử

Chính solution này là bộ kiểm thử; mỗi task liệt kê tên test. Lệnh: trong `/opt/repos/orca/agent`, `pnpm exec vitest run src/relay/codeintel/security-<tên>.test.ts`; toàn bộ: `pnpm exec vitest run --config vitest.code-intel-contract.config.ts` (cấu hình do AG-CV-SOL-070). Chưa chạy.

## 6. Rủi ro và điểm chưa kiểm chứng

- Test phụ thuộc mã chưa viết (001, 002, 003, 004, 081); khi tên/chữ ký khác, chỉ đổi dòng import (đặt hằng đường dẫn ở đầu tệp).
- **Bề mặt cũ không bị chặn:** `tools/call name:"gitnexus" args:["analyze"]`, `tools/call name:"shell"`, `shell.eval/exec`, `fs.readFile` tuyệt đối. Nếu backend hoặc client MCP gọi chúng, mọi bảo đảm của v7 bị bỏ qua. Cần quyết định có tắt/giới hạn `gitnexus`/`codegraph` ở `tools/call` khi `codeintel` được bật không (câu hỏi 1).
- Hành vi `--` (kết thúc tuỳ chọn) với từng subcommand của GitNexus/CodeGraph chưa kiểm chứng ngoài `impact` (CR-072 mục 1.1).
- Chưa kiểm chứng GitNexus/CodeGraph có chuẩn hoá Unicode (U+FF0D → `-`) hay không; test dựa vào từ chối ký tự ngoài tập cho phép.
- Giết cây tiến trình ở Windows/WSL/macOS khác Linux (nhóm tiến trình); Windows không hỗ trợ ở v7.
- TOCTOU thực tế chưa kiểm; chỉ giảm thiểu.
- Che secret theo regex không phủ mọi dạng; chỉ mục của công cụ có thể đã chứa secret trong `content` (ngoài kiểm soát).
- Thời gian chạy test (fuzz, stdout 20 MiB, treo + SIGKILL 5 s) chưa đo; có thể làm chậm job; tách `*.slow.test.ts` nếu cần.
- SSH (AGENTS.md): hệ tệp xa (symlink, mount) khác máy dev; chạy `--stdio` dùng cùng mã Part A nhưng chưa kiểm riêng.

## 7. Câu hỏi mở và điểm hợp đồng thiếu hoặc mâu thuẫn

1. Có chặn/giới hạn tool `gitnexus`/`codegraph` hiện có ở `tools/call` khi `codeintel` bật (ngoài hợp đồng v7)? Hiện không có điều khoản nào.
2. **Mâu thuẫn hợp đồng:** §2.4 nói env con GitNexus/CodeGraph = `config.toolEnv + NO_COLOR=1` (toàn `process.env`), §10 nói "CR-072 phải xử lý" và README v7 điểm 22 nói tool hiện có cũng bị ảnh hưởng. Cần quyết định: lọc env cho `codeintel.*` (solution mặc định) hay giữ `toolEnv`.
3. **Hợp đồng thiếu:** nơi xuất bảng method + schema `validate` công khai (cần cho test phản chiếu) và hằng `CODEINTEL_METHODS`/`QUALITY_METHODS` (hợp đồng §8 nhắc `CODEINTEL_METHODS` nhưng không `QUALITY_METHODS`).
4. Job bảo mật có chung workflow với job hợp đồng (070) hay tệp riêng? Mặc định: hai job trong cùng tệp nếu BE-CV-SOL-070 đồng ý; nếu không, tệp riêng.
5. Từ cấm Cypher nằm **bên trong literal** (ví dụ symbol tên `SET`): bị từ chối hay cho phép khi đã thoát? Hợp đồng `assertReadOnlyCypher` "cấm" theo từ; nếu áp trên chuỗi cuối kể cả literal thì symbol tên `Set`/`Import` (thực tế phổ biến) sẽ bị chặn nhầm. Cần chốt.
6. Registry trùng: hợp đồng ("chọn `indexedAt` mới nhất + cảnh báo") khác CR-072 ("từ chối mơ hồ"). Theo hợp đồng; xác nhận.
7. Registry thay đổi giữa hai lần gọi: huỷ cache repo 30 s thế nào (theo `mtime` của `registry.json`?). Hợp đồng chưa nói.
8. `ORCA_FUZZ_ITERATIONS`, `ORCA_UPDATE_GOLDEN` là biến test mới; chấp nhận không?
9. Ai sở hữu việc chặn `fs.*` ngoài root ở agent (CR-072 Q4)? Ngoài v7.
