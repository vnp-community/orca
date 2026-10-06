# AG-CV-SOL-070: Fixture vàng, kiểm thử parser theo phiên bản công cụ và job CI cho `agent/`

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mọi dòng "đã đọc" là đọc code/tài liệu thật; chưa chạy build, test, `gitnexus`, `codegraph` nào. Mọi hình dạng đầu ra công cụ lấy từ CR-CV-070 (chạy thử chỉ-đọc ngày 2026-10-05 bởi người soạn CR) và **chưa được chạy lại** ở đây.

**CR:** [CR-CV-070](../../../../../../docs/crs/v7/quality-rollout/CR-CV-070-golden-fixtures-and-tool-contract-tests.md) (P0, Medium)
**Service:** `agent/` (gói `orca-agent`, TypeScript, vitest)
**Hợp đồng chuẩn tắc:** [`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md) (chính), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md), [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md)
**TDD tham chiếu:** [v5/00-index](../../../../tdd/v5/00-index.md), [v5/05-tool-registry](../../../../tdd/v5/05-tool-registry.md), [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md), [v5/08-deployment](../../../../tdd/v5/08-deployment.md)
**Mẫu định dạng:** `specs/agent/crs/v6/agent-capabilities/solutions/`

## 0. Hợp đồng áp dụng

| Mục hợp đồng | Áp dụng thế nào |
|---|---|
| Agent contract §0 (bảng bằng chứng), §1.3 (dải phiên bản, marker lược đồ, `supported:false`, `unsupported_version`) | `SUPPORTED_TOOL_VERSIONS`, marker, phân loại `verified/untested/incompatible` (task 07) |
| Agent contract §2.2 (phong bì kết quả: `sources[]`, `headCommit`, `stale`, `truncated`, `totalCount`, `warnings`, `perf`, `data`), §2.6 (`SymbolRef`, quy tắc khoá, ánh xạ kind) | Hình dạng của tệp vàng C2 (task 08); `perf`, `startedAt` **không** nằm trong so sánh vàng (§8 hợp đồng: "trừ `perf`, `startedAt`") |
| Agent contract §3.2 (`CODEINTEL_TOOL_FAILED reason=format_drift|unknown_shape|write_blocked|truncated_stdout|registry_unreadable`, `CODEINTEL_REPO_NOT_REGISTERED`, `CODEINTEL_TOOL_UNAVAILABLE reason=unsupported_version|schema_version_unsupported`) | Mã lỗi mà test parser khẳng định (task 04 đến 07). `reason` (không phải `detail` như CR viết) |
| Agent contract §2.4 (stdout GitNexus qua tệp tạm vì pipe cắt cụt; `cypher` không có tham số) | Fixture chụp theo cùng đường tệp tạm; test không tin mã thoát |
| Agent contract §9 (an toàn), §10 (CI không chạy test `agent/`), §11 (bảng "Việc phải làm ở `agent/`": fixture vàng + `agent/scripts/capture-codeintel-fixtures.mjs` + job CI `code-intel-contract`) | Cây fixture, script chụp, workflow |
| Proto/data-map: PQ-20 (agent chuẩn hoá `SymbolRef`, `lineBase=1`, vector kiểm thử dùng chung), PQ-03 (tập mã lỗi), PQ-37(c) (`analyze` luôn `--index-only`), §7.1 cổng G1 (tệp vàng agent mở khoá backend collector và FE fake backend), §8.3 (kiểm chéo) | Task 08 sinh tệp vàng C2; task 03 chụp với `--index-only` |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `agent/package.json`, `agent/vitest.config.ts`, `agent/tsconfig.json`, `agent/build.mjs`, `agent/src/relay/agent-tool-registry.ts`, `agent-config.ts`, `agent-session-capabilities.ts`, `agent-rpc-dispatch.ts` (đầu file, `extractTraceFields`, danh sách `dispatchXxxRpc`), `agent-rpc-dispatch-misc.ts` + `agent-rpc-dispatch-misc.test.ts` (khuôn test dispatcher), `agent-git-exec-validator.ts` + `.test.ts`, `.github/workflows/` (danh sách), `.github/workflows/pr.yml` (dòng 1-40, 100-145), `package.json` gốc (scripts `test`, `lint`, `engines`), `pnpm-workspace.yaml`, `deploy/agent/*`.

| Điểm | Hiện trạng thật | Hệ quả |
|---|---|---|
| Thư mục `agent/src/relay/codeintel/`, mọi `codeintel-*.ts`, `gitnexus-*.ts`, `codegraph-*.ts` | **Chưa tồn tại** (`ls agent/src/relay/codeintel` lỗi; `ls agent/scripts` lỗi) | Mọi tên module của AG-CV-SOL-001/002/003/005 trong solution này là "theo hợp đồng §11, tên cuối do solution đó chốt"; test import qua một hằng đường dẫn ở đầu file để sửa một chỗ |
| Quy ước fixture | Repo dùng `__fixtures__` ở TS (`desktop/src/main/startup/__fixtures__`), `testdata` ở Go (CR-070 mục 1.3, tôi chưa tự kiểm lại danh sách này) | Fixture agent đặt ở `__fixtures__`; tệp vàng C2 đặt ở `backend-go/services/code-intel-service/testdata/agent-results/` |
| vitest của agent | `agent/vitest.config.ts`: `environment: 'node'`, `include: ['src/**/*.test.ts']`; `package.json` có `"test": "vitest run"`; tên gói `orca-agent`; `pnpm-workspace.yaml` liệt kê `agent` | Test colocated trong `src/` chạy được bằng `pnpm --filter orca-agent exec vitest run <đường dẫn>`. Chưa chạy lần nào trong đợt soạn |
| CI | `.github/workflows/` có 18 workflow `backend-go-*`, `computer-e2e.yml`, `e2e.yml`, `pr.yml`, `release-*`, `terminal-perf.yml`... `grep -rn "agent/" .github/workflows` **không có kết quả**; `pr.yml` bước Test là `pnpm test` ở gốc; script `test` gốc là `ensure-native-runtime ... && vitest run --config config/vitest.config.ts`; `config/` ở gốc **không có** `vitest.config.ts` (chỉ `max-lines-baseline.txt`, `oxlint-react-doctor.json`, `patches`, `scripts`) | Xác nhận lại CR-070 mục 1.4: **test `agent/` không chạy trong CI nào**. Cần workflow riêng (task 09). Không sửa `pr.yml` trong solution này (ngoài phạm vi; câu hỏi mở 3) |
| `build.mjs` | `import { build } from 'esbuild'` (esbuild có sẵn, không cần thêm phụ thuộc), bundle `agent-entry.ts` thành `out/agent.js` (CJS, target node22); không có script chạy TypeScript trực tiếp (`ls node_modules/.bin` trong `agent/` không có `tsx`; chỉ có `vitest`, `jiti`, ...) | Script `.mjs` cần chạy hằng số `argv` bằng TS: dùng esbuild bundle một entry tạm (task 03). `jiti` có trong `.bin` nhưng không phải phụ thuộc khai báo của `agent/` (chưa kiểm), không dùng |
| `tsconfig.json` | `include` gồm `./src/relay/**/*` (kể cả `*.test.ts`, trừ `integration.test.ts`); `rootDir: ./src` | Test `.ts` mới phải qua `tsc`; tránh import `.mjs` từ test |
| Node | `engines.node = "24"` ở gốc; máy soạn `node v22.23.2` | `node:sqlite`, type stripping không dùng ở đây |
| Công cụ đã có trong registry của agent | `agent-tool-registry.ts` có tool `gitnexus` và `codegraph` với `args` tự do, `timeout 60_000`, `env: config.toolEnv` (không phải đường `codeintel.*`) | Fixture/parsers chỉ phục vụ `codeintel.*`; không thay đổi tool cũ |

**Correction relative to CR-CV-070:**

| # | CR viết | Thực tế / hợp đồng | Xử lý trong solution |
|---|---|---|---|
| 1 | Mã lỗi kèm `detail: "schema_version_unsupported"` / `detail: "format_drift"` | Hợp đồng §3.2 dùng `data.reason` | Dùng `reason` |
| 2 | `codeintel.status` trả `compatibility ∈ verified|untested|incompatible` | Hợp đồng §4.1 chỉ có `tools.<tool>.supported` (boolean), `indexes.codegraph.reindexRecommended`; **không** có `compatibility` | Nội bộ giữ ba trạng thái; ánh xạ `supported = state !== 'incompatible'`; `untested` thêm `warnings:["tool_version_untested"]` vào phong bì; đề nghị bổ sung hợp đồng (mục 7, câu hỏi 1) |
| 3 | `status.warnings[]` có `tool_version_untested`, `index_built_with_old_extraction` | Hợp đồng §2.2 có `warnings` ở **phong bì**, mã ổn định; hai chuỗi này chưa có trong hợp đồng | Dùng đúng hai chuỗi, ghi vào danh sách đề nghị bổ sung |
| 4 | Whitelist CodeGraph không có `affected` và không có `check --cycles` của GitNexus | Hợp đồng §9.1: GitNexus = `query, context, impact, trace, cypher, list, status, detect-changes` + `check --cycles`; CodeGraph = `query, callers, callees, impact, files, status, node, explore, affected` | Fixture dựng theo hợp đồng; `codegraph affected` nằm trong danh sách ứng viên chụp (task 03) nhưng chỉ chụp khi parser của CR-003 dùng nó |
| 5 | Vị trí code: `agent/src/relay/codeintel/` (đề xuất) | Hợp đồng §11 ghi `agent-rpc-dispatch-codeintel.ts` + `codeintel-*.ts` (phẳng ở `agent/src/relay/`) nhưng đặt fixture ở `agent/src/relay/codeintel/__fixtures__/` | Fixture và mô-đun mới của solution này ở `agent/src/relay/codeintel/`; đường dẫn lọc CI dùng `agent/**` để không lọt tệp phẳng (task 09). Mâu thuẫn trong hợp đồng, xem mục 7 |
| 6 | Job nằm trong `code-intel-contract.yml` gồm cả `go test` | Hợp đồng §11 gán job `code-intel-contract` cho `agent/`; phần Go thuộc BE-CV-SOL-070 | Solution này tạo/điền **job agent**; job Go do BE-CV-SOL-070 thêm vào cùng tệp (điểm phối hợp, mục 0 của task 09) |
| 7 | Script `agent/scripts/capture-codeintel-fixtures.mjs` "dùng cùng hằng số argv mà parser dùng" | Agent không có runner TypeScript; hằng số argv là TS | esbuild bundle entry tạm trong script (task 03) |

### 1.1 Lệch giữa CR và hợp đồng (ngoài bảng trên)

- CR-070 §2.6 phân loại `untested` là "cùng major, minor/patch lạ"; hợp đồng §1.3 định nghĩa **dải** `>=1.6.0 <2` (GitNexus), `>=1.4.0 <2` (CodeGraph) là `supported` và ghi "đã thử" 1.6.9/1.4.1. Hai cách nói khớp nhau nếu: `verified` = phiên bản ∈ `SUPPORTED_TOOL_VERSIONS` (đã có fixture); `untested` = trong dải nhưng không có fixture; `incompatible` = ngoài dải hoặc marker lược đồ lạ (`meta.json.schemaVersion ∉ {5}`; `extractionVersion ∉ {24}`; `schema_versions` ngoài `[1,8]`). Solution theo cách hiểu này.
- Fixture trong CR là "một thư mục mỗi phiên bản"; hợp đồng không quy định thêm.

### 1.2 Phụ thuộc chéo khu vực

| Đối ứng | Quan hệ |
|---|---|
| `BE-CV-SOL-070-collector-golden-contract` (backend-go) | **Đọc** các tệp `backend-go/services/code-intel-service/testdata/agent-results/*.json` mà task 08 sinh; cổng G1 (§7.1 hợp đồng proto): backend không cần dev server thật. Hai bên phải thống nhất thứ tự khoá và tên tệp (bảng 2.5) |
| `BE-CV-SOL-070-...` job Go | cùng tệp workflow `.github/workflows/code-intel-contract.yml` với task 09 |
| `FE-CV-SOL-073-flag-gating-and-web-e2e` | `code-intel-fake-backend.ts` dựng dữ liệu từ cùng tệp vàng C2 (CR-073 mục 2.x); không có việc ở agent |
| `BE-CV-SOL-071-metrics-tracing-and-budgets` | dùng script chụp chế độ `--bench` (task 03 chỉ dành chỗ, việc thật ở AG-CV-SOL-071 task 06) |
| `AG-CV-SOL-001/002/003/005` (cùng khu vực, ngoài phân công) | cung cấp parser, whitelist, `buildCodeIntelResult`, `SUPPORTED_*`; solution này viết test và fixture cho chúng |

## 2. Giải pháp

### 2.1 Cây file

```
agent/src/relay/codeintel/
  __fixtures__/
    mini-repo/                                 (mới) 15-25 tệp mã nguồn mẫu, KHÔNG có .gitnexus/.codegraph
    gitnexus/1.6.9/MANIFEST.json + tệp thô     (mới) danh sách ở 2.3
    codegraph/1.4.1/MANIFEST.json + tệp thô    (mới)
    path-attack-vectors.json                   (mới, owner AG-CV-SOL-072 task 05; chỉ nhắc ở đây)
  fixture-manifest.ts                          (mới) schema MANIFEST, loader, ngân sách, quét rò rỉ
  fixture-manifest.test.ts                     (mới) TestFixtureBudget, TestFixtureHasNoLeaks, TestManifestHashes
  fixture-capture-plan.ts                      (mới) danh sách lệnh chụp dựng từ hằng số argv của whitelist
  gitnexus-cypher-output.golden.test.ts        (mới)
  gitnexus-json-commands.golden.test.ts        (mới) context, impact, query
  gitnexus-text-commands.golden.test.ts        (mới) detect-changes, status/list (văn bản)
  codegraph-output.golden.test.ts              (mới) JSON, ANSI "not found", sqlite schema, lỗi quy trình
  tool-compatibility.test.ts                   (mới) bảng 2.6 + TestEverySupportedVersionHasFixtures
  agent-result-golden.test.ts                  (mới) C2: so với/ghi testdata của backend
agent/scripts/
  capture-codeintel-fixtures.mjs               (mới) chạy tay, không trong CI
  esbuild-run-typescript-entry.mjs             (mới) bundle một entry .ts vào thư mục tạm rồi import
agent/vitest.code-intel-contract.config.ts     (mới) `include` đúng các test hợp đồng cho job CI
.github/workflows/code-intel-contract.yml      (mới) job `code-intel-contract` (agent) + `code-intel-live-contract` (đêm)
backend-go/services/code-intel-service/testdata/agent-results/*.json   (mới, do task 08 sinh; BE đọc)
```

### 2.2 Repo mẫu `mini-repo/` (task 01)

15 đến 25 tệp, **chỉ mã nguồn** (CR-070 §2.2): Go hexagonal (`internal/{domain,usecase,adapter}`) + một `.proto` + một `migrations/*.sql`; TypeScript (một component, một hàm gọi kênh); một route HTTP; **hai symbol trùng tên** ở hai tệp (để có `ambiguous` và va chạm `SymbolRef.key`, PQ-20); một hàm có chuỗi chứa `|`, xuống dòng, dấu nháy; tên có Unicode (NFC/NFD); tệp rỗng; tên tệp có khoảng trắng; dòng rất dài. `.gitignore` của `mini-repo/` không cần, vì không bao giờ có `.gitnexus/` hay `.codegraph/` trong cây git (script chụp dựng chỉ mục ở thư mục tạm).

### 2.3 Bố cục fixture và `MANIFEST.json`

Theo CR-070 §2.3 (không chép lại). Các điều chỉnh theo hợp đồng:

- Thêm `gitnexus/1.6.9/check-cycles.json` **chỉ khi** AG-CV-SOL-037 dùng `check --cycles` (hợp đồng §2.4); chưa kiểm chứng đầu ra.
- Thêm `codegraph/1.4.1/affected.json` chỉ khi CR-003 dùng `codegraph affected`.
- `MANIFEST.json`: đúng cấu trúc CR-070 (`tool, toolVersion, capturedAt, hostOs, node, sourceRepo, sourceCommit, markers, files{argv,sha256,bytes}`); thêm `captureEnv: { HOME: "<tmp>", NO_COLOR: "1" }` để ai chụp lại biết điều kiện `spawn` (CR-070 §6: ANSI phụ thuộc TTY/`NO_COLOR`).
- Ngân sách: ≤ 20 KiB mỗi tệp, ≤ 300 KiB mỗi thư mục phiên bản (kiểm bằng `TestFixtureBudget`). `files.head.json` của CodeGraph cắt 20 mục.

```ts
// fixture-manifest.ts (mới)
export type FixtureFileEntry = { argv: readonly string[]; sha256: string; bytes: number }
export type FixtureManifest = {
  tool: 'gitnexus' | 'codegraph'
  toolVersion: string
  capturedAt: string
  hostOs: string
  node: string
  sourceRepo: 'mini-repo'
  sourceCommit: string
  captureEnv: Readonly<Record<string, string>>
  markers: { metaSchemaVersion?: number; extractionVersion?: number; schemaVersions?: readonly number[] }
  files: Readonly<Record<string, FixtureFileEntry>>
}
export const FIXTURE_BUDGET = { maxFileBytes: 20 * 1024, maxVersionDirBytes: 300 * 1024 } as const
export type LeakFinding = { rule: 'abs_path' | 'windows_path' | 'token_like' | 'file_hashes_key' | 'email'; sample: string; line: number }
export function scanTextForLeaks(text: string): LeakFinding[]
export function loadFixtureManifest(toolDir: string): FixtureManifest // ném Error có tên tệp khi sai schema
export function listFixtureVersionDirs(): ReadonlyArray<{ tool: FixtureManifest['tool']; version: string; dir: string }>
```

`scanTextForLeaks` quét `/home/`, `/opt/`, `/Users/`, `C:\`, mẫu token (`gh[pousr]_`, `github_pat_`, `AKIA`, `sk-`, JWT 3 đoạn base64url, `-----BEGIN`), khoá `fileHashes`/`cacheKeys`. Mẫu token trùng với bộ che stderr của AG-CV-SOL-001 (hợp đồng §9.4); nếu 001 xuất một danh sách mẫu thì import, **không** chép (một nguồn).

### 2.4 Script chụp (task 03) — chạy tay, không trong CI

`agent/scripts/capture-codeintel-fixtures.mjs --tool gitnexus@1.6.9 [--out <dir>] [--bench <outDir>]`:

1. So `<binary> --version` với phiên bản mục tiêu; lệch thì thoát mã ≠ 0, không ghi gì.
2. Sao `mini-repo/` ra thư mục tạm (`fs.mkdtemp(os.tmpdir() + '/orca-fixture-')`), `git init` + một commit (có commit thứ hai để `detect-changes` có hunk), **đặt `HOME` và `USERPROFILE` của mọi tiến trình con về thư mục tạm** để `~/.gitnexus/registry.json` toàn cục không bị ghi (CR-070 §6 rủi ro; việc HOME có cô lập registry là **chưa kiểm chứng**: script in cảnh báo nếu `~/.gitnexus/registry.json` thật đổi mtime sau khi chụp).
3. Dựng chỉ mục **chỉ trong thư mục tạm**: `gitnexus analyze --index-only` (PQ-37(c)), `codegraph init`/`index` (tên lệnh chưa kiểm chứng; script đọc từ `fixture-capture-plan.ts`, một chỗ để sửa).
4. Chạy đúng các lệnh trong `fixture-capture-plan.ts`; plan dựng từ **cùng hằng số `argv` của whitelist** (hợp đồng §9.1; export của AG-CV-SOL-001/002/003), không có chuỗi lệnh thứ hai.
5. Che: đường dẫn tuyệt đối → `<tmp>`/`<repo>`, thời gian giữ định dạng nhưng cố định, loại `fileHashes`/`cacheKeys` khỏi `meta.json`; sau đó **chạy `scanTextForLeaks`** trên mọi tệp, còn phát hiện thì không ghi.
6. Ghi `MANIFEST.json` (sha256, bytes), in `git diff --stat` để người duyệt thấy chính xác cái gì đổi.
7. Chế độ `--bench` ghi ra thư mục ngoài git (artifact), không phải fixture (CR-070 §2.8); phần đo thật ở AG-CV-SOL-071.

`esbuild-run-typescript-entry.mjs`: `build({ entryPoints:[entry], bundle:true, platform:'node', format:'esm', write:false, ... })`, ghi vào `os.tmpdir()`, `import()` rồi gọi `export default`/`main`. Dùng `esbuild` đã có (cùng cách `agent/build.mjs`); không thêm phụ thuộc. Chưa chạy; nếu esbuild không phân giải được từ `agent/scripts/` thì dùng đường tương đối `../node_modules/esbuild` (chưa kiểm chứng cách pnpm đặt esbuild).

### 2.5 Test parser (C1) và tệp vàng (C2)

Nguyên tắc của CR giữ nguyên: parser được test bằng **tệp thô** + `*.expected.json` (đầu ra chuẩn hoá, khoá sắp xếp ổn định, kiểm tay lần đầu). Tên hàm của CR (`parseCypherOutput`, `parseContext`, `parseImpact`, `parseQuery`, `parseDetectChanges`, `parseCodegraphJson`) là tên **tham khảo**; tên thật do AG-CV-SOL-002/003/005 chốt, test chỉ đổi dòng import.

| Test | Tệp | Mã lỗi khẳng định (hợp đồng §3.2) |
|---|---|---|
| `parseCypherOutput`: `{markdown,row_count}`; `[]` và `{markdown:"…",row_count:0}` đều 0 hàng; `{error}` (exit 0); ô có `\|`, `\n`, dấu nháy; chuỗi `'comm_x'` bỏ nháy; số cột khớp tiêu đề; CRLF; dòng lệch cột bị **bỏ + `warnings`** (hợp đồng §10: "dòng lệch bị bỏ + cảnh báo") | `gitnexus-cypher-output.golden.test.ts` | `{error}` chứa `Write operations` → `TOOL_FAILED reason=write_blocked`; `not found|does not exist` → `SYMBOL_NOT_FOUND`/`INVALID_PARAMS` theo hợp đồng §7.4; còn lại `unknown_shape` |
| `parseContext/Impact/Query`: `found`, `ambiguous` (+ `candidates`, tối đa 10), `not found`, `impact` không tìm thấy (`risk:"UNKNOWN"`, `impactedCount:0`, exit 0); khoá nhóm cạnh snake_case động; thiếu khoá bắt buộc | `gitnexus-json-commands.golden.test.ts` | `AMBIGUOUS_SYMBOL` (kèm `candidates`), `SYMBOL_NOT_FOUND`, thiếu khoá → `TOOL_FAILED reason=format_drift` |
| `parseDetectChanges`: văn bản → symbol + số tệp + mức rủi ro; biến thể CRLF sinh khi chạy test; số 0; dòng lạ → `format_drift`, không bỏ qua; trần 15 symbol, 10 luồng (README v7 điểm 8, theo CR, chưa kiểm) | `gitnexus-text-commands.golden.test.ts` | `format_drift` |
| CodeGraph `status/query/callers/callees/impact/files`; `[]`; đầu ra ANSI "not found" dù có `--json` → "không có kết quả", không phải lỗi JSON; `sqlite-schema.json` nếu CR-003 đọc SQLite | `codegraph-output.golden.test.ts` | JSON hỏng thật → `format_drift` |
| Lỗi quy trình: thiếu `-r` (stack trace trên stderr, liệt kê tên repo khác) → `REPO_NOT_REGISTERED` hoặc `TOOL_FAILED`, **thông điệp không chứa tên repo khác, đường dẫn nội bộ công cụ** (đã tái hiện bởi CR-072) | cùng tệp CodeGraph hoặc tệp riêng nếu 001 chia | `REPO_NOT_REGISTERED` |
| Mã thoát 0 + `error` | mọi lệnh | không tin mã thoát (D4 của CR) |
| `TestParserGolden` | khớp từng byte với `*.expected.json` | — |
| `TestResultMatchesServiceGolden` (C2) | `agent-result-golden.test.ts` | — |

Tệp vàng C2 (task 08): **theo bố cục của BE-CV-SOL-070 mục 5.1** (hai bên phải trùng tên): `testdata/agent-results/MANIFEST.json`; `gitnexus-1.6.9/{status-stale-repo-root,overview,processes,process,subgraph,impact-found,symbol-found,routes,detectChanges,structuralFacts}.json`; `codegraph-1.4.1/{status-ready,codegraphSearch,files.head}.json`; `merged/symbol-two-sources.json`; `errors/{ambiguous-symbol,tool-unavailable-schema,tool-failed-format-drift,index-missing,reindex-in-progress,timeout-queue-wait,output-too-large,path-not-allowed,repo-not-registered,symbol-not-found}.json` (phong bì lỗi JSON-RPC). `symbolref-key-vectors.json` và `security/` do BE/072. Mỗi tệp ≤ 20 KiB; `MANIFEST.json` ghi sha256 + bytes. Hình dạng: phong bì §2.2, **đã loại** `perf`, `startedAt`; `indexedAt`, `headCommit` cố định (commit `mini-repo`). Go **không** có cờ `-update` (BE D2): chỉ agent ghi. Cập nhật chỉ khi đặt `ORCA_UPDATE_GOLDEN=1` (PR riêng, nêu phiên bản công cụ); mặc định chỉ so sánh. Khoá JSON sắp xếp tăng dần, thụt 2 dấu cách, kết thúc `\n`, để diff ổn định cho cả Go và TS.

### 2.6 Phiên bản và marker (task 07)

```ts
// SUPPORTED_* ở một chỗ duy nhất của agent (AG-CV-SOL-001 sở hữu; thêm vào đó, không tạo bản thứ hai)
export const SUPPORTED_TOOL_VERSIONS = { gitnexus: ['1.6.9'], codegraph: ['1.4.1'] } as const
export const SUPPORTED_TOOL_RANGES = { gitnexus: '>=1.6.0 <2', codegraph: '>=1.4.0 <2' } as const
export const SUPPORTED_SCHEMA_MARKERS = {
  gitnexus: { metaSchemaVersion: [5] },
  codegraph: { extractionVersion: [24], schemaVersionsMax: 8 }
} as const
export type ToolCompatibility = 'verified' | 'untested' | 'incompatible'
```

Bảng hành vi: `verified` phục vụ; `untested` phục vụ + `warnings:["tool_version_untested"]`; `incompatible` → method đọc trả `CODEINTEL_TOOL_UNAVAILABLE reason="unsupported_version"` hoặc `"schema_version_unsupported"` kèm `{tool, seen, supported}`; `reindexRecommended` (hoặc `builtWithExtractionVersion != currentExtractionVersion`) phục vụ + `warnings:["index_built_with_old_extraction"]`. Một so sánh semver tối thiểu viết tay (không thêm `semver`; repo có `semver` hay không là chưa kiểm). Shape guard của mỗi parser trả `format_drift`, ghi log **một lần mỗi `(tool, version, command)`**, không kèm đầu ra thô.

### 2.7 CI (task 09)

`.github/workflows/code-intel-contract.yml`:

| Job | Khi | Nội dung | Chặn |
|---|---|---|---|
| `code-intel-contract` | `pull_request` với `paths: ['agent/**', 'backend-go/services/code-intel-service/testdata/**', '.github/workflows/code-intel-contract.yml']` | checkout, setup node (`node-version-file: package.json` như `pr.yml`), setup pnpm, `pnpm install --frozen-lockfile` (phạm vi cài và `--ignore-scripts` chưa kiểm chứng), `pnpm --filter orca-agent exec vitest run --config vitest.code-intel-contract.config.ts`, rồi bước khẳng định "có test thật chạy" | **Có** |
| `code-intel-live-contract` | `schedule` hằng đêm + `workflow_dispatch` | cài `npm i -g gitnexus` (theo `AGENTS.md`/CR-070: bản mới nhất) và CodeGraph (cách cài không tương tác **chưa biết**), chạy script chụp vào thư mục tạm, `diff -r` với fixture đã commit, đẩy artifact | Không |

Bước khẳng định: `vitest list --config ...` phải liệt kê ≥ 1 test của mỗi tệp trong `agent-result-golden`, `fixture-manifest`, `tool-compatibility`; thiếu thì thoát ≠ 0. Có thể dựa vào bước này để không "xanh vì không chạy gì".

`agent/vitest.code-intel-contract.config.ts`: `include` = `src/relay/codeintel/**/*.test.ts`, `src/relay/codeintel-*.test.ts`, `src/relay/gitnexus-*.test.ts`, `src/relay/codegraph-*.test.ts`, `src/relay/quality-*.test.ts`; `environment: 'node'`. Lý do cấu hình riêng: vitest gốc (`vitest.config.ts`) chạy 365 tệp `*.test.ts` (đếm bằng `find agent/src`, chưa chạy) trong đó có thể có test cần PTY/native (chưa kiểm); job hợp đồng phải nhanh và ổn định.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Chụp từ `mini-repo/`, không từ Orca | CR-070 D1: ổn định, nhỏ, có ca đối kháng |
| D2 | Tầng offline chặn PR; tầng live không chặn | CR-070 D2; công cụ ngoài không ổn định |
| D3 | Danh sách phiên bản hỗ trợ là dữ liệu + test "mỗi phiên bản có fixture" | CR-070 D3 |
| D4 | Không tin mã thoát; kiểm nội dung | CR-070 D4; `{error}` kèm exit 0 |
| D5 | Trôi định dạng → lỗi rõ + `format_drift`, không đoán | CR-070 D5 |
| D6 | Dùng `reason` (không `detail`), mã lỗi theo hợp đồng §3.2 | Hợp đồng thắng CR |
| D7 | Tệp vàng C2 do agent sinh, Go đọc; khoá sắp xếp, `perf`/thời gian loại | CR-070 D7; hợp đồng §8 |
| D8 | Cấu hình vitest riêng cho job hợp đồng; không sửa `pr.yml` | `pr.yml` đã trỏ `config/vitest.config.ts` không tồn tại, sửa nó là việc riêng (chưa duyệt) |
| D9 | Không thêm phụ thuộc (không `semver`, `fast-check`, `tsx`) | O12 (chưa duyệt); esbuild đã có |
| D10 | Pure logic (quét rò rỉ, so phiên bản, phân loại) viết TS, test bằng vitest; script `.mjs` chỉ là vỏ | `.mjs` không có kiểu, tsc không phủ; tránh test import `.mjs` |

## 4. Tiêu chí chấp nhận

- [ ] `mini-repo/` và script chụp có mặt; script từ chối chạy khi phiên bản không khớp; `analyze`/`init` chỉ chạy trong thư mục tạm với `HOME` cô lập; `analyze` luôn có `--index-only`.
- [ ] Fixture `gitnexus/1.6.9` và `codegraph/1.4.1` đủ danh sách 2.3; mỗi thư mục có `MANIFEST.json` (`argv` mẫu, `sha256`, `bytes`); `TestFixtureBudget`, `TestFixtureHasNoLeaks`, `TestManifestHashes` xanh.
- [ ] Parser đúng: hai hình dạng rỗng của `cypher`, ô có `|`/`\n`, `{error}` kèm exit 0, văn bản `detect-changes` (cả CRLF), ANSI "not found" của CodeGraph.
- [ ] `TestEverySupportedVersionHasFixtures` xanh; thêm phiên bản giả vào `SUPPORTED_TOOL_VERSIONS` mà không có fixture thì đỏ (kiểm bằng test nhỏ dùng đối tượng tiêm vào, không sửa hằng số thật).
- [ ] `codeintel.status` (qua hàm phân loại) cho `supported` + `warnings` đúng bảng 2.6; marker lạ → `CODEINTEL_TOOL_UNAVAILABLE reason=schema_version_unsupported`.
- [ ] Sai hình dạng → `CODEINTEL_TOOL_FAILED reason=format_drift`; thông điệp không chứa đầu ra thô.
- [ ] Tệp vàng C2 được sinh bằng `ORCA_UPDATE_GOLDEN=1`, so khớp ở chế độ thường; BE-CV-SOL-070 đọc được cùng tệp.
- [ ] Workflow có job `code-intel-contract` chạy trên PR chạm `agent/**` và có bước khẳng định test thật chạy; `workflow_dispatch` chạy được tầng live (cách cài CodeGraph chưa kiểm chứng).
- [ ] Không có tệp mẫu nào từ Orca trong git; không `max-lines` disable; tên tệp theo khái niệm.

## 5. Kiểm thử

Bảng 2.5 và danh sách ở từng task. Lệnh: trong `/opt/repos/orca/agent`, `pnpm exec vitest run src/relay/codeintel/<tệp>`; toàn job: `pnpm exec vitest run --config vitest.code-intel-contract.config.ts`. Kiểm kiểu: `npx tsc --noEmit -p tsconfig.json` (chỉ so sánh trước/sau; số lỗi có sẵn chưa biết, chưa chạy).

Thử ngược (CR-070 §5): sửa tay một fixture (đổi tên cột, bớt một khoá) thì golden đỏ và thông điệp chỉ tệp; thực hiện trong task 04 dưới dạng test dùng bản sao trong bộ nhớ.

Chưa chạy: toàn bộ.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa chạy `gitnexus analyze --index-only` hay `codegraph init` trên `mini-repo/`; chưa biết `HOME` cô lập có đủ để registry toàn cục không bị ghi, và công cụ có dựng chỉ mục được trên cây nhỏ không cấu hình (CR-070 §6).
- `gitnexus trace`, `check --cycles`, `codegraph affected`, `node`, `explore`: chưa có mẫu đầu ra; chỉ chụp khi parser tương ứng tồn tại.
- `detect-changes` là văn bản người đọc; đổi câu chữ làm vỡ parser (tầng live bắt sớm).
- ANSI trong "not found" của CodeGraph phụ thuộc TTY/`NO_COLOR`; fixture phải chụp đúng điều kiện `spawn` của agent (`stdio` pipe, `NO_COLOR=1`; hợp đồng §2.4).
- Hai bản `agent-tool-registry.ts` (`agent/`, `desktop/`): nếu bản chạy thật là bản `desktop/`, phải đổi vị trí (CR-070 §6). Hợp đồng §8 và README v7 điểm 20 đã coi `agent/` là nơi sở hữu; solution theo đó.
- `pnpm install` đầy đủ ở runner có thể nặng (Electron...); phạm vi cài tối thiểu chưa thử; job có thể chậm hơn "vài phút" của CR (chưa đo).
- Windows/WSL: chỉ có biến thể CRLF sinh tay; method trả `unsupported_platform` nên không chụp trên Windows.
- SSH (AGENTS.md): fixture chụp trên máy cục bộ; hành vi qua `--stdio` (Part A) chạy cùng mã, không kiểm riêng ở đây.
- Tệp vàng nằm trong cây `backend-go/` nhưng do mã agent sinh: nếu hai bên đổi cùng lúc cần PR phối hợp.

## 7. Câu hỏi mở và điểm hợp đồng thiếu hoặc mâu thuẫn

1. **Thiếu trong hợp đồng:** trường `compatibility` (tri-state) và hai chuỗi `warnings` (`tool_version_untested`, `index_built_with_old_extraction`). Đề nghị bổ sung vào §2.2/§4.1; solution chạy được mà không cần (ánh xạ sang `supported`).
2. **Mâu thuẫn:** hợp đồng §11 đặt mô-đun phẳng `agent/src/relay/codeintel-*.ts` nhưng fixture ở `agent/src/relay/codeintel/__fixtures__/`; bộ lọc `paths` của CR-070 (`agent/src/relay/codeintel/**`) không bắt tệp phẳng. Xin chốt một dạng.
3. Có sửa `pr.yml`/script `test` gốc (trỏ `config/vitest.config.ts` không tồn tại) không? Ngoài phạm vi; ảnh hưởng toàn repo.
4. Repo mẫu: cây tệp thường hay submodule (CR-070 Q2)? Mặc định cây thường.
5. Tầng live dùng "bản mới nhất" hay pin `~minor` (CR-070 Q3)? Mặc định không pin (báo sớm).
6. Nếu AG-CV-SOL-005 chuyển `detect-changes` sang Cypher thì fixture văn bản còn cần không (CR-070 Q4)? Giữ tới khi quyết.
7. Parser đầu ra của công cụ chất lượng (oxlint/tsc/vitest/go vet...; CR-082) **không** thuộc CR-070 nên chưa có fixture; có mở rộng solution này hay để AG-CV-SOL-082 tự lo? Đề nghị dùng lại `fixture-manifest.ts` và workflow này.
8. Chủ sở hữu `ORCA_UPDATE_GOLDEN`: một PR đổi tệp vàng buộc cả BE-CV-SOL-070 cập nhật collector; cần quy tắc review chung.
