# CR-CV-002 — Trích xuất GitNexus: status, overview, processes, process, subgraph, impact, symbol, routes

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-002 |
| **Tên** | Handler `codeintel.*` đọc dữ liệu từ GitNexus: mẫu Cypher chỉ-đọc, parser đầu ra markdown của `cypher`, xử lý `ambiguous`, cắt giới hạn, cache ngắn hạn |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-001 |
| **Mở khoá** | CR-CV-004 (đọc `lastCommit`), CR-CV-005 (ánh xạ hunk → symbol bằng Cypher), CR-CV-021 (collector Go), CR-CV-070 (fixture vàng) |
| **Tác động** | `agent/src/relay/codeintel-method-table.ts` (thêm 7 dòng), các file mới `agent/src/relay/gitnexus-*.ts`, `agent/src/relay/codeintel-symbol-ref.ts`, `agent/src/relay/codeintel-short-lived-cache.ts` (liệt kê ở 2.1) |

---

## 1. Bối cảnh và vấn đề

Các số liệu và hình dạng đầu ra dưới đây do người soạn **chạy thật** `gitnexus 1.6.9` trên repo `/opt/repos/orca` ngày 2026-10-05 (chỉ lệnh đọc). Chúng khác với giả định của bộ nghiên cứu ở nhiều điểm, nên đây là nền để triển khai.

1. **CLI `cypher` không có tham số ràng buộc.** `gitnexus cypher --help` chỉ có `-r`, `--branch`, `-l`. Truy vấn có `$id` trả `{"error":"Parameter id not found."}`. "Mẫu có tham số" (README v7 mục 6) vì vậy phải là **thay thế chuỗi phía agent có mã hoá chặt**. Đã chứng minh rủi ro: `WHERE f.name = 'x' OR 1=1` chạy được và trả 80 348 hàng khi ghép chuỗi thô; `'it\'s'` (thoát `\'`) được parse đúng.
2. **GitNexus tự chặn lệnh ghi**: `CREATE ...` trả `{"error":"Write operations (CREATE, DELETE, SET, MERGE, REMOVE, DROP, ALTER, COPY, DETACH) are not allowed. The knowledge graph is read-only."}` (đã chạy). Danh sách này **không có `CALL`, `LOAD`, `INSTALL`, `ATTACH`, `EXPORT`**; lớp bảo vệ phía agent phải bao phủ thêm.
3. **Lỗi đi qua stdout với exit code 0.** Truy vấn sai (`Table Nope does not exist`), ghi bị chặn, tham số thiếu, symbol không tồn tại (`context`: `{"error":"Symbol 'X' not found"}`) đều thoát mã 0. Thiếu `-r` thì Node ném lỗi ra stderr.
4. **Ba dạng đầu ra của `cypher`:** `{"markdown": "...", "row_count": n}`; `[]` (tập rỗng, **không** có `markdown`); `{"error": "..."}`.
5. **Markdown không thoát ký tự.** Ô chứa ` | ` không được thoát (đã tạo ca: nội dung `Section` chứa bảng markdown, `s.content` trả `... | bug | found by | seeds | ...` nguyên văn; đếm `\|` bằng 0). Xuống dòng trong ô bị **gộp thành dấu cách** (nội dung `Section` đọc lại thành một dòng liền). Ô rỗng biểu diễn cả chuỗi rỗng lẫn `null` (cột `r.method` của route trống). Mảng hiển thị là chuỗi JSON: `["'comm_7339'","'comm_7395'"]` (phần tử có nháy đơn thừa); mảng rỗng là `[]`.
6. **Stdout bị cắt khi đi qua pipe** (65 536 byte với pipe shell; 146 176 byte với `spawn` pipe; xem CR-CV-001 mục 1.9). Mọi truy vấn trong CR này **bắt buộc** chạy qua cơ chế ghi tệp tạm của `runCodeIntelTool`.
7. **Số dòng của GitNexus là 0-based.** `runToolCommand` thật nằm ở dòng 72-113 (1-based, `grep -n`); GitNexus báo `startLine: 71, endLine: 112` (qua `cypher`, `context`, `impact`); `resolveToolBinary` thật ở 54-66, GitNexus báo 53-65. CodeGraph báo 72-113 (1-based, đúng). Ngoài ra `context --content` trả nội dung **lệch một dòng** (bắt đầu bằng `}` của hàm trước, ví dụ `makeError` GitNexus `startLine 407`, thật là dòng 408) và đã gộp xuống dòng, nên **không dùng `--content`** để lấy mã nguồn.
8. **Hình dạng JSON không đồng nhất giữa lệnh:** `context` dùng `uid`/`kind`/`startLine`; `impact` dùng `id`/`type` cho `target` và `id`/`name`/`filePath`/`relationType`/`confidence`/`processes` cho từng nút (không có `kind`, `startLine`); ứng viên của `ambiguous` dùng `uid`/`kind`/`line`/`score` (và thêm `impactedCount`, `risk`, `direct` ở `impact`). Mọi lệnh đều có `epistemic: "exact"` ở các ví dụ đã thấy (ý nghĩa chưa kiểm chứng).
9. **`impact` mặc định rộng.** `getConnectionId` (upstream, depth 2) cho 155 nút, `risk: CRITICAL`; `-l` giới hạn số nút **mỗi mức** và cả `affected_processes/modules`; `pagination.truncated: true` khi bị cắt. `affected_processes[]` là danh sách **symbol điểm vào** (`name,type,filePath,affected_process_count,total_hits,earliest_broken_step`), không có `processId`.
10. **Chi phí:** mỗi lần gọi ~1,6-1,8 s (mở DB 1,6 GB); truy vấn tổng hợp cạnh giữa cụm (`ORDER BY w DESC LIMIT 5000`, hai `MATCH` + `MEMBER_OF`) mất 4,4-4,5 s và trả 5 000 hàng (149-189 KB markdown); top-file cho 200 cụm: 4 178 hàng, 1,9 s. Ba lệnh `cypher` song song cùng chạy được.
11. **Dữ liệu:** `Community` 9 039 (khớp `count(*)`), trong khi `.gitnexus/meta.json` và registry ghi `communities: 10252` (chưa rõ nguyên nhân; số thật dùng cho UI là số đếm trong đồ thị). Route chỉ có 92 nút (Express/Expo Router), **không có route Go**; tất cả `HANDLES_ROUTE`/`FETCHES` đều trỏ tới nút `File` (không phải symbol), `handlerSymbolId` rỗng.
12. **Chỉ mục có thể cũ.** `gitnexus status` trong repo này: `Indexed commit: d819812`, `Current commit: 1b0c760`, `⚠️ stale`. Bản trạng thái chỉ in văn bản; dữ liệu có cấu trúc nằm ở `~/.gitnexus/registry.json` (CR-CV-001 1.4) và `.gitnexus/meta.json` (2,8 MB vì chứa `fileHashes`; không đọc cả tệp này mỗi lần).

## 2. Giải pháp đề xuất

### 2.1 File mới (`agent/src/relay/`)

| File | Vai trò |
|---|---|
| `gitnexus-cypher-literal.ts` | Bộ mã hoá giá trị: `cypherString`, `cypherInt`, `cypherStringList`, `cypherKindList` (2.2) |
| `gitnexus-cypher-guard.ts` | `assertReadOnlyCypher(text)` (2.2) |
| `gitnexus-cypher-templates.ts` | Bảng mẫu: `id`, `text` có khe `{{tên}}`, `columns` (tên + kiểu) (2.4) |
| `gitnexus-cypher-markdown-parser.ts` | `parseCypherOutput(stdoutText, columns)` → `{ rows, rowCount, warnings }` (2.3) |
| `gitnexus-cypher-runner.ts` | `runCypherTemplate(binding, templateId, slots, opts)`: render → guard → `runCodeIntelTool` → parse; quy lỗi (1.3, 1.4) |
| `gitnexus-index-probe.ts` | Probe chỉ mục cho `codeintel.status` (2.5) |
| `codeintel-gitnexus-overview.ts`, `-processes.ts`, `-process.ts`, `-subgraph.ts`, `-impact.ts`, `-symbol.ts`, `-routes.ts` | Handler từng method (2.6) |
| `codeintel-symbol-ref.ts` | Hàm thuần (không I/O): bảng kind, dựng `SymbolRef`, `key`. Phần GitNexus ở đây; CR-CV-003 thêm phần CodeGraph (2.7) |
| `codeintel-short-lived-cache.ts` | Cache bộ nhớ ngắn hạn + singleflight (2.8) |

Test (mới), tên cụ thể: `gitnexus-cypher-literal.test.ts`, `gitnexus-cypher-guard.test.ts`, `gitnexus-cypher-markdown-parser.test.ts`, `gitnexus-cypher-runner.test.ts`, `gitnexus-index-probe.test.ts`, `codeintel-gitnexus-methods.test.ts` (các handler, binary giả), `codeintel-symbol-ref.test.ts`, `codeintel-short-lived-cache.test.ts`. Fixture đầu ra thật đặt ở `agent/src/relay/__fixtures__/gitnexus-1.6.9/*.json` (CR-CV-070 sở hữu quy trình cập nhật).

### 2.2 Cypher chỉ-đọc, "có tham số" bằng thay thế có mã hoá

Mẫu là chuỗi hằng trong mã nguồn, khe dạng `{{ten}}`. Chỉ có **bốn** kiểu giá trị được phép điền:

| Bộ mã hoá | Quy tắc |
|---|---|
| `cypherInt(n, min, max)` | số nguyên hữu hạn trong `[min,max]`, in dạng thập phân; ngoài biên → `CODEINTEL_INVALID_PARAMS` |
| `cypherString(s)` | độ dài 1..512; cấm NUL và mọi ký tự điều khiển (kể cả `\n`, `\r`, `\t`); thoát `\` → `\\` và `'` → `\'`; bọc nháy đơn. Chuỗi Unicode (id `Section` có chữ tiếng Việt, gạch dài) được giữ nguyên |
| `cypherStringList(a)` | 1..300 phần tử, mỗi phần tử qua `cypherString`; in `['a','b']` |
| `cypherKindList(a, allowed)` | mỗi phần tử phải thuộc tập hằng (ví dụ `CALLS`, `IMPORTS`, `ACCESSES`, `EXTENDS`, `IMPLEMENTS`, `MEMBER_OF`, `STEP_IN_PROCESS`, `DEFINES`, `HANDLES_ROUTE`, `FETCHES`, `HAS_METHOD`, `HAS_PROPERTY`, `METHOD_IMPLEMENTS`, `METHOD_OVERRIDES`, `CONTAINS`) |

Không có đường nào ghép chuỗi người dùng vào chỗ khác ngoài các khe này (không có khe cho tên nhãn, tên thuộc tính, hay từ khoá).

`assertReadOnlyCypher(text)` chạy trên văn bản **đã render**, là lớp phòng thủ thứ hai (GitNexus cũng tự chặn ghi, 1.2):

1. Độ dài ≤ 16 KiB; đúng **một** câu lệnh (không có `;` ngoài chuỗi).
2. Loại bỏ mọi chuỗi trong nháy đơn (xử lý `\'` và `\\`), rồi từ chối nếu còn chứa từ nguyên vẹn (không phân biệt hoa/thường) thuộc `CREATE|MERGE|DELETE|SET|REMOVE|DROP|ALTER|COPY|DETACH|CALL|LOAD|INSTALL|ATTACH|EXPORT|IMPORT|FOREACH|UNWIND`. (`UNWIND` bị cấm vì mẫu hiện không cần; thêm khi có nhu cầu và đã chạy thử.)
3. Phải bắt đầu bằng `MATCH ` (mọi mẫu bắt đầu như vậy).

Test: bảng ca độc hại (`x' OR 1=1 --`, `\\'`, xuống dòng, NUL, `'; MATCH ... CALL ...`, từ khoá trong chuỗi hợp lệ như tên symbol `deleteFile`) và test phản chứng tĩnh: mỗi mẫu trong bảng qua guard; không mẫu nào chứa `{{` còn sót sau khi render.

### 2.3 Parser đầu ra `cypher`

`parseCypherOutput(stdoutText, columns)` trong `gitnexus-cypher-markdown-parser.ts`:

1. `JSON.parse(stdoutText)`. Lỗi parse → `CODEINTEL_TOOL_FAILED (reason = "truncated_stdout")` (xem 1.6).
2. Nếu là mảng rỗng `[]` → `rows: []`. Nếu có `error` (string) → phân loại: chứa `not found`/`does not exist` → `CODEINTEL_INVALID_PARAMS (reason = "not_found")`; chứa `Write operations` → `CODEINTEL_TOOL_FAILED (reason = "write_blocked")` (lỗi lập trình, ghi log mức error); còn lại → `CODEINTEL_TOOL_FAILED`. Nếu có `markdown` và `row_count` → tiếp tục. Dạng khác → `CODEINTEL_TOOL_FAILED (reason = "unknown_shape")`.
3. Tách theo `\n`: dòng 1 là tiêu đề, dòng 2 là dòng phân cách `| --- |...`, các dòng sau là dữ liệu. Tiêu đề phải khớp **đúng thứ tự và số cột** của `columns` (đối chiếu tên, vì mẫu luôn dùng bí danh `AS` hoặc tên đầy đủ `a.id`); lệch → `CODEINTEL_TOOL_FAILED (reason = "unexpected_columns")` (phát hiện sớm khi GitNexus đổi định dạng).
4. Mỗi dòng: bỏ `| ` đầu và ` |` cuối, tách bằng ` | `. Nếu số ô > số cột và mẫu khai báo `freeTextColumn` (tối đa **một** cột, **luôn ở cuối**), gộp các ô thừa vào cột đó bằng ` | `. Nếu vẫn lệch → bỏ dòng, tăng `skippedRows`, thêm `warnings` (`"gitnexus: skipped N rows with unparsable columns"`). Không bao giờ đoán.
5. Kiểu ô: `string` (giữ nguyên, không trim bên trong), `int`, `float`, `jsonArray` (parse JSON; mỗi phần tử string bỏ cặp nháy đơn bao ngoài `'comm_123'` → `comm_123`; lỗi parse → `[]` kèm cảnh báo), `nullableString` (ô rỗng → `null`).
6. Đối chiếu: số dòng đã parse + `skippedRows` phải bằng `row_count`; lệch → thêm cảnh báo `row_count_mismatch` (không lỗi).
7. Mọi mẫu **cấm chọn cột `content`/`description`/`docstring`** (văn bản dài, xuống dòng bị gộp, có thể chứa ` | `). Cột tự do duy nhất được phép là tên/nhãn (`label`) đặt ở cuối.

Bộ test của parser dùng chính các chuỗi thật: tiêu đề + phân cách; ô `["'comm_7339'","'comm_7395'"]`; id `Section:CLAUDE.md:L23:GitNexus — Code Intelligence`; ô rỗng; ô chứa ` | ` ở cột cuối (gộp) và ở cột giữa (bỏ dòng + cảnh báo); `row_count` lệch; `[]`; `{"error"}`; JSON cụt.

### 2.4 Mẫu Cypher (đã chạy thử trên repo Orca trừ khi ghi khác)

Quy ước: nút nhận diện bằng `id` (chuỗi dạng `Kind:đường/dẫn:tên[#n]`); `label(n)` cho loại nút (đã chạy, trả `Method`, `Function`, ...); cạnh là `CodeRelation` với thuộc tính `type`, `confidence`, `reason`; luôn `RETURN` cột phẳng.

| Mã mẫu | Văn bản | Kiểm chứng |
|---|---|---|
| `OV_CLUSTERS` | `MATCH (c:Community) RETURN c.id, c.symbolCount, c.cohesion, c.keywords, c.label ORDER BY c.symbolCount DESC, c.id LIMIT {{topN}}` | cột `c.id/label/symbolCount/cohesion` đã chạy (1,6 s); thêm `keywords` đã thấy dạng `[]`; `ORDER BY ..., c.id` đã chạy |
| `OV_EDGES` | `MATCH (a)-[r:CodeRelation]->(b) WHERE r.type IN {{kinds}} MATCH (a)-[:CodeRelation {type:'MEMBER_OF'}]->(ca:Community), (b)-[:CodeRelation {type:'MEMBER_OF'}]->(cb:Community) WHERE ca.id <> cb.id RETURN ca.id, cb.id, r.type, count(*) AS w ORDER BY w DESC LIMIT {{maxEdges}}` | chạy với `kinds=['CALLS','IMPORTS']`, `LIMIT 5000`: 5 000 hàng, 4,5 s. **Chú ý:** bản trong `04-raw-data-and-pipeline.md` dùng `WHERE ca<>cb` (so sánh nút); bản đã chạy dùng `ca.id <> cb.id` |
| `OV_TOPFILES` | `MATCH (s)-[m:CodeRelation {type:'MEMBER_OF'}]->(c:Community) WHERE c.id IN {{ids}} RETURN c.id, s.filePath, count(*) AS n` | chạy với 200 id: 4 178 hàng, 1,9 s, 361 KB markdown. **Không** dùng `LIMIT` toàn cục (sẽ lệch về cụm lớn); agent gom top-3 tệp mỗi cụm sau khi nhận |
| `PR_LIST` | `MATCH (p:Process) RETURN p.id, p.processType, p.stepCount, p.communities, p.entryPointId, p.terminalId, p.label ORDER BY p.stepCount DESC, p.id SKIP {{offset}} LIMIT {{limit}}` | `ORDER BY p.stepCount DESC, p.id SKIP .. LIMIT ..` đã chạy; các cột đã chạy riêng lẻ |
| `PR_COUNT` | `MATCH (p:Process) RETURN count(*) AS n` | đã chạy kiểu tương tự trên `Community` |
| `PR_ONE` | `MATCH (p:Process) WHERE p.id = {{id}} RETURN p.id, p.processType, p.stepCount, p.communities, p.entryPointId, p.terminalId, p.label` | cột đã chạy; điều kiện `p.id = '...'` đã chạy ở `PR_STEPS` |
| `PR_STEPS` | `MATCH (s)-[r:CodeRelation {type:'STEP_IN_PROCESS'}]->(p:Process) WHERE p.id = {{id}} RETURN s.id, s.name, s.filePath, label(s), s.startLine, s.endLine, r.step ORDER BY r.step LIMIT {{limit}}` | đã chạy (cột `s.id,s.name,s.filePath,r.step`); `label(s)`, `startLine/endLine` đã chạy ở mẫu `NODES_BY_ID` |
| `NODES_BY_ID` | `MATCH (n) WHERE n.id IN {{ids}} RETURN n.id, n.filePath, label(n), n.startLine, n.endLine, n.name` | đã chạy (1,7 s); `n.name` đặt cuối vì là cột tự do |
| `STEP_EDGES` | `MATCH (a)-[e:CodeRelation]->(b) WHERE a.id IN {{ids}} AND b.id IN {{ids}} AND e.type IN {{kinds}} RETURN a.id, b.id, e.type, e.confidence, e.reason` | cú pháp `IN` hai vế và `e.confidence/e.reason` đã chạy riêng lẻ; **tổ hợp chưa chạy** |
| `MEMBER_CLUSTER` | `MATCH (s)-[:CodeRelation {type:'MEMBER_OF'}]->(c:Community) WHERE s.id IN {{ids}} RETURN s.id, c.id` | `MEMBER_OF` đã chạy; điều kiện theo `s.id IN` chưa chạy |
| `SG_EDGES_AROUND` | `MATCH (a)-[e:CodeRelation]->(b) WHERE (a.id IN {{frontier}} OR b.id IN {{frontier}}) AND e.type IN {{kinds}} RETURN a.id, b.id, e.type, e.confidence, e.reason LIMIT {{limit}}` | dạng `a.id = X OR b.id = X` đã chạy (1,75 s, trả cả hai hướng); dạng `IN` danh sách chưa chạy |
| `SG_CLUSTER_MEMBERS` | `MATCH (s)-[:CodeRelation {type:'MEMBER_OF'}]->(c:Community) WHERE c.id = {{id}} RETURN s.id, s.filePath, label(s), s.startLine, s.endLine, s.name LIMIT {{limit}}` | đã chạy với `LIMIT 3` (1,7 s) |
| `SG_FILE_SYMBOLS` | `MATCH (n) WHERE n.filePath = {{path}} RETURN n.id, label(n), n.startLine, n.endLine, n.name ORDER BY n.startLine LIMIT {{limit}}` | dạng `MATCH (n) WHERE n.filePath = ...` đã chạy trên `agent/src/relay/context.ts` (trả `Function`, `Method`, `File`, `Class`). **Không dùng** `File -DEFINES-> s`: với `context.ts` nó bỏ sót `Method:...RelayContext.registerRoot#1` (phương thức chỉ nối với lớp qua `HAS_METHOD`); đã chạy cả hai để so sánh |
| `FILE_SYMBOLS_BATCH` (cho CR-CV-005) | `MATCH (n) WHERE n.filePath IN {{paths}} RETURN n.filePath, n.id, label(n), n.startLine, n.endLine, n.name` | dạng `= 'x'` đã chạy; dạng `IN` danh sách đã chạy với `DEFINES` (1,65 s) nhưng **chưa** chạy với `MATCH (n)`; nút `File` có `startLine`/`endLine` rỗng |
| `SYMBOL_FLOWS` | `MATCH (s)-[r:CodeRelation {type:'STEP_IN_PROCESS'}]->(p:Process) WHERE s.id IN {{ids}} RETURN s.id, p.id, p.stepCount, r.step, p.label` | đã chạy (1,64 s) |
| `RT_LIST` | `MATCH (h)-[e:CodeRelation]->(r:Route) WHERE e.type IN ['HANDLES_ROUTE','FETCHES'] RETURN r.id, r.method, r.filePath, e.type, h.id, e.confidence, r.name ORDER BY r.id SKIP {{offset}} LIMIT {{limit}}` | phần `RETURN`/`WHERE` đã chạy; `ORDER BY r.id SKIP` chưa chạy riêng (nhưng cùng cú pháp với `PR_LIST`) |
| `RT_COUNTS` | `MATCH (h)-[e:CodeRelation]->(r:Route) WHERE e.type IN ['HANDLES_ROUTE','FETCHES'] RETURN e.type, label(h), count(*) AS n` | đã chạy: `HANDLES_ROUTE`/`File` 92, `FETCHES`/`File` 21 |

Trong tên cột render ra, GitNexus dùng chính biểu thức làm tiêu đề (`s.id`, `kind` khi có `AS`); `columns` của mỗi mẫu ghi đúng tiêu đề đó.

Mọi mẫu `chưa chạy` ở trên là **điều kiện tiên quyết để merge**: người triển khai chạy từng mẫu bằng `gitnexus cypher -r <repo> "<mẫu đã render>"`, lưu đầu ra làm fixture, và sửa mẫu hoặc cập nhật bảng này nếu sai.

### 2.5 `status`: probe chỉ mục GitNexus

`gitnexus-index-probe.ts` đăng ký probe với `codeintel-status.ts` (CR-CV-001). Không chạy Cypher, không gọi CLI:

```jsonc
{ "state": "ready|stale|missing|building|unknown",
  "indexedCommit": "d8198127b6bd14a3bf02dda1762ee13b7f3848ed",   // registry.lastCommit
  "indexedAt": "2026-10-05T05:55:17.289Z",                       // registry.indexedAt
  "branch": "main",
  "stats": { "files": 20174, "nodes": 247556, "edges": 644157, "communities": 10252, "processes": 300 },
  "schemaVersion": 5,                                            // chỉ có trong meta.json; xem dưới
  "storagePath": "/opt/repos/orca/.gitnexus" }
```

- Nguồn chính là phần tử registry (nhỏ). `schemaVersion` chỉ có trong `.gitnexus/meta.json` (2,8 MB): đọc **một lần** khi `mtime` thay đổi và chỉ lấy khoá cần thiết bằng parse đầy đủ rồi bỏ phần `fileHashes` (chưa đo chi phí parse; cache theo `mtime`). Nếu quá 200 ms thì bỏ `schemaVersion`.
- `state`: `missing` nếu `.gitnexus/lbug` không tồn tại; `building` nếu job reindex GitNexus đang chạy (CR-CV-004); `stale` nếu `indexedCommit !== headCommit` hoặc `worktreeMismatch`; còn lại `ready`.
- Khi có tệp `lbug.wal.missing-shadow.*` (4 tệp trong `.gitnexus/` của repo này, từ 2026-08 đến 2026-09): **không** coi là lỗi; thêm `indicators: ["wal_missing_shadow_files:4"]` để vận hành biết (nguyên nhân tệp chưa rõ; xem CR-CV-004).
- Ghi chú về số `communities`: giữ số của registry trong `stats` (như README v7), nhưng `codeintel.overview` trả `totalCount` là số đếm thật (9 039) và thêm cảnh báo khi chênh lệch.

### 2.6 Các method

Tất cả: kiểm tham số bằng bảng bên dưới (tham số lạ bị từ chối), gọi `resolveCodeIntelRepo`, kiểm tra `tools.gitnexus` có/hỗ trợ và chỉ mục (`missing` → `CODEINTEL_INDEX_MISSING`), chạy qua `runCypherTemplate`/CLI với hạn mức của CR-CV-001, dựng phần đầu chung.

**`codeintel.overview`**

| Tham số | Kiểu | Mặc định | Biên |
|---|---|---|---|
| `topN` | int | 200 | 1..500 |
| `maxEdges` | int | 5000 | 1..5000 |
| `edgeKinds` | string[] | `["CALLS","IMPORTS"]` | tập con của `CALLS, IMPORTS, ACCESSES, EXTENDS, IMPLEMENTS` |
| `withTopFiles` | bool | true | |

Chạy `OV_CLUSTERS` rồi `OV_EDGES` và `OV_TOPFILES` song song (2 tiến trình). Lọc cạnh về các cụm trong `topN`. `ClusterNode` theo 05 §2.1: `{id,label,symbolCount,cohesion,keywords[],topFiles[≤3],dominantLanguage,area}`; `area` = thành phần đường dẫn đầu của tệp chiếm đa số (ví dụ `backend-go`, `frontend`); `dominantLanguage` suy từ phần mở rộng (`.go`, `.ts`, `.tsx`, ...). `ClusterEdge`: `{from,to,weight,kinds:{CALLS:n,IMPORTS:n}}` (gộp theo cặp từ cột `r.type`). `totalCount` = tổng số `Community` (thêm một `count(*)`); `truncated` = `topN` < tổng hoặc số cạnh = `maxEdges`. Thời gian kỳ vọng 5-7 s; timeout phương thức 25 s.

```jsonc
{ "data": { "nodes": [ { "id": "comm_6117", "label": "Components", "symbolCount": 2108, "cohesion": 0.8607, "keywords": [],
                         "topFiles": ["frontend/src/renderer/src/components/PullRequestPage.tsx"], "dominantLanguage": "typescript", "area": "frontend" } ],
            "edges": [ { "from": "comm_6707", "to": "comm_6117", "weight": 338, "kinds": { "CALLS": 338 } } ] },
  "totalCount": 9039, "truncated": true }
```

Lưu ý: nhãn cụm của Orca là tên thư mục chung chung (`Components`, `Runtime`, `Usecase`; `keywords` đang rỗng, `enrichedBy` chưa kiểm tra). Nhiều cụm có thể trùng nhãn; UI/CR-CV-020 phải dùng `id`.

**`codeintel.processes`**: `limit` 1..100 (mặc định 50), `offset` ≥ 0. Chạy `PR_LIST` và `PR_COUNT` song song, rồi `NODES_BY_ID` cho `entryPointId`/`terminalId` của trang (≤ 200 id). `FlowSummary`: `{id,label,processType,stepCount,communities[],entry:SymbolRef,terminal:SymbolRef}`. Sắp xếp ổn định `stepCount DESC, id` để phân trang không lặp.

**`codeintel.process`**: `processId` (chuỗi 1..128, bắt buộc). `PR_ONE` rồi song song `PR_STEPS` (limit 200), `STEP_EDGES` (kinds `['CALLS']`) và `MEMBER_CLUSTER`. `FlowGraph`: `{flow, steps:[{step,symbol,cluster,filePath,startLine}], edges}`; không tìm thấy → `CODEINTEL_INVALID_PARAMS (reason = "not_found")`. `stepCount` > 200 → `truncated: true` kèm cảnh báo.

**`codeintel.subgraph`**

| Tham số | Kiểu | Mặc định | Biên |
|---|---|---|---|
| `center` | `{ "symbol": uid }` hoặc `{ "file": đường dẫn tương đối }` hoặc `{ "cluster": id }` | bắt buộc | đúng một khoá |
| `depth` | int | 1 | 1..3 (`cluster`/`file` bỏ qua, luôn 1) |
| `kinds` | string[] | `["CALLS","IMPORTS","EXTENDS","IMPLEMENTS"]` | tập con cạnh hợp lệ |
| `limit` | int (số nút) | 800 | 1..1500 |

Duyệt theo mức, mỗi mức một lần chạy `SG_EDGES_AROUND` (tối đa 3 lần x ~1,7 s): `frontier` ≤ 300 id mỗi lần (chia nhóm nếu nhiều hơn), dừng khi số nút ≥ `limit` hoặc số cạnh ≥ 4 000 (`truncated: true`). Sau cùng một lần `NODES_BY_ID` cho mọi nút (chia nhóm 300) và `MEMBER_CLUSTER` tuỳ chọn. Với `center.cluster`: lấy `SG_CLUSTER_MEMBERS` (limit) rồi cạnh giữa các thành viên bằng `STEP_EDGES`; với `center.file`: `SG_FILE_SYMBOLS` rồi như `symbol` cho từng symbol (tổng frontier bị chặn bởi `limit`). Kiểu nút mặc định **loại `Const`, `Variable`, `Property`, `Section`** (chiếm 36 507 + 34 424 + 26 345 + 1 111 trong 247 556 nút) trừ khi `center` là chính loại đó; lý do: gây nhiễu và là nguồn id có khoảng trắng/` | `. Kết quả `SymbolGraph` theo 05 §2.3 (`confidence`, `reason` giữ nguyên cho cạnh yếu).

**`codeintel.impact`**

| Tham số | Kiểu | Mặc định | Biên |
|---|---|---|---|
| `target` | `{ "uid": ... }` hoặc `{ "name": ..., "file"?: ..., "kind"?: ... }` | bắt buộc | |
| `direction` | `upstream` \| `downstream` | `upstream` | |
| `depth` | int | 2 | 1..3 |
| `limit` | int (tổng nút) | 300 | 1..300 |
| `includeTests` | bool | false | |

Chạy `gitnexus impact -r <path> [-u uid | name -f file --kind kind] -d <dir> --depth <n> -l <limit> [--include-tests]` rồi:
1. `status: "ambiguous"` (hoặc `candidates`) → `CODEINTEL_AMBIGUOUS_SYMBOL` với `data.candidates = [{uid,name,kind,filePath,line(+1),score,impactedCount,risk}]` (số dòng cộng 1, xem 1.7). Backend/UI chọn rồi gọi lại với `uid`. Không tự chọn ứng viên.
2. `{"error": "Target ... not found"}` → `CODEINTEL_INVALID_PARAMS (reason = "not_found")`.
3. Thành công: `ImpactGraph` `{target, direction, risk, impactedCount, levels:[{depth, symbols:[{symbol,via,confidence,direct}]}], affectedFlows, affectedClusters, testsCovering}`. `via` = `relationType`; `direct` = `depth === 1`. `risk` giữ nguyên chuỗi của GitNexus (`LOW|MEDIUM|HIGH|CRITICAL|UNKNOWN`). `affectedClusters` từ `affected_modules` (`{name,hits,impact}`; **không có id cụm**, chỉ tên). `affectedFlows`: không dùng `affected_processes` của CLI (đó là điểm vào, không phải process); thay vào đó chạy `SYMBOL_FLOWS` trên các nút bị ảnh hưởng (≤ 300 id, chia nhóm) và gom theo `p.id`. `testsCovering`: lọc các nút có `filePath` khớp mẫu test (`*.test.*`, `*.spec.*`, `_test.go`, thư mục `__tests__`, `tests/`) khi `includeTests` bật, ngược lại để trống và `warnings: ["tests_excluded"]`. Giữ nguyên `summary` gốc dưới `data.rawSummary` (`direct`, `processes_affected`, `modules_affected`).
4. Nút của `byDepth` không có `kind`/`startLine`: `kind` suy từ tiền tố `id`; số dòng chỉ bổ sung khi UI gọi `symbol` hoặc agent chạy `NODES_BY_ID` (làm cho tối đa 300 nút trong một lần).
`pagination.truncated` → `truncated: true`. Tổng nút = tổng các mức, cắt theo thứ tự mức tăng dần.

**`codeintel.symbol`**

| Tham số | Kiểu | Mặc định | Biên |
|---|---|---|---|
| `uid` hoặc `name`+`file` | string | bắt buộc đúng một dạng | |
| `includeSource` | bool | true | |
| `relationLimit` | int | 20 | 1..50 |

Chạy `gitnexus context -r <path> -u <uid> -l <relationLimit>` (hoặc `context <name> -f <file>`), **không** `--content`. `ambiguous` xử lý như `impact`. Từ JSON dựng `{symbol: SymbolRef, incoming:{calls[],imports[],...}, outgoing:{...}, flows:[...]}` (các khoá `incoming/outgoing` là bản đồ loại cạnh → danh sách `{uid,name,filePath}`; giữ nguyên tên loại cạnh viết thường như CLI). Mã nguồn: agent **tự đọc tệp** `workspaceRoot/filePath` theo dòng `[startLine+1 .. endLine+1]` (1-based, sau khi sửa lệch 0-based) bằng `fs`, bảo toàn xuống dòng, với các điều kiện: đường dẫn sau `realpath` nằm trong `workspaceRoot`; `git check-ignore -q -- <path>` (mã 0 = bị bỏ qua) → `source: null, sourceOmitted: "gitignored"`; tệp có byte NUL trong 8 KiB đầu → `sourceOmitted: "binary"`; tổng ≤ 200 KiB (cắt theo dòng, `source.truncated: true`). Khi `stale` hoặc `worktreeMismatch`, thêm `warnings: ["source_may_not_match_index"]` vì dòng của chỉ mục có thể không còn khớp tệp. Vì sao không dùng `--content`: 1.7.

**`codeintel.routes`**: `limit` 1..500 (mặc định 200), `offset` ≥ 0. Chạy `RT_LIST` và `RT_COUNTS`. `RouteMap`: `{routes:[{id,path,method,filePath,side,handler}], edges:[{route,handler,kind}]}`; `handler` là `SymbolRef` kiểu `file` (không phải symbol, 1.11); `side` = `server` cho `HANDLES_ROUTE`, `client` cho `FETCHES`. `middleware`, `responseKeys`, `errorKeys` hiện đều `[]` trong dữ liệu đã thấy (chưa thấy giá trị khác); chỉ trả nếu khác rỗng. Trong dữ liệu Orca, route chỉ phủ Express (`backend/src/main/admin`) và Expo Router (`mobile/app`), **không phủ gRPC/`wscompat` của Go**; phần đó thuộc CR-CV-032. Ghi vào `warnings: ["routes_coverage_js_only"]` khi repo có `backend-go/`.

### 2.7 `SymbolRef` và ánh xạ id

`codeintel-symbol-ref.ts` (hàm thuần):

| Chuẩn (`kind`) | GitNexus nhãn |
|---|---|
| `function` | `Function` |
| `method` | `Method`, `Constructor` |
| `type` | `Struct`, `Class`, `Interface`, `Enum` (giữ thêm `rawKind` gốc, trường bổ sung) |
| `value` | `Const`, `Variable`, `Property` |
| `file` / `folder` | `File` / `Folder` |
| `route` | `Route` |
| `cluster` | `Community` |
| `flow` | `Process` |
| `doc` | `Section` |

- Tên đủ điều kiện **không đồng nhất giữa hai công cụ**: GitNexus `RelayContext.registerRoot#1` (dấu `.`), CodeGraph `RelayContext::registerRoot` (`qualified_name` trong SQLite, dấu `::`). `qualifiedName` chuẩn hoá bằng cách đổi `::` thành `.`; áp dụng ở cả hai phía (CR-CV-003).
- `gitnexusId` = `id` nguyên văn. `filePath` lấy từ cột `filePath` (không phân tích id). `name` từ cột `name`. `qualifiedName` = phần sau `Kind:filePath:` của `id`, bỏ hậu tố `#<số>` (ví dụ `Method:...project.pb.go:ListReposResponse.GetRepos#0` → `ListReposResponse.GetRepos`; nhưng `Method:...:handler#2` → `handler`). Hậu tố `#n` giữ ở trường bổ sung `ordinal` vì hai `handler` cùng tệp sẽ trùng `key`.
- `key = "<kind>:<filePath>:<qualifiedName>"`; **trùng `key` trong cùng một kết quả** → thêm `#<ordinal>` và `warnings: ["key_collision"]` (CR-CV-020 quyết định hợp nhất hai nguồn).
- `startLine = startLine + 1`, `endLine = endLine + 1` cho mọi dữ liệu GitNexus (1.7); ghi `lineBase: 1` trong `sources[]` để backend biết đã chuẩn hoá.
- Hàm phải chịu id có khoảng trắng, dấu `:` trong tên (id `Section`) và chữ Unicode.

### 2.8 Cache ngắn hạn tại agent

Mục đích: gộp các lần gọi trùng liên tiếp (UI mở nhiều panel, backend gọi lặp) và bảo vệ DB khỏi bão truy vấn; **không** thay cache bền ở `code-intel-service` (CR-CV-022).

| Thuộc tính | Giá trị |
|---|---|
| Khoá | `(repoRegistryPath, indexedAt|lastCommit, method, hash(paramsChuẩnHoá))` |
| TTL | 60 s |
| Dung lượng | ≤ 64 mục và ≤ 32 MiB (loại bỏ LRU) |
| Singleflight | hai lần gọi cùng khoá cùng lúc dùng chung một lần chạy |
| Huỷ | khi probe chỉ mục thấy `indexedAt`/`lastCommit` đổi, khi `reindex` kết thúc (CR-CV-004), khi `codeintel.indexChanged` được phát |
| Không cache | lỗi; kết quả `symbol` có `source` (nội dung tệp đổi theo working tree) |
| `stale`/`headCommit` | tính lại mỗi lần (không nằm trong giá trị cache) |

Chưa đo tỷ lệ trúng; giá trị 60 s là giả định.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Thay thế chuỗi có mã hoá + guard thay vì mong đợi tham số của CLI | CLI không có tham số ràng buộc (1.1) |
| Chỉ bốn kiểu giá trị, không khe cho nhãn/thuộc tính | Loại hẳn lớp lỗi chèn cấu trúc truy vấn |
| Parser từ chối đoán, bỏ dòng và cảnh báo | Dữ liệu sai âm thầm nguy hiểm hơn thiếu dữ liệu; markdown không thoát (1.5) |
| Cấm cột `content`/`description` trong Cypher | Xuống dòng bị gộp, ` | ` không thoát; nguồn mã đọc từ tệp |
| Mã nguồn đọc từ tệp thay cho `context --content` | Nội dung lệch một dòng và mất xuống dòng (1.7) |
| Chuẩn hoá dòng +1 ngay ở agent | Mọi nơi tiêu thụ (backend, UI, CR-CV-005) dùng số dòng 1-based, như Git và CodeGraph |
| `impact`: không dùng `affected_processes` của CLI làm "luồng bị ảnh hưởng" | Đó là điểm vào, không có `processId` (1.9); luồng thật lấy bằng `SYMBOL_FLOWS` |
| Không tự chọn ứng viên khi `ambiguous` | Ví dụ `runToolCommand` có hai bản (`agent/` và `desktop/`) với mức ảnh hưởng khác nhau; chọn sai gây kết luận sai |
| Duyệt subgraph theo mức bằng nhiều lần truy vấn có `IN` | Tránh đường đi độ dài biến đổi chưa kiểm chứng trên LadybugDB, giữ ngân sách thời gian dự đoán được |
| Mặc định loại `Const/Variable/Property/Section` khỏi subgraph | Nhiễu và id không an toàn cho parser |

## 4. Tiêu chí chấp nhận

- [ ] Mỗi mẫu ở 2.4 đã được chạy trên repo thật; bảng được cập nhật (cột "Kiểm chứng") và đầu ra lưu làm fixture trong `__fixtures__/gitnexus-1.6.9/`.
- [ ] Không còn khe `{{...}}` sau render; test độc hại (2.2) không có ca nào qua guard; `OR 1=1`, xuống dòng, NUL, `;` bị từ chối trước khi spawn.
- [ ] Parser: đủ các ca ở 2.3; dòng lệch cột bị bỏ kèm `warnings`; `row_count` lệch → cảnh báo; JSON cụt → `CODEINTEL_TOOL_FAILED (truncated_stdout)`.
- [ ] `codeintel.overview` trên Orca (`topN=200`): ≤ 200 nút, ≤ 5 000 cạnh, mọi cạnh có hai đầu trong tập nút, `totalCount = 9039`, thời gian < 25 s trên máy khảo sát; kết quả JSON < 8 MiB.
- [ ] `codeintel.processes`: 300 luồng phân trang qua `offset` không trùng/không sót (so sánh với `count(*)`).
- [ ] `codeintel.process` với `proc_0_checkspanel` trả 9 bước theo thứ tự `r.step`, bước 1 là `ChecksPanel`.
- [ ] `codeintel.impact` với `name = "runToolCommand"` trả `CODEINTEL_AMBIGUOUS_SYMBOL` với 2 ứng viên (`agent/...` và `desktop/...`); với `uid` của bản `agent/` trả `ImpactGraph` có đúng 1 nút depth 1 (`handler#2`); `getConnectionId` depth 2 `limit 300` không vượt 300 nút.
- [ ] `codeintel.symbol` cho `runToolCommand` (uid `agent/`): `startLine = 72`, `endLine = 113`, `source.text` bắt đầu bằng dòng `export function runToolCommand(` và kết thúc bằng `}` của hàm; tệp trong `.gitignore` → `sourceOmitted`.
- [ ] `codeintel.routes` trả 92 tuyến (`HANDLES_ROUTE`) + 21 (`FETCHES`) và cảnh báo phạm vi JS.
- [ ] Mọi số dòng GitNexus trong kết quả đã +1; `sources[].lineBase = 1`.
- [ ] Truy vấn 3 000 hàng không bị cụt (CR-CV-001 1.9).
- [ ] Trên repo có `worktreeMismatch`, mọi method trả `stale: true` và cảnh báo; `symbol` có `source_may_not_match_index`.
- [ ] Gọi lặp cùng tham số trong 60 s chỉ spawn một tiến trình; hai lần gọi đồng thời cũng một; huỷ cache khi `indexedAt` đổi.
- [ ] Không file nào tên `helpers/utils/common/misc`; không `max-lines` disable.

## 5. Kiểm thử

- **Unit (Vitest, không cần GitNexus):** các file test ở 2.1; parser và encoder dùng chuỗi thật làm dữ liệu; runner dùng binary giả (script Node trong `PATH` tạm) phát lại fixture, kể cả trường hợp ghi 5 MB rồi thoát.
- **Hợp đồng với công cụ thật (CR-CV-070):** workflow có `gitnexus` cài sẵn, index một repo fixture nhỏ (tạo trong thư mục tạm bằng `gitnexus analyze --index-only`, chưa kiểm chứng thời gian), chạy từng mẫu, so với fixture vàng; thất bại khi tiêu đề cột, dạng `[]`/`{error}`, hoặc cơ số dòng đổi.
- **Thủ công, một lần, trước merge:** chạy bảng 2.4 trên `/opt/repos/orca` và ghi thời gian từng mẫu; ghi vào PR.
- Test mới của agent: `agent/src/relay/gitnexus-cypher-literal.test.ts`, `gitnexus-cypher-guard.test.ts`, `gitnexus-cypher-markdown-parser.test.ts`, `gitnexus-cypher-runner.test.ts`, `gitnexus-index-probe.test.ts`, `codeintel-gitnexus-methods.test.ts`, `codeintel-symbol-ref.test.ts`, `codeintel-short-lived-cache.test.ts`. Chưa chạy gì ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Định dạng markdown của `cypher` và hình dạng JSON của `context/impact` là hành vi nội bộ của GitNexus 1.6.9**, không có cam kết ổn định. Giảm thiểu: kiểm tra tiêu đề cột, fixture vàng, dải phiên bản (CR-CV-001 2.7). Giai đoạn sau có thể chuyển sang MCP stdio (nghiên cứu 02, phương án B) để có đối tượng thật, ngoài phạm vi.
- Cú pháp một số mẫu **chưa chạy** (2.4: `STEP_EDGES`, `MEMBER_CLUSTER` theo `s.id IN`, `SG_EDGES_AROUND` với danh sách `IN`, `RT_LIST` có `SKIP`). Tiêu chí chấp nhận đầu tiên bắt buộc chạy.
- **Chi phí:** `overview` ~6 s và 3-4 tiến trình mỗi lần; giới hạn 3 tiến trình đồng thời của CR-CV-001 làm hàng đợi dài khi nhiều người xem. Chưa đo trên dev server thực. Backend cần cache (CR-CV-022).
- **Dữ liệu chỉ mục có thể sai lệch:** `impact` upstream của `runToolCommand` (bản `agent/`) chỉ trả 1 nút (`handler#2`), trong khi `agent-tool-registry.ts` gọi hàm này từ bảy `handler` trong `ALL_TOOL_DEFINITIONS` (đọc code: `claude_code`, `gh`, `git`, `gitnexus`, `codegraph`, `docker`, `shell`); GitNexus có vẻ gộp các `handler` vô danh cùng tên thành một nút. Nghĩa là blast radius có thể bị đếm thiếu với hàm gọi từ nhiều hàm vô danh; độ phủ cạnh của GitNexus không được đo ở đây. UI không được trình bày "blast radius" như bằng chứng đầy đủ; văn bản UI nên nói "theo chỉ mục GitNexus tại <commit>".
- **Số `communities` lệch** (10 252 vs 9 039) chưa rõ nguyên nhân.
- `--branch` của `cypher/context/impact` (chỉ mục theo nhánh, "multi-branch") **chưa được kiểm chứng** và không dùng; có thể là chìa khoá để chỉ mục theo worktree (Q1).
- Mã nguồn lấy từ tệp hiện tại có thể không khớp chỉ mục cũ (cảnh báo đã nêu); không có cách xác minh khớp ngoài so `indexedCommit` với `HEAD` và `git status`.
- Đọc đồng thời với `analyze` đang chạy chưa kiểm chứng; CR-CV-004 quyết định chặn đọc GitNexus khi có job.
- GitNexus ghi `AGENTS.md`/`CLAUDE.md` khi `analyze` không có `--index-only`; không ảnh hưởng đường đọc nhưng liên quan CR-CV-004.

## 7. Câu hỏi mở

- **Q1.** Có nên dùng `--branch <name>` để đọc chỉ mục theo nhánh/worktree thay vì chỉ mục "workspace" không? Cần thử `gitnexus analyze --branch` trên repo thử nghiệm (CR-CV-004).
- **Q2.** `codeintel.symbol` có nên cho phép đọc nội dung theo `commit` (qua `git show <commit>:<path>`) thay vì tệp working tree khi `stale`? Đề xuất: chưa; thêm nếu review cho thấy cần.
- **Q3.** Thêm mã lỗi `CODEINTEL_SYMBOL_NOT_FOUND`? Hiện gộp vào `CODEINTEL_INVALID_PARAMS (reason = "not_found")`; xem "Điều chỉnh hợp đồng".
- **Q4.** Giới hạn `UNWIND` bị cấm trong guard có cản mẫu hợp lệ nào về sau không? Chưa có mẫu cần nó.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 3.2, 3.3, 6)
- `/opt/repos/orca/docs/research/view-code/04-raw-data-and-pipeline.md`, `05-graph-schemas.md`, `06-gaps-risks-roadmap.md`
- `/opt/repos/orca/docs/crs/v7/agent-codeintel/CR-CV-001-codeintel-agent-foundation.md` (runner, whitelist, mã lỗi, 1.9)
- `~/.gitnexus/registry.json`, `/opt/repos/orca/.gitnexus/meta.json` (đọc ngày 2026-10-05)
- `/opt/repos/orca/agent/src/relay/agent-tool-registry.ts` (`runToolCommand` `:72-113`; số dòng 1-based dùng cho phép đối chiếu 1.7)
- `/opt/repos/orca/agent/src/relay/agent-rpc-dispatch.ts` (`makeError` `:408`; số dòng 1-based)
- `/opt/repos/orca/agent/src/relay/git-handler-check-ignore.ts` (`checkIgnoredPathsOp`, có thể tái dùng cho `check-ignore`)
