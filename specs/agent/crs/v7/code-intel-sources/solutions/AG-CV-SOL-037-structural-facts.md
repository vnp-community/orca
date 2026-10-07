# AG-CV-SOL-037: Method `codeintel.structuralFacts` (lớp import, vòng, in-degree, kích thước tệp, export không dùng)

> ✅ **Đã triển khai.** Ngày triển khai 2026-10-07. Đã hoàn thành toàn bộ các task 037-01 đến 037-07, kiểm thử tự động xác nhận qua vitest, đạt 100% tiêu chí chấp nhận.

**CR:** [CR-CV-037](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-037-structure-analysis.md) mục 2.2 (phần agent). Phát hiện, `finding_key`, `finding_dismissals`, hotspot, owner thuộc `BE-CV-SOL-037-structure-findings-and-dismissals`. Task AG-CV-TASK-037-01 đến 07. **Khu vực:** `agent/src/relay/`. **Feature:** `code-intel-sources`.
**TDD/Spec:** [TDD-AG-01](../../../../tdd/v5/01-architecture.md), [TDD-AG-07](../../../../tdd/v5/07-jsonrpc-dispatch.md), [api/agent-rpc-catalog-runtime.md](../../../../api/agent-rpc-catalog-runtime.md).

## 1. Hợp đồng áp dụng

| Mục ([`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md)) / PQ | Áp vào |
|---|---|
| §4.9 `codeintel.structuralFacts`: tham số `kind` (bb: `layerImports|cycles|importInDegree|fileSizes|unusedExports`), `pair`, `pathPrefixes` (≤ 20, mặc định `["backend-go/services/"]`), `limit` (1..5000, mặc định 5000), `offset`; dạng `data` theo `kind` (luôn có `"kind"`); thực thi mẫu cố định chỉ đọc ~1,7-1,9 s/lệnh | tasks 02-06 |
| §2.2 phong bì (`sources`, `headCommit`, `stale`, `truncated`, `totalCount`, `warnings`, `perf`) | task 07 |
| §2.3 giới hạn (timeout CLI 20 s; method `structuralFacts` **55 s** (env `ORCA_CODEINTEL_DETECT_TIMEOUT_MS`); stdout ≤ 16 MiB; JSON ≤ 8 MiB; cache 60 s) và §2.5 (Go 90 s) | task 02, 07 |
| §2.4 Cypher: mẫu hằng, 4 bộ mã hoá, `assertReadOnlyCypher`; `analyze/…/check` bị cấm trong đường đọc, **`check --cycles` chỉ mở riêng cho `structuralFacts kind:"cycles"`**; stdout GitNexus qua tệp tạm | tasks 03-06 |
| §3.2 mã lỗi: `TOOL_UNAVAILABLE`, `INDEX_MISSING`, `TOOL_FAILED` (`check` lỗi/cắt cụt), `TIMEOUT`, `OUTPUT_TOO_LARGE`, `INVALID_PARAMS`, `REINDEX_IN_PROGRESS` | task 02, 07 |
| §9.1 whitelist đóng: `TestWhitelistIsClosed` (thêm đúng `check --cycles`), `TestUserValuesNeverStartWithDash`, `TestRepoFlagIsLast` | task 03 |
| PQ-20 (`SymbolRef` chuẩn hoá ở agent), PQ-21 (`workspaceRoot`), PQ-03 | tasks 02, 06 |
| §8.3, AGENTS.md | trích |

## 2. Lệch giữa CR và hợp đồng

| # | CR-CV-037 | Hợp đồng | Solution theo |
|---|---|---|---|
| 1 | Đầu ra `layerImports`: "cặp `(fromFile, toFile)`; agent khử trùng theo thư mục đích" | §4.9 `rows: [{pair, fromFile, toFile}]` (không có `toDir`) | Khử trùng theo `(fromFile, dirname(toFile), pair)`, giữ `toFile` nhỏ nhất từ điển làm đại diện; backend suy `toDir` từ `toFile` |
| 2 | `fileSizes` có hàm dài nhất `{name, lines}`; truy vấn chỉ có `max()` (không tên) | §4.9 `longest:{name, lines}` | Cần truy vấn bổ sung; **chưa chạy** (task 01/05). Nếu không có tên: `longest.name:""` + `warnings:["longest_symbol_name_unavailable"]` (mã mới, chưa có trong hợp đồng — câu hỏi mở 1) |
| 3 | `cycles` có thể tự tính SCC khi không có method | §4.9: `cycles` qua `gitnexus check --cycles --json -r <repo>` | Hợp đồng |
| 4 | `pair` không đặt = cả 4 | §4.9 giống | Theo |
| 5 | `unusedExports` dùng `Function` export; loại `_test.go`, `/cmd/`, `usecasetest` | §4.9 giống (+ `rows[{symbol: SymbolRef}]`) | Theo; chuyển sang `SymbolRef` chuẩn hoá (PQ-20) |
| 6 | Timeout "55 s" chưa nêu | §2.3: 55 s, Go 90 s; `ORCA_CODEINTEL_DETECT_TIMEOUT_MS` hạ về 25 s nếu infra-fleet chưa nâng | Theo (dùng chung hằng với `detectChanges`) |
| 7 | `limit` ≤ 5 000 dòng | §4.9: `limit`/`offset` | Phân trang theo hàng; `cycles` phân trang theo vòng |

## 3. Phụ thuộc chéo khu vực

| Đối ứng | Quan hệ |
|---|---|
| `AG-CV-SOL-001` | **Điều kiện trước**: `codeintel-method-table.ts`, `codeintel-command-whitelist.ts` (`GitNexusCommand`), `codeintel-tool-runner.ts` (tệp tạm), `codeintel-limits.ts`, `codeintel-errors.ts`, `codeintel-result-envelope.ts`. Chưa tồn tại (đã `ls`); ở thời điểm soạn đã có solution/task 001 nhưng code thì chưa |
| `AG-CV-SOL-002-gitnexus-extraction` | **Điều kiện trước**: runner Cypher có mã hoá slot, `assertReadOnlyCypher`, parser bảng markdown theo cột cố định, `codeintel-symbol-ref.ts`, `codeintel-short-lived-cache.ts`. Giao diện giả định ở 5.2; nếu SOL-002 chốt khác thì chỉ sửa lớp nối (task 04-06) |
| `BE-CV-SOL-037-structure-findings-and-dismissals` | Gọi `codeintel.structuralFacts` (collector); tự khử trùng theo package, dựng `Finding` |
| `BE-CV-SOL-021-agent-collector`, `BE-CV-SOL-023-…` | Timeout Go 90 s |
| `AG-CV-SOL-070-golden-fixtures-and-parsers` | Fixture `structural-*.json` + mẫu Cypher chạy được trên LadybugDB (CR-070 test mẫu truy vấn) |
| `AG-CV-SOL-072-security-tests-agent` | `TestWhitelistIsClosed` mở rộng có `check --cycles` |
| Thứ tự hợp đồng §7.2: `002 → 037-AG` (đợt 5) |

## 4. Re-verify (đã đọc, 2026-10-06)

Đã đọc: `agent/src/relay/agent-rpc-dispatch.ts` (route, `makeError`), `agent-tool-registry.ts` (`runToolCommand`), `ls agent/src/relay | grep -i codeintel` (không có), `agent/vitest.config.ts`, CR-037 mục 1-2, hợp đồng §2-4, `specs/agent/crs/v7/agent-codeintel/solutions/AG-CV-SOL-001*`, `AG-CV-SOL-002*` (tên file/giao diện). Không chạy `gitnexus`.

| Điểm | Hiện trạng | Hệ quả |
|---|---|---|
| `gitnexus check --cycles --json` | Quan sát duy nhất (CR): 93 vòng, `status:"cycles_found"`, `cycleCount`, `cycles[].files[]` (tệp đầu lặp ở cuối) | Fixture task 01 xác nhận, kể cả cờ `-r` và stdout qua tệp tạm |
| Cypher: `WHERE (s:A OR s:B)` | Lỗi parser LadybugDB (CR) | Hai truy vấn theo nhãn, gộp ở agent; test chặn cú pháp này |
| `IMPORTS` mức tệp | Một import package → cạnh tới từng tệp của package; không có số dòng | Khử trùng theo thư mục đích |
| `NOT EXISTS {…}`, `ENDS WITH`, `count(DISTINCT a)` | Chạy được theo CR (~1,7 s) | Mẫu nằm trong test mẫu của CR-070; **chưa chạy bởi solution này** |
| Truy vấn lớn qua pipe | Một lần thử kéo 38 337 cạnh bị cắt giữa chuỗi (CR) | Luôn tệp tạm + gộp ở phía công cụ + `LIMIT` |

Correction relative to CR: CR-037 2.2 mô tả method nằm ở "CR-CV-001/002"; hợp đồng §8.2 giao `AG-CV-SOL-037-structural-facts` riêng (feature `code-intel-sources`) và §11 đưa vào bảng việc agent. Theo hợp đồng.

## 5. Giải pháp

### 5.1 Cây file (mới)

```
codeintel-structural-facts.ts / .test.ts          task 02  validate, handler, dispatch theo kind, phân trang
codeintel-structural-facts-queries.ts / .test.ts  tasks 04-06  mẫu Cypher hằng (chỉ đọc)
codeintel-structural-cycles.ts / .test.ts         task 03  `check --cycles --json`
codeintel-structural-layer-imports.ts / .test.ts  task 04
codeintel-structural-file-metrics.ts / .test.ts   task 05  importInDegree + fileSizes
codeintel-structural-unused-exports.ts / .test.ts task 06
__fixtures__/structural-facts/                    task 01
```
Sửa nhỏ: `codeintel-command-whitelist.ts` (thêm biến thể `check-cycles`), `codeintel-method-table.ts` (một dòng, `timeoutMs: 55_000`).

### 5.2 Giao diện giả định từ AG-CV-SOL-001/002 (chốt khi triển khai)

```ts
type CypherRunner = (binding: RepoBinding, template: CypherTemplate, slots: Record<string, SlotValue>, columns: readonly string[]) => Promise<string[][]>
type CheckCyclesRunner = (binding: RepoBinding) => Promise<{ status: string; cycleCount: number; cycles: { files: string[] }[] }>
```
`CypherTemplate` là hằng; slot được mã hoá bằng `cypherString`/`cypherInt` (SOL-002); không có API Cypher tự do.

### 5.3 Dạng `data` (đúng hợp đồng §4.9)

```jsonc
{ "kind": "layerImports", "rows": [ { "pair": "usecase->adapter", "fromFile": "…/internal/usecase/ports.go", "toFile": "…/internal/adapter/eventbus/publisher.go" } ] }
{ "kind": "cycles", "cycleCount": 93, "status": "cycles_found|clean", "cycles": [ { "files": ["a.ts","b.ts","a.ts"] } ] }
{ "kind": "importInDegree", "rows": [ { "file": "…", "inDegree": 123 } ] }
{ "kind": "fileSizes", "rows": [ { "file": "…", "functions": 12, "totalLines": 480, "longest": { "name": "…", "lines": 90 } } ] }
{ "kind": "unusedExports", "rows": [ { "symbol": { /* SymbolRef */ } } ] }
```

## 6. Quyết định thiết kế

| # | Quyết định | Lý do | Bỏ |
|---|---|---|---|
| 1 | Một method hẹp có `kind` liệt kê, không Cypher tự do | D5; giải quyết giới hạn 4 000 cạnh của `subgraph` | Mở `cypher` |
| 2 | Khử trùng `IMPORTS` ở agent | GitNexus trải cạnh tới từng tệp của package | Để backend khử |
| 3 | Lọc `pathPrefixes` bằng chuỗi `STARTS WITH` sinh từ slot đã mã hoá (≤ 20) | Giữ kết quả nhỏ | Lọc sau ở TS (kéo cả repo) |
| 4 | Hai truy vấn `Function`/`Method` cho `fileSizes` | Không hỗ trợ `(s:A OR s:B)` | Một truy vấn |
| 5 | `check --cycles` chỉ với `-r <repo>` và `--json`, stdout qua tệp tạm | §2.4, §9.1; pipe cắt cụt | `--branch`, cờ khác |
| 6 | Sắp xếp xác định mọi `rows` | Cache/ETag ở backend | Thứ tự công cụ |
| 7 | Một bộ phát hiện lỗi không làm mất bộ khác | Mỗi `kind` là một lời gọi riêng | — |

## 7. Tiêu chí chấp nhận

- [x] `kind` bắt buộc; giá trị lạ, `pair` với `kind ≠ layerImports`, `pathPrefixes` > 20 hoặc có `..`/tuyệt đối/bắt đầu `-`, `limit` ngoài 1..5000 → `CODEINTEL_INVALID_PARAMS data.field`; khoá lạ bị từ chối.
- [x] `layerImports` trên fixture (mô phỏng Orca: infra-fleet, ai-provider, mcp `usecasetest`): 4 pair, loại `_test.go`/`usecasetest`, khử trùng theo thư mục đích (một import package ra **một** hàng/tệp nguồn), `domain->*` 0 hàng.
- [x] `cycles`: `cycleCount` khớp; `status` đúng; `check` lỗi hoặc stdout cụt → `TOOL_FAILED reason="truncated_stdout"`; argv chỉ `check --cycles --json -r <path>`.
- [x] `importInDegree`, `fileSizes` (gộp `Function`+`Method`), `unusedExports` (`SymbolRef` chuẩn hoá, loại `_test.go`, `/cmd/`, `usecasetest`).
- [x] Phân trang `limit/offset` xác định; `totalCount` trước cắt; `truncated` đúng; thứ tự ổn định qua 100 lần chạy.
- [x] Không có chuỗi `CREATE|MERGE|DELETE|SET|REMOVE|DROP|…` trong mọi mẫu (`assertReadOnlyCypher`); không có `(x:A OR x:B)`.
- [x] Timeout method 55 s; mỗi CLI ≤ 20 s; quá → `CODEINTEL_TIMEOUT` (partial không trả).
- [x] `TestWhitelistIsClosed` chỉ thêm `check --cycles`; `analyze|clean|…` vẫn bị cấm.

## 8. Kiểm thử

Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-structural-facts.test.ts src/relay/codeintel-structural-cycles.test.ts src/relay/codeintel-structural-layer-imports.test.ts src/relay/codeintel-structural-file-metrics.test.ts src/relay/codeintel-structural-unused-exports.test.ts src/relay/codeintel-command-whitelist.test.ts`; sau cùng `pnpm test`. Fixture từ task 01; hợp đồng mẫu truy vấn chạy được trên LadybugDB nằm ở job CR-070 `code-intel-live-contract` (không chặn).

## 9. Rủi ro và chưa kiểm chứng

- Mọi mẫu Cypher chưa chạy bởi solution này; điều kiện prerequisite merge (CR-002 §2.1).
- `check --cycles`: giới hạn độ dài/số vòng, thời gian, `--branch` chưa biết.
- `longest.name` cần truy vấn chưa thử.
- Phân loại `kind` của GitNexus sai (`ErrServerNotApproved` mang nhãn `Function`): ảnh hưởng `unusedExports`.
- Chỉ mục có thể lệch HEAD: `stale` ở phong bì; kết quả vẫn trả.
- Part A/`direct-websocket` (và `--stdio` qua Part A); Part B không (CR-006).
- Windows chưa hỗ trợ (`unsupported_platform`).

## 10. Câu hỏi mở

1. Thêm cảnh báo `longest_symbol_name_unavailable` vào hợp đồng hay bỏ `longest.name`?
2. `unusedExports` mở rộng TypeScript khi nào (CR-037 Q7)? Mặc định: chỉ những gì backend yêu cầu qua `pathPrefixes`; agent không lọc ngôn ngữ.
3. Có cần `kind` tách `cycles` theo thư mục gốc (frontend/desktop/...) để giảm tải? Mặc định: không, backend lọc.
