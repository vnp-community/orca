# CR-CV-003 — Trích xuất CodeGraph: status, query, callers, callees, files, node; ánh xạ id/kind; đọc SQLite chỉ-đọc (tuỳ chọn)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-003 |
| **Tên** | Nguồn CodeGraph cho `codeintel.*`: bọc CLI `--json`, ánh xạ `SymbolRef`, đọc `codegraph.db` bằng `node:sqlite` ở chế độ chỉ-đọc khi có, và cách hợp nhất với kết quả GitNexus |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-001; dùng `codeintel-symbol-ref.ts` do CR-CV-002 tạo (nếu CR này merge trước thì tạo file đó với bảng kind đầy đủ, CR-CV-002 chỉ thêm hàm phía GitNexus) |
| **Mở khoá** | CR-CV-005 (danh sách test bị ảnh hưởng qua `affected`), CR-CV-020 (hợp nhất hai nguồn), CR-CV-037 (mã chết/hotspot), CR-CV-070 |
| **Tác động** | `agent/src/relay/codeintel-method-table.ts` (thêm dòng), các file mới `agent/src/relay/codegraph-*.ts`, `agent/src/relay/codeintel-symbol-ref-codegraph.ts`; `agent/package.json` **không đổi** (xem 2.5) |

---

## 1. Bối cảnh và vấn đề

Số liệu do người soạn chạy thật `codegraph 1.4.1` trên `/opt/repos/orca` ngày 2026-10-05 (chỉ lệnh đọc).

1. **Phạm vi dữ liệu.** `codegraph status --json`: `nodeCount 295910`, `edgeCount 959095`, `fileCount 15773`, DB 1 257 566 208 byte (~1,2 GB), `backend: "node-sqlite"`, `journalMode: "wal"`, `index.state: "complete"`, `extractionVersion 24`, `pendingChanges {added,modified,removed}`, `worktreeMismatch: null`. Các số này đã lệch nhẹ so với README v7 (295 761 / 958 441 / 15 759) vì chỉ mục tự đồng bộ (thư mục `.codegraph/` có `daemon.pid`, `daemon.sock`, `daemon.log`). `status --json` **không** có commit; `lastIndexed` là chuỗi ISO.
2. **Lệnh nào có JSON.** `status -j`, `query -j`, `callers -j`, `callees -j`, `impact -j`, `files -j`, `affected -j` (đã chạy tất cả); `explore` và `node` chỉ có text (`node` in dạng markdown: tiêu đề, `**Location:** path:line`, chữ ký, "Trail"). Lệnh `callers/callees/impact` nhận **tên symbol** (không nhận id hay tệp), `node` nhận `-f <file>` để phân giải tên trùng.
3. **Lỗi đi qua stdout dạng văn bản và thoát mã 0.** `callers NoSuchSymbolZzz -j` in `ℹ Symbol "NoSuchSymbolZzz" not found` (có mã ANSI, **không phải JSON**); `status` ở thư mục chưa khởi tạo với `-j` trả `{"initialized":false,...,"lastIndexed":null}`; `query foo -p /tmp -j` in `✗ CodeGraph not initialized in /tmp`. Phải kiểm tra stdout có bắt đầu bằng `[` hoặc `{` trước khi parse.
4. **Gọi theo tên sai lệch.** `callers runToolCommand -j` trả toàn các nút `kind: "file"` (kể cả `desktop/src/relay/agent-tool-registry.ts` và `deploy/agent/agent.js`, tức là hai bản và cả bản đóng gói); `callees runToolCommand` lẫn `spawn` (`agent/src/relay/subprocess.test.ts:49`) và `kill` (`desktop/src/main/computer/sidecar-client.test.ts:45`); `callers handler` trả tệp test không liên quan. Lý do: phân giải tên chung + gộp cạnh `calls` từ cấp tệp. Kết quả CLI theo tên **không đủ tin cậy làm đồ thị gọi**; cần truy theo `id`.
5. **Số dòng của CodeGraph là 1-based và đúng** (`runToolCommand` 72-113, khớp `grep -n`), khác GitNexus (0-based, CR-CV-002 1.7). `files -j` luôn trả mảng phẳng `[{path, language, nodeCount, size}]` (đã thử `--format grouped`: vẫn mảng phẳng); `--filter` nhận tiền tố thư mục hoặc đường dẫn tệp.
6. **Id không ổn định:** `function:3d380603e4c20fed6026ed3bcef2587f` (hash), còn nút tệp có id dạng đường dẫn `file:agent/src/relay/context.ts` và nút `import` dạng `import:<hash>`. Không dùng id băm làm khoá bền (README v7 mục 3.4).
7. **SQLite đọc được và rất nhanh.** Schema (`pragma`): bảng `schema_versions` (phiên bản 1 và 8), `nodes`, `edges`, `files`, `unresolved_refs`, `name_segment_vocab`, `project_metadata` (khoá `index_state`, `indexed_with_version`, `indexed_with_extraction_version`, `index_files_discovered`, `index_files_accounted`; **không** có khoá thời điểm), `sqlite_stat1`, FTS. Cột `edges`: `id, source, target, kind, metadata, line, col, provenance`. Mở bằng `node:sqlite` `DatabaseSync(path, {readOnly:true})` trong Node 22.23 chạy được khi daemon đang giữ WAL, tra `nodes` theo `(file_path, name)` mất 0 ms và tra callers theo `target` mất 0 ms (chưa đo trên tải thật). Node phát `ExperimentalWarning: SQLite is an experimental feature` ra **stderr**; trong chế độ `--stdio` stdout là giao thức nên cảnh báo này an toàn nhưng phải bị chặn khỏi log của agent (dùng `process.removeAllListeners('warning')` có điều kiện, hoặc cờ `--no-warnings`: chưa kiểm chứng cách khởi chạy agent).
8. **Driver SQLite trên agent.** `agent/package.json` không có `better-sqlite3` hay `sqlite3` trong `dependencies`/`devDependencies` (đã đọc file và `ls agent/node_modules | grep -i sqlite` rỗng). `agent/build.mjs` liệt kê `'better-sqlite3'` trong `external` nhưng không khai báo dependency (chỉ là phòng hờ; **không có driver**). Tiền lệ đã có: `agent/src/relay/external-automations-handler.ts:497-520` nạp `node:sqlite` tuỳ chọn qua `requireOptional('node:sqlite')`, bọc thành lớp `RelaySqliteDatabase` với `readOnly`, `timeout`, `fileMustExist`, và "máy không có `node:sqlite` thì bỏ qua". `engines.node` của `agent/package.json` là `24`, nhưng `build.mjs` đặt `target: 'node22'`; bản Node 22.5-22.12 cần cờ `--experimental-sqlite` (nhớ từ tài liệu Node, chưa kiểm chứng ở đây).
9. **Chạy trong worktree liên kết.** Đã chạy `codegraph status -j` trong `/opt/repos/orca/.claude/worktrees/dev-process-9beda3`: trả `projectPath: /opt/repos/orca` và `worktreeMismatch: {"worktreeRoot": ".../dev-process-9beda3", "indexRoot": "/opt/repos/orca"}`. Có sẵn cờ chính thức để phát hiện (CR-CV-001 1.5).
10. **`affected`** (`codegraph affected <files...> -j`) trả `{changedFiles, affectedTests[]}` (đã chạy với `agent/src/relay/agent-tool-registry.ts`: nhiều tệp test); đây là nguồn trực tiếp cho "test bị ảnh hưởng" (CR-CV-005, CR-CV-036).
11. **Tốc độ CLI.** `codegraph status -j` ~0,06 s; `callers -j` ~0,4 s; nhanh hơn GitNexus ~4 lần.

## 2. Giải pháp đề xuất

### 2.1 Probe chỉ mục và `codeintel.status` (phần CodeGraph)

File mới `agent/src/relay/codegraph-index-probe.ts` đăng ký probe với `codeintel-status.ts` (CR-CV-001): chạy `codegraph status <projectPath> -j`, parse JSON (dạng `initialized:false` → `state: "missing"`):

```jsonc
{ "state": "ready|stale|missing|building|unknown",
  "indexedAt": "2026-10-05T12:33:43.367Z",                        // lastIndexed
  "stats": { "files": 15773, "nodes": 295910, "edges": 959095 },
  "pendingChanges": { "added": 0, "modified": 0, "removed": 0 },
  "backend": "node-sqlite", "journalMode": "wal", "dbSizeBytes": 1257566208,
  "extractionVersion": 24, "reindexRecommended": false,
  "worktreeMismatch": null }                                      // hoặc {worktreeRoot, indexRoot}
```

- `state`: `missing` nếu `initialized:false`; `building` nếu job reindex CodeGraph đang chạy (CR-CV-004) hoặc `index.state !== "complete"`; `stale` nếu `pendingChanges` có số dương, `worktreeMismatch` khác `null`, hoặc `reindexRecommended: true`; còn lại `ready`.
- Vì CodeGraph không báo commit, `sources[].commit = null`. Độ cũ dựa vào `pendingChanges` (chỉ số này phản ánh working tree so với DB tại thời điểm hỏi, hữu ích hơn so commit) và `worktreeMismatch`. Ghi trong `warnings: ["codegraph_has_no_commit"]` khi `stale` suy ra từ `pendingChanges`.
- Không gọi bất kỳ lệnh nào có thể làm CodeGraph ghi. Daemon của CodeGraph tự đồng bộ (thấy `daemon.pid`/`daemon.sock`); agent **không** khởi động, dừng hay `unlock` daemon.

### 2.2 Method và hợp đồng (CodeGraph)

README v7 mục 3.2 không có method riêng cho CodeGraph; nguồn này **làm giàu** các method của CR-CV-002 và cung cấp ba method đọc nhẹ mới. Đề xuất chốt lại ở "Điều chỉnh hợp đồng":

| Method | Dùng cho | Nguồn | Giới hạn |
|---|---|---|---|
| `codeintel.symbol` (CR-CV-002) | Thêm `codegraphId`, `signature`, `docstring`, `isExported` khi khớp được | `query` hoặc SQLite | `docstring` ≤ 4 KiB |
| `codeintel.subgraph` (CR-CV-002) | Tuỳ chọn `source: "codegraph"` cho tầng gọi `calls` khi GitNexus thiếu cạnh | SQLite (ưu tiên) / CLI | cùng giới hạn subgraph |
| `codeintel.impact` (CR-CV-002) | `testsCovering` điền từ `affected` | `codegraph affected -j` | ≤ 200 tệp test |
| `codeintel.codegraphSearch` (**mới, đề xuất**) | Tìm symbol theo tên/từ khoá (UI Cmd+K, chọn `uid` khi `ambiguous`) | `query -j` | `limit` 1..50 (mặc định 10) |
| `codeintel.files` (**mới, đề xuất**) | Cây tệp có `language`, `nodeCount`, `size` cho view Cấu trúc | `files -j` | ≤ 5 000 tệp/lần, `truncated` |

(Hai method mới là đề xuất bổ sung hợp đồng; nếu bị từ chối, hai chức năng này sẽ nằm trong `codeintel.symbol` và `codeintel.subgraph`.)

**`codeintel.codegraphSearch`** `{workspaceRoot, search (1..256), limit (1..50, mặc định 10), kind? }`. Chạy `codegraph query <search> -p <root> -j -l <limit> [-k <kind>]`. Trả `{results:[{symbol: SymbolRef, score}]}` đã ánh xạ (2.4), bỏ `kind: import`. `kind` thuộc tập kind của CodeGraph (`function, method, class, interface, struct, enum, type_alias, constant, variable, property, field, route, component, namespace`).

**`codeintel.files`** `{workspaceRoot, filter? (đường dẫn tương đối, ≤ 512), limit (1..5000, mặc định 2000)}`. `codegraph files -p <root> -j [--filter <filter>]`; trả `{files:[{path,language,nodeCount,size}]}` (đã mảng phẳng). Cấm `filter` bắt đầu bằng `-` hoặc chứa `..`.

### 2.3 Hai đường lấy dữ liệu: CLI và SQLite chỉ-đọc

| Truy vấn | CLI (luôn có) | SQLite (nếu `sqliteReadAvailable`) |
|---|---|---|
| Trạng thái | `status <path> -j` | `project_metadata` (chỉ đọc bổ sung; không cần) |
| Tìm symbol theo tên | `query` (FTS + điểm số) | `nodes` theo `(file_path, name, kind)` (khớp chính xác); tìm mờ vẫn qua CLI |
| Callers/callees theo `id` | **không có theo id** (theo tên, mơ hồ, 1.4) | `edges` theo `target`/`source` và `kind='calls'`, nối `nodes` |
| Danh sách tệp | `files -j` | `files` |
| Test bị ảnh hưởng | `affected -j` | (phức tạp, dùng CLI) |
| Mã nguồn | không (dùng đọc tệp, CR-CV-002) | không |

Quy tắc: **trả lời chính xác chỉ khi có id**. Nếu SQLite khả dụng, `callers/callees` của một symbol đã phân giải (`file_path`, `name`, `kind` duy nhất; nếu nhiều nút trùng → `CODEINTEL_AMBIGUOUS_SYMBOL` với `candidates`) đi qua truy vấn SQL theo id. Nếu SQLite không khả dụng, dùng CLI theo tên và: (a) đặt `warnings: ["codegraph_name_based_resolution"]`, (b) lọc kết quả về `filePath` thuộc cùng gói (không tin tuyệt đối), (c) đánh dấu mọi cạnh `confidence: "low"`. Kết quả CLI theo tên không được dùng cho view cần tính đúng (mã chết, CR-CV-037).

### 2.4 Đọc SQLite chỉ-đọc (tuỳ chọn, mặc định bật nếu có)

File mới `agent/src/relay/codegraph-sqlite-reader.ts`:

- Nạp `node:sqlite` bằng cách tương tự `external-automations-handler.ts:497-520` (`requireOptional`), **không thêm dependency** (`agent/package.json` không đổi); mở `new DatabaseSync(dbPath, { readOnly: true, timeout: 2000 })` chỉ cho tệp `<projectPath>/.codegraph/codegraph.db` (đường dẫn dựng từ binding, không nhận từ client; `realpath` phải nằm trong `projectPath`). Không bao giờ mở tệp ở chế độ ghi; không chạy `PRAGMA` có tác dụng ghi; chỉ câu `SELECT` có sẵn trong mã (danh sách hằng), không nhận SQL từ ngoài.
- Một kết nối cho mỗi DB, đóng khi nhàn rỗi 60 s hoặc khi reindex CodeGraph bắt đầu (CR-CV-004) và mở lại sau; vì DB ~1,2 GB và WAL do daemon ghi, tránh giữ kết nối dài quá cần thiết.
- Kiểm tra `schema_versions`: chỉ chạy khi phiên bản tối đa nằm trong dải đã thử (hiện `8`) và `project_metadata.indexed_with_extraction_version` đã biết (hiện `24`); khác → `sqliteReadAvailable = false` kèm `warnings: ["codegraph_schema_unsupported:<ver>"]` và dùng CLI.
- `sqliteReadAvailable` trong `codeintel.status` = module nạp được **và** mở thử được **và** schema trong dải. Không nạp được `node:sqlite` (Node cũ, thiếu cờ) → `false`, không phải lỗi.
- Truy vấn mẫu (đã chạy trên Orca):
  - tìm nút: `SELECT id, kind, name, qualified_name, file_path, start_line, end_line, signature, is_exported FROM nodes WHERE file_path = ? AND name = ? AND kind IN (?, ...)` (0 ms);
  - callers: `SELECT n.id, n.kind, n.name, n.qualified_name, n.file_path, n.start_line, e.kind AS ek, e.line FROM edges e JOIN nodes n ON n.id = e.source WHERE e.target = ? AND e.kind IN ('calls','references','instantiates') LIMIT ?` (0 ms, đã thấy phần lớn người gọi là nút `file` khi gọi từ mức module/hàm ẩn danh);
  - callees: đối xứng theo `e.source`.
- Mọi truy vấn có `LIMIT` ≤ 1 500; không `JOIN` không có chỉ mục (chưa kiểm chứng chỉ mục; `sqlite_stat1` tồn tại). Người triển khai phải chạy `EXPLAIN QUERY PLAN` cho từng truy vấn và ghi lại.
- Cảnh báo thử nghiệm: `node:sqlite` đang "experimental" (1.7). Mặc định đặt cờ cấu hình `ORCA_CODEINTEL_SQLITE=auto|off` (mặc định `auto`); `off` buộc dùng CLI.

### 2.5 Ánh xạ `SymbolRef` (phần CodeGraph) và hợp nhất id

`codeintel-symbol-ref-codegraph.ts` (hàm thuần) dùng bảng kind trong `codeintel-symbol-ref.ts` (CR-CV-002 2.7):

| Chuẩn | CodeGraph `kind` |
|---|---|
| `function` | `function` |
| `method` | `method` |
| `type` | `struct`, `class`, `interface`, `enum`, `type_alias`, `namespace` (**đề xuất:** `namespace` không có trong bảng 05 §3; giữ `rawKind = namespace`) |
| `value` | `constant`, `variable`, `property`, `field`, `enum_member` (**đề xuất** cho `enum_member`) |
| `file` | `file` |
| `route` | `route` |
| `component` | `component` |
| (bỏ) | `import` (67 865 nút, không có trong 05 §3; không đưa vào đồ thị mặc định) |

- `codegraphId` = `id`; `name` = `name`; `qualifiedName` chuẩn hoá `::` → `.` (CodeGraph `RelayContext::registerRoot` ↔ GitNexus `RelayContext.registerRoot`); `filePath` = `file_path`/`filePath`; `startLine`, `endLine` giữ nguyên (đã 1-based); thêm `signature`, `isExported`, `docstring` (cắt 4 KiB).
- **Khoá hợp nhất** `key = "<kind>:<filePath>:<qualifiedName>"` giống GitNexus. Sai khác đã biết: GitNexus gộp các `handler` vô danh cùng tệp thành `handler#<n>`; CodeGraph đặt `qualified_name` = `handler` cho từng nút (chưa đối chiếu); `key` có thể trùng nhiều nút → CR-CV-020 quyết định (agent chỉ báo `key_collision` trong `warnings`).
- Cộng thêm quy tắc kiểm tra chéo cho CR-CV-020: khi hai nguồn cùng `key` nhưng `startLine` chênh > 2 sau khi chuẩn hoá cơ số, đánh dấu `sourcesDisagree` (lý do có thể: chỉ mục khác commit).

### 2.6 `affected` và test bị ảnh hưởng

Hàm `getAffectedTests(binding, files[])` trong `codegraph-affected-tests.ts` (mới): `codegraph affected --stdin -p <root> -j -d 5` với danh sách tệp qua **stdin** (tránh giới hạn độ dài dòng lệnh và giá trị bắt đầu bằng `-`). Giới hạn: ≤ 500 tệp đầu vào (chia nhóm), ≤ 200 tệp test trong kết quả (`truncated`). Chạy qua `runCodeIntelTool` (CR-CV-001; cần cho phép ghi stdin có kiểm soát: bổ sung tuỳ chọn `stdinText` vào `runToolCommand`, hiện `child.stdin?.end()` ngay). Nếu CR chưa thêm được `stdinText`, rơi về truyền tệp làm đối số (mỗi giá trị qua kiểm tra không bắt đầu bằng `-`), chia nhóm 50 tệp.

### 2.7 `codeintel.node` dạng text

Không có method `node` công khai (`node` chỉ có text). `codeintel.symbol` có thể gọi `codegraph node <name> -f <file> -p <root>` để lấy "Trail" (người gọi/được gọi kèm số dòng) khi `includeTrail: true` (mặc định `false`); parse tối thiểu: dòng `**Calls →**` và `**Called by ←**` tách theo `, ` thành `name (path:line)`. Định dạng này **không ổn định** (ví dụ xuất ra có thể thêm cột); chỉ phục vụ gợi ý trong UI, đánh dấu `experimental: true`, không dùng cho tính toán. Khi parse lỗi: bỏ qua im lặng (kèm cảnh báo), không lỗi.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| CLI `--json` là đường chính, SQLite chỉ-đọc là tối ưu tuỳ chọn | CLI luôn có; SQLite phụ thuộc `node:sqlite` (không có driver nào khác trên agent, 1.8) |
| Không thêm dependency SQLite | Native module làm phức tạp đóng gói agent đa nền tảng (`build.mjs` đã phải `external` hoá nhiều native); `node:sqlite` đã được dùng tuỳ chọn ở `external-automations-handler.ts` |
| Chỉ câu `SELECT` hằng, không nhận SQL ngoài | Giữ ranh giới "agent chỉ chạy lệnh đã định" (D5) |
| Callers/callees chính xác chỉ khi có id; còn lại gắn cờ `codegraph_name_based_resolution` | CLI theo tên trả nhiễu thật (1.4) |
| Không đưa nút `import` vào đồ thị | 67 865 nút gần như không có giá trị cho review và làm phình dung lượng |
| Chuẩn hoá `::` → `.` và cơ số dòng khi hợp nhất | Hai công cụ khác dấu phân cách và cơ số (GitNexus 0-based) |
| Không chạm daemon CodeGraph | Daemon tự đồng bộ; agent chỉ đọc |
| `stale` của CodeGraph dựa trên `pendingChanges` + `worktreeMismatch` | CodeGraph không báo commit (1.1) |

## 4. Tiêu chí chấp nhận

- [ ] `codeintel.status` trên Orca trả khối `indexes.codegraph` đúng các trường ở 2.1; thư mục chưa khởi tạo trả `state: "missing"` (không lỗi); thư mục không phải repo → lỗi của CR-CV-001.
- [ ] Mọi lệnh CodeGraph chạy với `-j` (trừ `node`), kiểm tra stdout bắt đầu `[`/`{`; ca `Symbol "..." not found` (text + ANSI, exit 0) được quy về `CODEINTEL_INVALID_PARAMS (not_found)`; ca `not initialized` về `CODEINTEL_INDEX_MISSING`.
- [ ] `codeintel.codegraphSearch search="runToolCommand"` trả hai kết quả hàm (`agent/` dòng 72 và `desktop/` dòng 72) với `startLine` 1-based, `codegraphId` bắt đầu bằng `function:`; không có nút `import`.
- [ ] Khi `sqliteReadAvailable = true`: callers của `agent/src/relay/agent-tool-registry.ts:runToolCommand` (id) trả đúng tập người gọi từ SQL, không lẫn `spawn`/`kill` từ tệp khác; có `ORCA_CODEINTEL_SQLITE=off` thì dùng CLI và có cờ `codegraph_name_based_resolution`.
- [ ] Mở DB chỉ ở chế độ `readOnly: true`; test chứng minh ghi vào DB thất bại; không tạo/ sửa `codegraph.db-wal`/`-shm` ngoài hành vi của SQLite khi đọc (đối chiếu `mtime` của `codegraph.db` không đổi).
- [ ] `schema_versions` ngoài dải → `sqliteReadAvailable = false`, vẫn trả kết quả qua CLI.
- [ ] `node:sqlite` thiếu (mô phỏng bằng cách mock `requireOptional`) → không lỗi; `sqliteReadAvailable = false`.
- [ ] `affected` với `agent/src/relay/agent-tool-registry.ts` trả ≥ 1 test, `truncated` đúng khi > 200.
- [ ] `status` trong worktree liên kết trả `worktreeMismatch` khác `null` và `state: "stale"`.
- [ ] Bảng ánh xạ kind: test phủ mọi `kind` trong `nodesByKind` của repo thật (17 loại) — không có kind chưa được ánh xạ hoặc loại bỏ có chủ ý.
- [ ] Không có `agent/package.json` mới dependency; `pnpm lint` và `pnpm test` trong `agent/` xanh; không `max-lines` disable.

## 5. Kiểm thử

| File test (mới) | Nội dung |
|---|---|
| `agent/src/relay/codegraph-index-probe.test.ts` | JSON `status` thật (fixture), `initialized:false`, `worktreeMismatch`, `pendingChanges`, text lỗi |
| `agent/src/relay/codegraph-cli-output.test.ts` | Parse `query`, `callers`, `callees`, `files`, `affected`; text lỗi + ANSI; mảng rỗng |
| `agent/src/relay/codegraph-sqlite-reader.test.ts` | DB SQLite tạm tạo bằng `node:sqlite` với schema thu gọn (bảng `nodes`, `edges`, `schema_versions`, `project_metadata`), kiểm `readOnly`, schema ngoài dải, thiếu module (bỏ qua nếu `node:sqlite` không có trong môi trường CI: dùng `describe.skipIf`) |
| `agent/src/relay/codeintel-symbol-ref-codegraph.test.ts` | Bảng kind đầy đủ, `::`→`.`, `key`, cơ số dòng |
| `agent/src/relay/codegraph-affected-tests.test.ts` | Chia nhóm, giới hạn, stdin vs đối số |
| `agent/src/relay/codeintel-codegraph-methods.test.ts` | `codeintel.codegraphSearch`, `codeintel.files`, `symbol`/`subgraph`/`impact` làm giàu; binary giả |

Hợp đồng với công cụ thật (CR-CV-070): chạy `codegraph init` trên repo fixture nhỏ trong CI, kiểm hình dạng JSON và schema SQLite. Chưa chạy gì ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- **`node:sqlite` đang "experimental"** và cần Node ≥ 22.5 (cờ ở dưới 22.13, chưa kiểm chứng); `agent/package.json` ghi `engines.node: 24`, `build.mjs` target `node22`, môi trường khảo sát là Node 22.23. Dev server có Node cũ → chỉ dùng CLI.
- **Schema SQLite là nội bộ của CodeGraph** (`schema_versions` 8; `extractionVersion 24`). Đã thử một phiên bản; dải rộng hơn là giả định. Fixture vàng bắt buộc (CR-CV-070).
- **Đọc DB 1,2 GB khi daemon ghi WAL**: `readOnly` chạy được ở thử nghiệm 1 lần; ảnh hưởng hiệu năng/độ nhất quán khi `sync`/`index` lớn đang chạy chưa đo. Khi reindex CodeGraph chạy, ngừng đọc SQLite và quay về CLI hoặc trả `CODEINTEL_REINDEX_IN_PROGRESS` (quyết định ở CR-CV-004).
- **Hệ thống tệp chỉ-đọc/NFS** làm `readOnly` + WAL (cần `-shm`) có thể lỗi: chưa kiểm chứng; đã có đường lùi sang CLI.
- **Telemetry**: `codegraph telemetry` tồn tại (CLI có lệnh `telemetry [action]`); chưa kiểm tra trạng thái trên dev server (CR-CV-001 mục 6).
- **Độ chính xác của đồ thị gọi**: người gọi thường là nút `file` (1.4); số liệu "callers" không phải số hàm thật. UI/CR-CV-037 phải diễn đạt trung thực ("tham chiếu gọi trong tệp") và không suy ra mã chết từ đây nếu chưa kiểm chứng.
- Số liệu lệch giữa hai công cụ (GitNexus 247 556 nút, CodeGraph 295 910) là do phạm vi trích xuất khác nhau, không phải lỗi; chưa có phép đối chiếu theo tệp.
- Cần chạy `EXPLAIN QUERY PLAN` (2.4); chưa làm.

## 7. Câu hỏi mở

- **Q1.** Có chấp nhận thêm hai method `codeintel.codegraphSearch`, `codeintel.files` vào hợp đồng README v7 không, hay gói vào method hiện có?
- **Q2.** Có dùng `better-sqlite3` (native) làm driver dự phòng cho Node cũ không? Đề xuất: không, vì đóng gói; chấp nhận chỉ CLI.
- **Q3.** `namespace` và `enum_member` ánh xạ vào kind nào (2.5)? Cần chốt cùng CR-CV-020.
- **Q4.** `ExperimentalWarning` của `node:sqlite`: chặn bằng cách nào mà không ẩn các cảnh báo khác? Cần quyết định với người sở hữu `agent-entry.ts`.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 3.2, 3.4)
- `/opt/repos/orca/docs/research/view-code/04-raw-data-and-pipeline.md` §1.2, `05-graph-schemas.md` §3
- `/opt/repos/orca/docs/crs/v7/agent-codeintel/CR-CV-001-codeintel-agent-foundation.md`, `CR-CV-002-gitnexus-extraction.md`
- `/opt/repos/orca/.codegraph/codegraph.db` (đọc `readOnly` qua `node:sqlite`, 2026-10-05)
- `/opt/repos/orca/agent/package.json` (dependencies, `engines`), `/opt/repos/orca/agent/build.mjs` (`external`, `target`)
- `/opt/repos/orca/agent/src/relay/external-automations-handler.ts` (`:497-520`, tiền lệ nạp `node:sqlite`)
- `/opt/repos/orca/agent/src/relay/agent-tool-registry.ts` (`runToolCommand` `:72`, `child.stdin?.end()`)
