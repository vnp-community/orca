# AG-CV-SOL-003: Trích xuất CodeGraph (`codegraphSearch`, `files`, làm giàu `symbol/subgraph/impact`, SQLite chỉ-đọc)

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mọi mục "đã đọc" là đọc code/CR, chưa chạy gì. Hình dạng đầu ra CodeGraph 1.4.1 và schema SQLite lấy từ CR-CV-003, **chưa chạy lại**.

**CR:** [CR-CV-003](../../../../../../docs/crs/v7/agent-codeintel/CR-CV-003-codegraph-extraction.md)
**Service:** `agent/src/relay/` (Part A); sao sang `desktop/` ở AG-CV-SOL-006
**TDD tham chiếu:** [v5/05-tool-registry](../../../../tdd/v5/05-tool-registry.md) mục 3; [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md) mục 1, 5; [v5/08-deployment](../../../../tdd/v5/08-deployment.md) mục 1 (build `external`)
**Mẫu định dạng:** `specs/agent/crs/v6/agent-capabilities/solutions/AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md`

## 0. Hợp đồng áp dụng

| Nguồn | Mục |
|---|---|
| `CONTRACT-codeintel-agent-rpc.md` | §2.6 (kind CodeGraph, `import` bỏ, `namespace` kind riêng), §4.1 (`indexes.codegraph`, `sqliteReadAvailable`, `ORCA_CODEINTEL_SQLITE`), §4.4 (`source`), §4.5 (`testsCovering` từ `affected`), §4.6 (`includeTrail` thử nghiệm), §4.14 (`codegraphSearch`, `files`), §10 (`node:sqlite` experimental, Node ≥ 22.5, không thêm dependency) |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-19 (`rootMismatch` thay `worktreeMismatch` ở `indexes.codegraph`), PQ-20, PQ-21, O-3, §8.2, §9 O-14 |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): CR-CV-003 toàn bộ; `agent/src/relay/external-automations-handler.ts:490-525` (tiền lệ nạp `node:sqlite` bằng `requireOptional`, lớp `RelaySqliteDatabase` với `readOnly`, `timeout`, `fileMustExist`; chú thích "remote relay vẫn nhắm Node 18"); `agent/package.json` (không có `better-sqlite3`/`sqlite3`); `agent-tool-registry.ts` (`child.stdin?.end()`), `.codegraph/codegraph.db` tồn tại ở gốc repo (không mở).

| Điểm | Hiện trạng |
|---|---|
| Driver SQLite | không có dependency; chỉ `node:sqlite` tuỳ chọn |
| `stdinText` cho `codegraph affected --stdin` | `runToolCommand` chưa có; thêm ở AG-CV-TASK-001-04 |
| `codeintel-symbol-ref.ts` | do AG-CV-SOL-002 tạo (bảng kind đủ cả CodeGraph) |
| Số liệu (CR, chưa chạy lại) | 295 910 nút, 959 095 cạnh, 15 773 tệp, DB ~1,2 GB, `extractionVersion 24`, `schema_versions` 1 và 8, 67 865 nút `import` |

### Correction relative to CR
| # | CR nói | Quyết định |
|---|---|---|
| 1 | `worktreeMismatch` trong probe CodeGraph | `rootMismatch: null\|{worktreeRoot,indexRoot}` (PQ-19); `binding.worktreeMismatch` là boolean |
| 2 | `namespace` ánh xạ `type` (đề xuất) | contract §2.6: kind chuẩn `namespace` riêng |
| 3 | `enum_member` đề xuất `value` | contract: `value` (khớp) |
| 4 | `codeintel.codegraphSearch`, `files` là "đề xuất" | contract §4.14 chính thức, phát hành đợt 4 (P1) |
| 5 | `codeintel.node` như method | không công khai; chỉ nội bộ cho `symbol.includeTrail` (§4.15) |

### Lệch giữa CR và hợp đồng
`confidence:"low"` chuỗi ở CR vs `confidence` số trong `SymbolGraph` edges (contract §4.4 "0 = không biết"): dùng `0` + `warnings:["codegraph_name_based_resolution"]`. `rootMismatch` (PQ-19). `namespace` (§2.6).

### Phụ thuộc chéo khu vực
BE: `BE-CV-SOL-020-canonical-graph-model` (hợp nhất hai nguồn, `sourcesDisagree`), `BE-CV-SOL-021-agent-collector` (chịu `-32601` cho hai method mới). AG: `AG-CV-SOL-001`, `AG-CV-SOL-002` (symbol-ref, handler), `AG-CV-SOL-004` (đóng SQLite khi reindex CodeGraph), `AG-CV-SOL-005` (tuỳ chọn test), `AG-CV-SOL-037-structural-facts` (`unusedExports` cần người gọi chính xác), `AG-CV-SOL-070`. FE: `FE-CV-SOL-053-impact-lens-and-symbol-detail` (Cmd+K dùng `codegraphSearch`).

## 2. Giải pháp

### 2.1 Cây file
```
agent/src/relay/
  codegraph-cli-output.ts            (mới) parse query/callers/callees/files/affected; chặn text ANSI
  codegraph-index-probe.ts           (mới) status -> indexes.codegraph
  codeintel-symbol-ref-codegraph.ts  (mới) ánh xạ kind, key, ::→.
  codegraph-sqlite-reader.ts         (mới) node:sqlite readOnly, SELECT hằng
  codegraph-affected-tests.ts        (mới) affected --stdin, chia nhóm
  codeintel-codegraph-search.ts, codeintel-codegraph-files.ts   (mới) hai method
  codeintel-codegraph-enrichment.ts  (mới) làm giàu symbol/subgraph/impact, includeTrail
  codeintel-method-table.ts          (sửa) +2 dòng
```
### 2.2 Hai đường lấy dữ liệu
CLI `-j` luôn có; SQLite tối ưu khi `sqliteReadAvailable`. **Chính xác chỉ khi có id**: callers/callees theo id đi qua SQL; nếu SQLite không khả dụng dùng CLI theo tên + `codegraph_name_based_resolution`, cạnh `confidence:0`, và kết quả này **không** dùng cho mã chết (037). Nhiều nút trùng -> `AMBIGUOUS_SYMBOL`.
SQLite: `new DatabaseSync(<projectPath>/.codegraph/codegraph.db, {readOnly:true, timeout:2000})`, đường dẫn dựng từ binding (không từ client, `realpath` trong `projectPath`), chỉ `SELECT` hằng, `LIMIT ≤ 1500`, một kết nối/DB đóng sau 60 s nhàn rỗi hoặc khi reindex CodeGraph; điều kiện `schema_versions` max ≤ 8 và `indexed_with_extraction_version` = 24 (CR-070 marker `[1,8]`, `{24}`), nếu không `sqliteReadAvailable:false` + `codegraph_schema_unsupported:<ver>`; `ORCA_CODEINTEL_SQLITE=auto|off`; không thêm dependency; `ExperimentalWarning` ra stderr (cách chặn chưa chốt).
### 2.3 Method
`codeintel.codegraphSearch {search 1..256, limit 1..50 (10), kind?}` -> `codegraph query <s> -p <root> -j -l N [-k kind]`, bỏ `import`. `codeintel.files {filter?, limit 1..5000 (2000)}` -> `codegraph files -p <root> -j [--filter]`, `truncated` khi > limit. Cấm `filter` bắt đầu `-` hoặc chứa `..`. Lỗi text: `Symbol "…" not found` -> `SYMBOL_NOT_FOUND`, `not initialized` -> `INDEX_MISSING`. `affected --stdin -p <root> -j -d 5`: ≤ 500 tệp đầu vào, ≤ 200 test (`truncated`).
Probe: `codegraph status <projectPath> -j`; `initialized:false` -> `missing`; `building` nếu job CodeGraph hoặc `index.state !== complete`; `stale` nếu `pendingChanges` dương, `rootMismatch` khác null hoặc `reindexRecommended`; **`pendingChanges` chỉ nghĩa khi gốc chỉ mục == `workspaceRoot`, ngược lại `null`** (contract §4.1; `sync` ở worktree liên kết báo của checkout chính).

## 3. Quyết định thiết kế
| # | Quyết định | Lý do |
|---|---|---|
| 1 | CLI chính, SQLite tuỳ chọn | `node:sqlite` experimental |
| 2 | Không dependency | đóng gói native phức tạp |
| 3 | Chỉ SELECT hằng | D5 |
| 4 | Không chạm daemon | tự đồng bộ |
| 5 | Bỏ `import` | 67 865 nút nhiễu |

## 4. Thứ tự task
```
01 ─► 02 ; 03 độc lập ; 04 ; 01,03 ─► 05 ; 01 + SOL-001-04 ─► 06 ; 04,05,06 ─► 07 ; 07 ─► 08
```
01 (CLI parser) → 02 (probe) ; 03 (symbol-ref) ; 04 (SQLite) ; 05 (search, files) cần 01, 03 ; 06 (affected) cần 01 ; 07 (làm giàu) cần 03, 04, 05, 06 và AG-CV-TASK-002-09 ; 08 (re-verify) cuối.

## 5. Tiêu chí chấp nhận
- [ ] `status` trả `indexes.codegraph` đủ trường; thư mục chưa khởi tạo -> `missing`.
- [ ] Mọi lệnh `-j` (trừ `node`) kiểm stdout bắt đầu `[`/`{`; ca text lỗi đúng mã.
- [ ] `codegraphSearch runToolCommand`: 2 hàm (`agent/` dòng 72, `desktop/` dòng 72), `codegraphId` bắt đầu `function:`, không `import`.
- [ ] SQLite `readOnly:true`; ghi thất bại; `mtime` DB không đổi; schema ngoài dải/thiếu `node:sqlite` -> `sqliteReadAvailable:false`, vẫn trả qua CLI.
- [ ] `affected` với `agent/src/relay/agent-tool-registry.ts` ≥ 1 test; `truncated` > 200.
- [ ] Bảng kind phủ 17 loại trong `nodesByKind` thật.
- [ ] `agent/package.json` không đổi.

## 6. Kiểm thử
Trong `/opt/repos/orca/agent`: `pnpm exec vitest run src/relay/codegraph-cli-output.test.ts src/relay/codegraph-index-probe.test.ts src/relay/codeintel-symbol-ref-codegraph.test.ts src/relay/codegraph-sqlite-reader.test.ts src/relay/codegraph-affected-tests.test.ts src/relay/codeintel-codegraph-methods.test.ts` (SQLite test dùng `describe.skipIf` khi thiếu `node:sqlite`). Hợp đồng công cụ thật: AG-CV-SOL-070.

## 7. Rủi ro và điểm chưa kiểm chứng
`node:sqlite` experimental (Node ≥ 22.5; cờ dưới 22.13 chưa kiểm); schema nội bộ; đọc DB 1,2 GB khi daemon ghi WAL; NFS/chỉ-đọc + WAL; telemetry `codegraph telemetry` chưa kiểm; EXPLAIN QUERY PLAN chưa chạy; callers phần lớn là nút `file` (độ chính xác thấp).

## 8. Câu hỏi mở
1. Cách chặn `ExperimentalWarning` không ẩn cảnh báo khác (chủ `agent-entry.ts`).
2. Có dùng `better-sqlite3` dự phòng? Đề xuất không.
3. `confidence` thang số cho cạnh theo tên.

## 9. Tham chiếu
CR-CV-003; contract §2.6, §4.1, §4.14, §10; `external-automations-handler.ts:490-525`.
