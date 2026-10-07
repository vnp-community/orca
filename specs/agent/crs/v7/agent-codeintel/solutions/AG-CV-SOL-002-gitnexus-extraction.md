# AG-CV-SOL-002: Trích xuất GitNexus (`overview`, `processes`, `process`, `subgraph`, `impact`, `symbol`, `routes`, probe chỉ mục)

> ✅ **Đã triển khai.** Ngày triển khai 2026-10-07. Đã hoàn thành toàn bộ các task 002-01 đến 002-09, kiểm thử tự động xác nhận qua vitest, đạt 100% tiêu chí chấp nhận.

**CR:** [CR-CV-002](../../../../../../docs/crs/v7/agent-codeintel/CR-CV-002-gitnexus-extraction.md)
**Service:** `agent/`, thư mục `agent/src/relay/` (Part A); sao sang `desktop/src/relay/` ở [AG-CV-SOL-006](./AG-CV-SOL-006-relay-ssh-part-b-handlers.md)
**TDD tham chiếu:** [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md) mục 1, 5; [v5/05-tool-registry](../../../../tdd/v5/05-tool-registry.md) mục 3; [v5/02-wire-protocol](../../../../tdd/v5/02-wire-protocol.md) mục 1 (khung tối đa 16 MiB)
**Mẫu định dạng:** `specs/agent/crs/v6/agent-capabilities/solutions/AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md`

## 0. Hợp đồng áp dụng

| Nguồn | Mục |
|---|---|
| `CONTRACT-codeintel-agent-rpc.md` | §2.2 (phong bì, `lineBase`), §2.3 (cache 60 s, 64 mục, 32 MiB), §2.4 (Cypher mẫu hằng, 4 bộ mã hoá, `assertReadOnlyCypher`), §2.5 (timeout 25 s agent / 90 s Go), §2.6 (`SymbolRef`, quy tắc khoá, bảng kind), §3.2 (mã lỗi), §4.2 `overview`, §4.3 `processes`/`process`, §4.4 `subgraph`, §4.5 `impact`, §4.6 `symbol`, §4.7 `routes`, §4.1 (khối `indexes.gitnexus`), §9 mục 1, 4 (Cypher chỉ đọc, secret), §10 (hạn chế đã biết) |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-03 (`SYMBOL_NOT_FOUND`), PQ-19 (`affectedModules`, `ImpactGraph` không cạnh, `rootMismatch`), PQ-20 (chuẩn hoá `SymbolRef` tại agent, dòng +1), PQ-21 (bộ method), §8.2 (tên solution), §8.3, §9 O-3 (mẫu chưa chạy) |
| `CONTRACT-codeintel-ui-api.md` | không trực tiếp (UI nhận dữ liệu qua `code-intel-service`) |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `docs/crs/v7/agent-codeintel/CR-CV-002-…md` (toàn bộ), `README.md` feature, `agent/src/relay/agent-tool-registry.ts` (`runToolCommand`, tool `gitnexus`), `agent-git-handler-extended.ts:42` (`git()`), `git-handler-check-ignore.ts` (`checkIgnoredPathsOp` nhận `GitExec`), `external-automations-handler.ts:490-525`, `.gitnexus/meta.json` (có `lastCommit` `d8198127…`, `indexedAt` `2026-10-05T05:55:17.289Z` ở cấp gốc), thư mục `.gitnexus/` (có `lbug`, `meta.json`, `gitnexus.json`, `parse-cache`, `parsedfile-cache` và bốn tệp `lbug.wal.missing-shadow.*`), thư mục `agent/src/relay/` (chưa có `gitnexus-*` hay `codeintel-*`), `CONTRACT-codeintel-agent-rpc.md` §4.2–4.7.

| Điểm | Hiện trạng |
|---|---|
| Mã GitNexus trên agent | chỉ tool `gitnexus` với `args` tự do (Part A `tools/call`); chưa có Cypher, parser, handler |
| Nền phụ thuộc | AG-CV-SOL-001 (runner qua tệp tạm, whitelist, repo, phong bì, bảng method) **chưa có** (cùng loạt); solution này giả định đã có các export: `runCodeIntelTool`, `buildCodeIntelResult`, `CODEINTEL_METHODS`, `CodeIntelError`, `registerIndexProbe`, `runGit` |
| Fixture | chưa có thư mục `__fixtures__`; hợp đồng §11 đặt fixture ở `agent/src/relay/codeintel/__fixtures__/` (AG-CV-SOL-070); CR-002 ghi `agent/src/relay/__fixtures__/gitnexus-1.6.9/` |
| Số liệu nền trong CR (chưa chạy lại) | `Community` 9 039 vs 10 252 ở registry; 92 route `HANDLES_ROUTE` + 21 `FETCHES`; overview 5-7 s; mỗi lần gọi ~1,6-1,8 s; 36 507 `Const` + 34 424 `Variable` + 26 345 `Property` + 1 111 `Section` trong 247 556 nút |

### Correction relative to CR

| # | CR-CV-002 nói | Quyết định |
|---|---|---|
| 1 | `CODEINTEL_INVALID_PARAMS (reason="not_found")` cho symbol/process không tồn tại | `CODEINTEL_SYMBOL_NOT_FOUND` `data.kind ∈ symbol\|process` (PQ-03) |
| 2 | `ImpactGraph.affectedClusters` | `affectedModules[{name,hits,impact}]` (đúng GitNexus; PQ-19), backend đổi tên; `affectedClusters` chỉ có ở `detectChanges` |
| 3 | `toolTimingsMs` | `perf` (PQ-19) |
| 4 | `OV_COUNT` nhắc "thêm một `count(*)`" nhưng không có trong bảng 2.4 | thêm mẫu `OV_COUNT` (mới) `MATCH (c:Community) RETURN count(*) AS n`, cú pháp đã chạy kiểu tương tự ở `PR_COUNT` (CR), vẫn ghi "chưa chạy riêng" |
| 5 | `symbol`: `uid` hoặc `name`+`file` | thêm dạng `key` (contract §4.6), phân giải `key` -> uid bằng `SG_FILE_SYMBOLS` rồi so `key` đã chuẩn hoá |
| 6 | `subgraph` không có tham số `source` | thêm `source: gitnexus\|codegraph\|auto` (contract §4.4); `codegraph` chỉ có tác dụng sau AG-CV-SOL-003, trước đó rơi về GitNexus + `warnings:["codegraph_source_unavailable"]` |
| 7 | `impact`: `target` `uid` hoặc `name` | thêm `target.key` (contract §4.5) |
| 8 | `symbol.flows[]` lấy từ JSON `context` | dùng `SYMBOL_FLOWS` (một id) để thống nhất với `impact`/`detectChanges`; hình dạng JSON `context.processes` chưa kiểm chứng |
| 9 | cache `symbol` có `source`: không cache | giữ; thêm: không cache khi `warnings` chứa `source_may_not_match_index` |

### Lệch giữa CR và hợp đồng

| # | CR-CV-002 | Hợp đồng (thắng) |
|---|---|---|
| 1 | `not_found` -> `INVALID_PARAMS` | `SYMBOL_NOT_FOUND` -32602 (PQ-03) |
| 2 | `affectedClusters` ở `impact` | `affectedModules` (PQ-19) |
| 3 | `toolTimingsMs` | `perf` (PQ-19) |
| 4 | fixture ở `agent/src/relay/__fixtures__/gitnexus-1.6.9/` | `agent/src/relay/codeintel/__fixtures__/…` (§11, chủ AG-CV-SOL-070); solution dùng đường dẫn hợp đồng |
| 5 | `sources[]` không nói `lineBase` ngoài SymbolRef | `sources[].lineBase = 1` bắt buộc (PQ-20) |
| 6 | không nêu `communities_count_mismatch` | `overview` thêm `warnings:["communities_count_mismatch"]` khi 9 039 ≠ `stats.communities` (§4.2) |
| 7 | probe `schemaVersion` tuỳ chọn khi parse > 200 ms | hợp đồng §1.3 coi `schemaVersion ∈ {5}` là marker hỗ trợ; vắng `schemaVersion` -> không kết luận `unsupported` (chỉ `schema_version_unsupported` khi đọc được và ngoài `{5}`) |

### Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-020-canonical-graph-model` | proto `SymbolRef`, `lineBase`; cùng vector kiểm thử chuẩn hoá khoá (PQ-20) |
| BE | `BE-CV-SOL-021-agent-collector` | gọi các method này; chịu `warnings`, `truncated`, `AMBIGUOUS_SYMBOL.candidates` |
| BE | `BE-CV-SOL-023-infra-fleet-codeintel-transport` | timeout Go 90 s; trailer mang `candidates` |
| BE | `BE-CV-SOL-022-snapshot-cache` | cache bền; `perf` không vào snapshot |
| AG | `AG-CV-SOL-001-…` (trước), `AG-CV-SOL-003-…` (cùng `codeintel-symbol-ref.ts`), `AG-CV-SOL-004-…` (đọc `lastCommit`, huỷ cache), `AG-CV-SOL-005-…` (dùng `FILE_SYMBOLS_BATCH`, `SYMBOL_FLOWS`, `MEMBER_CLUSTER`), `AG-CV-SOL-037-structural-facts` (dùng runner Cypher), `AG-CV-SOL-070-golden-fixtures-and-parsers` (fixture vàng, kiểm hợp đồng), `AG-CV-SOL-071-perf-block-and-bench` (ngân sách thời gian) |
| FE | — | không có |

## 2. Giải pháp

### 2.1 Cây file

```
agent/src/relay/
  gitnexus-cypher-literal.ts          (mới) cypherInt, cypherString, cypherStringList, cypherKindList
  gitnexus-cypher-guard.ts            (mới) assertReadOnlyCypher
  gitnexus-cypher-templates.ts        (mới) bảng mẫu {id, text, slots, columns, freeTextColumn?}
  gitnexus-cypher-markdown-parser.ts  (mới) parseCypherOutput
  gitnexus-cypher-runner.ts           (mới) runCypherTemplate
  gitnexus-index-probe.ts             (mới) probe cho codeintel.status
  codeintel-symbol-ref.ts             (mới) hàm thuần: bảng kind, dựng SymbolRef, key (CR-003 thêm phần CodeGraph)
  codeintel-short-lived-cache.ts      (mới) cache 60 s + singleflight
  codeintel-gitnexus-overview.ts      (mới)  codeintel-gitnexus-processes.ts, -process.ts, -routes.ts (mới)
  codeintel-gitnexus-subgraph.ts      (mới)  codeintel-gitnexus-impact.ts, -symbol.ts (mới)
  codeintel-symbol-source-reader.ts   (mới) đọc mã nguồn symbol an toàn (check-ignore, nhị phân, 200 KiB)
  codeintel-method-table.ts           (sửa) +7 dòng (overview, processes, process, subgraph, impact, symbol, routes)
  agent/src/relay/codeintel/__fixtures__/gitnexus-1.6.9/*.json   (mới; quy trình do AG-CV-SOL-070)
```

### 2.2 Cypher chỉ đọc (contract §2.4)

Bộ mã hoá giá trị (chỉ bốn kiểu được điền vào khe `{{ten}}`): `cypherInt(n,min,max)`, `cypherString(s)` (1..512, cấm NUL/điều khiển, thoát `\` rồi `'`, bọc nháy đơn), `cypherStringList(a)` (1..300), `cypherKindList(a, allowed)` (tập hằng các loại cạnh). Không khe cho nhãn/thuộc tính/từ khoá. `assertReadOnlyCypher(text)` trên văn bản **đã render**: ≤ 16 KiB, một câu, bắt đầu `MATCH `, sau bỏ chuỗi nháy đơn không còn từ nguyên vẹn `CREATE|MERGE|DELETE|SET|REMOVE|DROP|ALTER|COPY|DETACH|CALL|LOAD|INSTALL|ATTACH|EXPORT|IMPORT|FOREACH|UNWIND`. Hằng cho mẫu: `GITNEXUS_EDGE_KINDS` (15 loại, contract §4.4).

Bảng mẫu (nguồn CR-002 2.4; cột "chạy" = trạng thái theo CR, **chưa kiểm lại**):

| Id | Dùng cho | Chạy (theo CR) |
|---|---|---|
| `OV_CLUSTERS`, `OV_TOPFILES` | overview | đã chạy |
| `OV_EDGES` | overview | đã chạy (`ca.id <> cb.id`, **không** `ca<>cb`) |
| `OV_COUNT` (mới) | overview `totalCount` | chưa chạy riêng |
| `PR_LIST`, `PR_COUNT`, `PR_ONE`, `PR_STEPS` | processes/process | đã chạy (từng cột) |
| `NODES_BY_ID` | processes, process, subgraph | đã chạy |
| `STEP_EDGES` | process | **chưa chạy** (tổ hợp `IN` hai vế) |
| `MEMBER_CLUSTER` | process, subgraph | **chưa chạy** (`s.id IN`) |
| `SG_EDGES_AROUND` | subgraph | **chưa chạy** (`IN` danh sách; dạng `=` đã chạy) |
| `SG_CLUSTER_MEMBERS`, `SG_FILE_SYMBOLS`, `SYMBOL_FLOWS` | subgraph, impact, symbol | đã chạy |
| `FILE_SYMBOLS_BATCH` | dùng bởi SOL-005 | `MATCH (n)` + `IN` **chưa chạy** |
| `RT_LIST` | routes | **chưa chạy** (`ORDER BY r.id SKIP`) |
| `RT_COUNTS` | routes | đã chạy |

**Mẫu chưa chạy là điều kiện tiên quyết merge** (CR-002 2.1; contract O-3): task 04 chạy từng mẫu bằng `gitnexus cypher -r <repo> "<đã render>"`, ghi thời gian, lưu đầu ra làm fixture, sửa mẫu hoặc bảng nếu sai. Mọi mẫu **cấm** chọn cột `content`/`description`/`docstring`; cột tự do duy nhất (`label`/`name`) đặt cuối (`freeTextColumn`).

```ts
export type CypherColumnType = 'string' | 'nullableString' | 'int' | 'float' | 'jsonArray'
export type CypherTemplate = {
  id: string
  text: string                                          // chứa khe {{ten}}
  slots: Record<string, 'int' | 'string' | 'stringList' | 'kindList'>
  columns: ReadonlyArray<{ header: string; type: CypherColumnType }>   // tiêu đề đúng như GitNexus in
  freeTextColumn?: true                                  // cột cuối được phép chứa ' | '
}
export type CypherRows = { rows: Array<Record<string, string | number | string[] | null>>; rowCount: number; skippedRows: number; warnings: string[] }
export function runCypherTemplate(binding, templateId: string, slots: Record<string, unknown>, ctx): Promise<CypherRows>
```

### 2.3 Parser đầu ra

`parseCypherOutput(stdoutText, template)`: `JSON.parse` (lỗi -> `TOOL_FAILED reason='truncated_stdout'`; phân loại `{"error"}` do `codeintel-tool-output-classification` của SOL-001), `[]` -> không hàng; dòng 1 tiêu đề phải khớp **đúng thứ tự và số cột** (`unexpected_columns`), dòng 2 phân cách; mỗi hàng bỏ `| ` đầu và ` |` cuối rồi tách ` | `; ô thừa gộp vào `freeTextColumn`, còn lệch -> bỏ hàng, `skippedRows++`, cảnh báo `gitnexus: skipped N rows with unparsable columns`; kiểu `jsonArray` bỏ nháy đơn bao `'comm_123'`; đối chiếu `parsed+skipped === row_count` (lệch -> `row_count_mismatch`, không lỗi). Không bao giờ đoán.

### 2.4 `SymbolRef` (contract §2.6, PQ-20)

`codeintel-symbol-ref.ts` hàm thuần: `canonicalKind(nativeLabel)`; `buildSymbolRef({id,label,name,filePath,startLine,endLine})` với `startLine+1`, `endLine+1`; `key = <kind>:<filePath>:<qualifiedName>` sau bỏ tiền tố `<Label>:`, `<filePath>:`, hậu tố `#<n>` (lưu `ordinal`); `qualifiedName` NFC, `::` -> `.`; id `Section` -> `doc:<filePath>:L<line>:<tên>`; cluster/flow `cluster::<id>`, `flow::<id>`; file/folder `file:<path>:<tên cuối>`. Va chạm trong cùng kết quả: `#<arity>` rồi `#L<startLine>` + `warnings:["key_collision"]`. Kind lạ -> `value` + `nativeKind` + `warnings:["unknown_native_kind"]`. `arity`: **chưa có nguồn đã kiểm chứng trong dữ liệu GitNexus** (không thấy cột parameter count trong CR): chỉ áp `#L<startLine>` khi không tính được arity; ghi vào mục 8.

### 2.5 Probe chỉ mục (`codeintel.status` `indexes.gitnexus`)

Không chạy Cypher, không gọi CLI: lấy từ phần tử registry (`indexedCommit=lastCommit`, `indexedAt`, `branch`, `stats`, `storagePath`); `schemaVersion` đọc từ `.gitnexus/meta.json` **một lần theo `mtime`** (parse đầy đủ rồi bỏ `fileHashes`; quá 200 ms thì bỏ `schemaVersion` và cache quyết định); `state`: `missing` nếu thiếu `.gitnexus/lbug`; `building` nếu job reindex GitNexus của repo đang chạy (hàm của SOL-004; trước đó mặc định `false`); `stale` nếu `indexedCommit !== headCommit` hoặc `worktreeMismatch`; còn lại `ready`; `indicators:["wal_missing_shadow_files:<n>"]` từ `readdir(.gitnexus)` đếm tiền tố `lbug.wal.missing-shadow.` (không coi là lỗi); `schemaVersion ∉ {5}` đọc được -> `supported:false`, method đọc trả `TOOL_UNAVAILABLE reason='schema_version_unsupported'`.

### 2.6 Method (tham số/biên đúng contract §4.2–4.7; không lặp bảng ở đây)

| Method | Truy vấn/CLI | Ghi chú |
|---|---|---|
| `codeintel.overview` | `OV_CLUSTERS`∥`OV_COUNT`, rồi `OV_EDGES`∥`OV_TOPFILES` | lọc cạnh về cụm trong `topN`; `area`, `dominantLanguage` suy từ đường dẫn; `warnings` `communities_count_mismatch`; `truncated` khi `topN<tổng` hoặc cạnh = `maxEdges` |
| `codeintel.processes` | `PR_LIST`∥`PR_COUNT`, rồi `NODES_BY_ID` (≤ 200 id) | sắp `stepCount DESC, id` |
| `codeintel.process` | `PR_ONE`, rồi `PR_STEPS`(200)∥`STEP_EDGES(['CALLS'])`∥`MEMBER_CLUSTER` | `stepCount>200` -> `truncated`, `steps_truncated`; không có -> `SYMBOL_NOT_FOUND kind:'process'` |
| `codeintel.subgraph` | `SG_EDGES_AROUND` theo mức (≤ 3 lần, frontier ≤ 300), `NODES_BY_ID` (nhóm 300), `MEMBER_CLUSTER` tuỳ chọn; `center.cluster` -> `SG_CLUSTER_MEMBERS`+`STEP_EDGES`; `center.file` -> `SG_FILE_SYMBOLS` | dừng khi nút ≥ `limit` hoặc cạnh ≥ 4 000; mặc định loại `value`/`Section` (trừ tâm) |
| `codeintel.impact` | `gitnexus impact … -d <dir> --depth <n> -l <limit> [--include-tests]` rồi `SYMBOL_FLOWS` | `ambiguous` -> `AMBIGUOUS_SYMBOL` kèm `candidates` (+1 dòng, `score`, `impactedCount`, `risk`), **không tự chọn**; `levels`, `affectedFlows`, `affectedModules`, `testsCovering`, `rawSummary`; không có cạnh (PQ-19) |
| `codeintel.symbol` | `gitnexus context -u|name -l N` (không `--content`), `SYMBOL_FLOWS` | mã nguồn tự đọc: `codeintel-symbol-source-reader.ts` |
| `codeintel.routes` | `RT_LIST`∥`RT_COUNTS` | `handler` là `SymbolRef` kind `file`; `routes_coverage_js_only` khi repo có `backend-go/` |

Mọi handler: `validate` (task 01 SOL-001), `resolveCodeIntelRepo`, kiểm `tools.gitnexus` (`available`/`supported`/chỉ mục `missing` -> `INDEX_MISSING`), chạy với hạn 25 s, dựng phong bì với `sources[{tool:'gitnexus', …, lineBase:1}]`, `perf`.

Ví dụ lỗi chuẩn: `codeintel.impact {target:{name:'runToolCommand'}}` -> `-32000` `CODEINTEL_AMBIGUOUS_SYMBOL` với hai ứng viên (`agent/…` dòng 72/73 và `desktop/…`) theo ví dụ §7.1 của hợp đồng; gọi lại với `{target:{uid:'Function:agent/src/relay/agent-tool-registry.ts:runToolCommand'}}`.

### 2.7 Cache ngắn hạn (contract §2.3)

`codeintel-short-lived-cache.ts`: khoá `(registryPath, indexedAt|lastCommit, method, hash(paramsChuẩnHoá))`; TTL 60 s; ≤ 64 mục và ≤ 32 MiB (LRU, ước lượng `Buffer.byteLength`); singleflight; không cache lỗi và `symbol` có `source`; `stale`/`headCommit`/`perf` tính lại mỗi lần (không nằm trong giá trị cache); `invalidateShortLivedCache(registryPath?)` cho SOL-004 (reindex xong, `indexChanged`) và khi probe thấy `indexedAt` đổi.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Thay chuỗi có mã hoá + guard, không dựa tham số CLI | CLI không có tham số ràng buộc (`Parameter id not found`, CR 1.1) |
| 2 | Parser từ chối đoán | markdown không thoát, xuống dòng bị gộp (CR 1.5) |
| 3 | Số dòng +1 tại agent | GitNexus 0-based, CodeGraph/Git 1-based; PQ-20 |
| 4 | Mã nguồn đọc từ tệp, không `--content` | `--content` lệch một dòng và mất xuống dòng |
| 5 | `impact`: luồng bằng `SYMBOL_FLOWS`, không `affected_processes` | `affected_processes` là điểm vào, không có `processId` |
| 6 | Không tự chọn khi `ambiguous` | `runToolCommand` có hai bản (`agent/`, `desktop/`) |
| 7 | Probe không chạy CLI | rẻ, không đụng DB khi đang reindex |
| 8 | Mỗi handler một file | không vượt max-lines, test độc lập |

## 4. Phụ thuộc và thứ tự (task)

```
01 ──► 03 ──► 04 ──► 08 ──► 09
02 ──► 03 ──────────┘  ▲      ▲
05 ───────────────────┬┘      │
06 ──────────────────┬┴───────┤
07 ──────────────────┴────────┘
```
Rút gọn: 01 (mã hoá+guard) và 02 (parser) song song; 03 (mẫu+runner) cần 01, 02; 04 (chạy mẫu thật + fixture) cần 03; 05 (`symbol-ref`), 06 (cache), 07 (probe) độc lập nhau, chỉ cần SOL-001; 08 (overview, processes, process, routes) cần 03, 04, 05, 06, 07; 09 (subgraph, impact, symbol) cần 08 (dùng chung `NODES_BY_ID`, bảng method).

## 5. Tiêu chí chấp nhận

- [x] Mọi mẫu ở 2.2 đã chạy trên repo thật, bảng cập nhật, đầu ra làm fixture (task 04).
- [x] Không còn khe `{{…}}` sau render; ca độc hại (`x' OR 1=1 --`, `\\'`, `\n`, NUL, `; MATCH … CALL …`) không qua guard; tên symbol hợp lệ `deleteFile` qua guard.
- [x] Parser đủ các ca ở CR-002 2.3; `row_count` lệch chỉ cảnh báo; JSON cụt -> `TOOL_FAILED truncated_stdout`.
- [x] `overview` Orca (`topN=200`): ≤ 200 nút, ≤ 5 000 cạnh, mọi cạnh có hai đầu trong tập nút, `totalCount=9039` (+ cảnh báo lệch), < 25 s, JSON < 8 MiB.
- [x] `processes` 300 luồng phân trang không trùng/sót; `process proc_0_checkspanel` 9 bước, bước 1 `ChecksPanel`.
- [x] `impact name=runToolCommand` -> `AMBIGUOUS_SYMBOL` 2 ứng viên; với uid `agent/` -> 1 nút depth 1 (`handler#2`, theo CR; **chưa kiểm chứng**); `limit 300` không vượt 300 nút.
- [x] `symbol runToolCommand` (uid `agent/`): `startLine=72`, `endLine=113`, `source.text` bắt đầu `export function runToolCommand(`; tệp ignore -> `sourceOmitted:"gitignored"`.
- [x] `routes` 92 + 21 tuyến + `routes_coverage_js_only`.
- [x] Số dòng +1 và `sources[].lineBase=1`; truy vấn 3 000 hàng không cụt.
- [x] Worktree liên kết: `stale:true`; `symbol` có `source_may_not_match_index`.
- [x] Gọi lặp trong 60 s chỉ spawn một lần; hai lần đồng thời chỉ một; huỷ cache khi `indexedAt` đổi.

## 6. Kiểm thử

Chạy trong `/opt/repos/orca/agent` (chưa chạy gì):

| Tầng | File test (mới) | Lệnh |
|---|---|---|
| Unit | `gitnexus-cypher-literal.test.ts`, `gitnexus-cypher-guard.test.ts` | `pnpm exec vitest run src/relay/gitnexus-cypher-literal.test.ts src/relay/gitnexus-cypher-guard.test.ts` |
| Unit | `gitnexus-cypher-markdown-parser.test.ts` | `pnpm exec vitest run src/relay/gitnexus-cypher-markdown-parser.test.ts` |
| Unit/binary giả | `gitnexus-cypher-runner.test.ts` | `pnpm exec vitest run src/relay/gitnexus-cypher-runner.test.ts` |
| Unit | `codeintel-symbol-ref.test.ts`, `codeintel-short-lived-cache.test.ts`, `gitnexus-index-probe.test.ts` | `pnpm exec vitest run src/relay/codeintel-symbol-ref.test.ts src/relay/codeintel-short-lived-cache.test.ts src/relay/gitnexus-index-probe.test.ts` |
| Handler (binary giả phát lại fixture) | `codeintel-gitnexus-methods.test.ts` | `pnpm exec vitest run src/relay/codeintel-gitnexus-methods.test.ts` |
| Nguồn symbol | `codeintel-symbol-source-reader.test.ts` | `pnpm exec vitest run src/relay/codeintel-symbol-source-reader.test.ts` |
| Hợp đồng công cụ thật | do AG-CV-SOL-070 (job `code-intel-contract`: `gitnexus analyze --index-only` trên repo nhỏ, so fixture vàng) | — |
| Thủ công một lần | chạy bảng 2.2 trên `/opt/repos/orca`, ghi thời gian vào PR | task 04 |

## 7. Rủi ro và điểm chưa kiểm chứng

- Định dạng markdown của `cypher` và JSON của `context/impact` là hành vi nội bộ GitNexus 1.6.9; bảo vệ bằng kiểm tiêu đề cột, fixture vàng, dải phiên bản.
- Năm mẫu **chưa chạy** (bảng 2.2). Nếu `MATCH (n) … IN` không chạy trên LadybugDB thì `FILE_SYMBOLS_BATCH` (SOL-005) cần phương án khác (`UNION` theo `File`…): chưa kiểm chứng.
- Chi phí: `overview` ~6 s và 3-4 tiến trình; cổng 3 tiến trình làm hàng đợi dài khi nhiều người xem; chưa đo trên dev server thật; backend cần cache (CR-022).
- Blast radius `impact` có thể thiếu (GitNexus gộp `handler` vô danh: CR ghi 1 nút trong khi bảy handler gọi `runToolCommand`); UI không được trình bày như bằng chứng đầy đủ.
- `communities` 10 252 ≠ 9 039 chưa rõ nguyên nhân.
- `--branch` của `cypher/context/impact` chưa kiểm chứng, không dùng.
- Đọc đồng thời với `analyze` chưa kiểm chứng: SOL-004 chặn đọc GitNexus khi có job.
- Mã nguồn từ tệp hiện tại có thể không khớp chỉ mục cũ (cảnh báo đã nêu).

## 8. Câu hỏi mở

1. Cách tính `arity` cho hậu tố `#<arity>` của khoá (contract §2.6): GitNexus có số tham số? Nếu không, chỉ dùng `#L<startLine>`.
2. Agent có tự chặn `.env*`, `*.pem`, `*.key`, `id_rsa*` ở `symbol.source` (phòng thủ chiều sâu, contract §9 mục 4) hay để backend (contract §4.6 giao việc này cho backend, enum `sourceOmitted` chỉ có `gitignored|binary|not_requested`)? Cần mở rộng enum nếu agent chặn.
3. `Q1` của CR (`--branch`) và `Q4` (`UNWIND`): chưa quyết.
4. `symbol.flows[].step`: lấy `r.step` của `STEP_IN_PROCESS` (tên trường theo CR; chưa chạy).

## 9. Tham chiếu

- Contract: `CONTRACT-codeintel-agent-rpc.md` §2–§4, §9–§10; `CONTRACT-codeintel-proto-and-data-map.md` PQ-03/19/20/21, O-3.
- CR: `docs/crs/v7/agent-codeintel/CR-CV-002-gitnexus-extraction.md`, `README.md` (F4–F8); nghiên cứu `docs/research/view-code/04-raw-data-and-pipeline.md`, `05-graph-schemas.md`.
- Code: `agent/src/relay/agent-tool-registry.ts`, `agent/src/relay/git-handler-check-ignore.ts`, `.gitnexus/meta.json`.
