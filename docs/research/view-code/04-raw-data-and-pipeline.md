# 04 — Dữ liệu thô và pipeline xử lý

Mọi số liệu dưới đây đo trực tiếp trên repo Orca của máy này (2026-10-05). Hai chỉ mục không khớp nhau về số node vì phạm vi trích xuất khác nhau; coi đó là hai nguồn bổ trợ chứ không phải hai bản sao.

## 1. Dữ liệu thô

### 1.1 GitNexus (graph LadybugDB, schema v5)

`.gitnexus/meta.json`: `repoPath, lastCommit, indexedAt, branch, remoteUrl, stats{files:20174, nodes:247556, edges:644157, communities:10252, processes:300}, capabilities, schemaVersion, fileHashes`.

Số liệu qua `cypher` (Orca):
- Nhãn node: Function 80 348, Section 36 507, Const 34 424, Method 31 432, Property 26 345, File 20 174, **Community 9 039**, Struct 4 740, Folder 1 616, Variable 1 111, Class 932, Interface 459, **Process 300**, **Route 92**, Enum 19, Constructor 18.
- Loại cạnh (`CodeRelation.type`): CALLS 180 460, DEFINES 123 928, IMPORTS 85 453, ACCESSES 82 920, CONTAINS 58 269, MEMBER_OF 55 259, HAS_METHOD 29 720, HAS_PROPERTY 25 135, STEP_IN_PROCESS 1 366, METHOD_IMPLEMENTS 762, METHOD_OVERRIDES 537, IMPLEMENTS 158, HANDLES_ROUTE 92, EXTENDS 58, FETCHES 21, ENTRY_POINT_OF 19.
- (Chênh `communities` 10 252 trong `meta.json` so với 9 039 node `Community` truy vấn được: cần xác minh khi cài đặt, có thể do ngưỡng lọc.)

Hình dạng bản ghi (mẫu thật):
```
Function { id:"Function:<path>:<name>", name, filePath, startLine, endLine, isExported, content, description, _label, _id }
Community{ id:"comm_6117", label, heuristicLabel, keywords[], description, enrichedBy, cohesion, symbolCount }
Process  { id:"proc_0_checkspanel", label:"A → B", heuristicLabel, processType:"cross_community", stepCount,
           communities:["'comm_7339'",…], entryPointId, terminalId }
Route    { id:"Route:/voice-settings", name, filePath, responseKeys[], errorKeys[], middleware[], method, handlerSymbolId }
CodeRelation { type, confidence(0..1), reason("local-call"|"trace-detection"|…), step, _src{offset,table}, _dst{offset,table} }
```
Lưu ý định dạng: `gitnexus cypher` **không trả JSON dòng**; trả `{ "markdown": "| cột |…", "row_count": n }`, mỗi ô object là một chuỗi JSON. Muốn lấy cạnh phải `RETURN a.id, b.id, r.type …` (cột phẳng) thay vì `RETURN r`, vì `r` chỉ mang offset nội bộ `_src/_dst`. `communities[]` của Process chứa chuỗi có dấu nháy đơn (`"'comm_7339'"`), cần bỏ nháy.

Các lệnh khác trả JSON: `context`, `impact` (có trạng thái `ambiguous` + `candidates[{uid,name,kind,filePath,line,score}]`), `query`, `trace` (cần kiểm tra khi cài đặt).

### 1.2 CodeGraph (SQLite `codegraph.db`)

Bảng: `nodes(id, kind, name, qualified_name, file_path, language, start_line, end_line, start_column, end_column, docstring, signature, visibility, is_exported, is_async, is_static, is_abstract, decorators, type_parameters, return_type, updated_at)`, `edges(id, source, target, kind, metadata, line, col, provenance)`, `files(path, content_hash, language, size, modified_at, indexed_at, node_count, errors)`, `unresolved_refs`, `project_metadata`, `schema_versions`, kèm FTS.

Số liệu (`codegraph status --json`): 295 761 node, 958 441 edge, 15 759 file, DB 1,26 GB; ngôn ngữ csharp, go, javascript, kotlin, python, ruby, swift, tsx, typescript, xml, yaml.
- `kind` node: import 67 796, property 63 589, function 59 588, method 37 968, constant 22 353, type_alias 19 350, file 15 710, struct 4 442, variable 2 965, class 953, interface 458, field 184, route 184, enum_member 122, component 62, enum 33, namespace 4.
- `kind` cạnh: calls 381 322, contains 296 175, references 135 310, imports 119 938, instantiates 23 006, implements 2 639, extends 51.
- JSON qua CLI (`--json`): `query` → `[{node:{id,kind,name,qualifiedName,filePath,language,startLine,endLine,…}, score}]`; `callers/callees` → `{symbol, callers[{name,kind,filePath,startLine}]}`; `status`, `files`.
- Không có JSON: `explore`, `node` (văn bản mã nguồn + vết gọi), dùng để hiển thị mã chứ không để vẽ.
- Tuỳ chọn riêng: CodeGraph có `route`, `component`, `import` (GitNexus không có node Import).

## 2. Pipeline xử lý (5 tầng)

```
[T0 Nguồn]   gitnexus / codegraph trên dev server (đọc index)
   │ raw: markdown(cypher), JSON(context/impact/--json), text(explore/node), SQL rows
[T1 Trích xuất — AGENT]   codeintel.* : dựng lệnh whitelist, chạy, parse, cắt giới hạn, gắn {commit, indexedAt, tool, toolVersion}
   │ CodeIntelChunk (JSON gọn, đã bị cắt)
[T2 Vận chuyển]           Relay/RelayStream (JSON-RPC, khung ≤16 MiB; notifyBulk cho gói lớn)
   │
[T3 Chuẩn hoá — BACKEND]  gộp 2 nguồn → CodeGraphModel (05), khử trùng id, gắn cụm, tính xếp hạng/ẩn cạnh rác
   │ lưu cache (repo, commit, level, params-hash), cờ stale
[T4 Phân phối]           gateway → WS result/push; phân trang; ETag theo commit
[T5 Hiển thị — UI]       chọn view (Architecture/Flow/Impact/Route/Symbol), layout, drill-down
```

### Các phép xử lý chính
1. **Chọn repo & kiểm tra tươi** (T1): so `lastCommit` của GitNexus với `git rev-parse HEAD` của worktree; `stale = đã lệch commit hoặc working tree bẩn`. CodeGraph có `pendingChanges{added,modified,removed}`.
2. **Dựng truy vấn** (T1): mẫu Cypher có tham số, ví dụ:
   - cụm: `MATCH (c:Community) RETURN c.id,c.label,c.symbolCount,c.cohesion ORDER BY c.symbolCount DESC LIMIT $n`
   - cạnh giữa cụm: `MATCH (a)-[r:CodeRelation]->(b) WHERE r.type IN ['CALLS','IMPORTS'] MATCH (a)-[:CodeRelation {type:'MEMBER_OF'}]->(ca:Community), (b)-[:CodeRelation {type:'MEMBER_OF'}]->(cb:Community) WHERE ca<>cb RETURN ca.id,cb.id,count(*) AS w` (cần kiểm thử cú pháp với LadybugDB trước khi chốt)
   - luồng: `MATCH (s)-[r:CodeRelation {type:'STEP_IN_PROCESS'}]->(p:Process) WHERE p.id=$id RETURN s.id,s.name,s.filePath,r.step ORDER BY r.step`
3. **Parse markdown** (T1): tách cột bằng `|`, giải mã ô JSON; test với tên có `|` và xuống dòng trong `content`.
4. **Cắt** (T1): luôn có `limit`/`depth`; trả `truncated:true` và `totalCount`.
5. **Hợp nhất id** (T3): khoá chuẩn `kind:filePath:qualifiedName`; bảng ánh xạ id GitNexus ↔ id CodeGraph (xem 05 §3).
6. **Tổng hợp cạnh** (T3): với mức Architecture, gộp cạnh cấp symbol thành cạnh cấp cụm với trọng số.
7. **Cache** (T3): khoá `(tenant, repo, commit, view, paramsHash)`; huỷ khi `codeintel.changed`.
8. **Chia trang dữ liệu lớn** (T4): UI không nhận quá ~1 500 nút/lần.

### Ngân sách kích thước
- Không bao giờ chuyển cả graph (Orca: 247k nút/644k cạnh).
- Mức tổng quan: ≤ 500 cụm + ≤ 5 000 cạnh tổng hợp.
- Mức subgraph: ≤ 1 500 nút, ≤ 4 000 cạnh; mã nguồn chỉ lấy theo từng symbol.

## 3. Mô hình lỗi
- Chưa có index: trả `INDEX_MISSING` + gợi ý `codeintel.reindex`.
- Index cũ: vẫn trả dữ liệu, kèm `stale:true`, `indexedCommit`, `headCommit`.
- Công cụ thiếu trên dev server: `TOOL_UNAVAILABLE` (biết trước từ `tools[]` của handshake).
- Truy vấn quá hạn: `TIMEOUT`, kèm gợi ý thu hẹp phạm vi.
- Mơ hồ symbol (GitNexus `status:"ambiguous"`): trả danh sách `candidates` để UI cho người dùng chọn.
