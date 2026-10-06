# CR-CV-057 — Lens ERD: bảng, cột, khoá, liên kết logic giữa service, tô thay đổi từ migration

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-057 |
| **Tên** | Lens ERD trong màn Review: nút xyflow là bảng (cột, PK/FK, kiểu), ERD theo từng service, nét đứt cho liên kết logic giữa service, tô cột thêm/xoá/đổi từ diff migration, bảng ↔ symbol repository đọc/ghi, tìm kiếm, lọc schema, giới hạn số bảng, dùng được với bảng lớn |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-050 (kiểu `ErdModel`, `codeIntelClient.call` và `useCodeIntelQuery` cho kênh `codeIntel.erd`, slice, i18n, `useCodeIntelSupport`), CR-CV-051 (khung, `ReviewLensTabs`, chip index, mẫu trạng thái/lỗi), CR-CV-053 (`SymbolDetailPanel`, mở diff đúng dòng); backend CR-CV-031 (parse SQL → ERD, liên kết bảng ↔ code), CR-CV-036 (`touchedTables[]`), CR-CV-040 (kênh `codeIntel.erd`) |
| **Mở khoá** | CR-CV-059 (bảng hợp đồng tham chiếu bảng bị migration chạm), CR-CV-073 (E2E) |
| **Tác động** | `frontend/src/renderer/src/components/review-map/erd/` (mới), `components/review-map/ReviewLensTabs.tsx` (đăng ký lens, CR-CV-051 sở hữu), `store/slices/code-intel.ts` (CR-CV-050 sở hữu; thêm khoá `erd`), `assets/main.css` (token nếu thiếu), `i18n/locales/*.json` |

---

## 1. Bối cảnh và vấn đề

Sau khi agent code, thay đổi ở `migrations/*.sql` là loại thay đổi khó review nhất bằng diff chữ: một `ALTER TABLE` nằm cách `CREATE TABLE` gốc hàng chục file, người review không thấy bảng "sau cùng" trông thế nào, cột mới có FK/index/RLS không, và code nào đang đọc/ghi bảng đó. Nghiên cứu [08 §5](../../../research/view-code/08-views-and-review-models.md) chọn dựng ERD bằng cách parse migration theo thứ tự (không đọc DB thật), mỗi service một ERD, liên kết giữa service là nét đứt logic. Backend (CR-CV-031) trả `ErdModel`; CR này là phần vẽ và tương tác.

Hiện trạng frontend (đã đọc code, 2026-10-05):

- `@xyflow/react` đã được dùng ở `components/task/TaskDAGView.tsx` và `components/workflow/DAGPreview.tsx` (nút tuỳ biến, bố cục theo wave). `DAGPreview.tsx` hardcode hex nên **không sao chép** style của nó; chỉ tham khảo cách khai báo `nodeTypes`.
- Chưa có component bảng-cơ-sở-dữ-liệu, bố cục ERD, hay lớp tô thay đổi nào. Chưa có `components/review-map/` (CR-CV-050/051 sẽ tạo).
- `components/ui/` có sẵn `badge`, `input`, `select`, `toggle-group`, `tooltip`, `popover`, `skeleton`, `scroll-area`, `table`, `command` (đã liệt kê khi đọc thư mục).
- Token màu trạng thái diff: `--git-decoration-added|modified|deleted` có trong `main.css` (cả `:root` và `.dark`) nhưng **chưa được bind** trong `@theme inline` (grep `--color-git-decoration` không có kết quả), nên dùng qua `var(--git-decoration-added)` hoặc thêm binding. STYLEGUIDE ("Git decoration colors") chỉ cho dùng các token này cho trạng thái git; thay đổi cột từ diff migration là trạng thái git của dòng SQL nên hợp lệ, nhưng **không** dùng chúng cho "bảng bị ảnh hưởng" hay cảnh báo khác.
- Thư viện bố cục (`elkjs`, `dagre`) **không có** và mặc định O5 là không thêm.

## 2. Giải pháp đề xuất

### 2.1 Dữ liệu vào và những chỗ hợp đồng phải bổ sung

Dùng `ErdModel` của [08 §5](../../../research/view-code/08-views-and-review-models.md): `{service, dialect, schema, tables[], relations[], asOfMigration}`, `Table {name, schema, columns[], pk[], indexes[], rls?, comment?, accessedBy[]}`, `Column {name, type, nullable, default?, isPk, isFk, comment?}`, `Relation {from:{table,cols}, to:{table,cols}, kind:"fk|logical", cardinality?, cross_service?}`. Phần bọc ngoài là `CodeIntelResult<ErdModel>` (có `stale`, `truncated`, `totalCount`, `sources`).

Lens cần thêm những thứ mà schema trong 08 **chưa có** (không tự bịa phía UI; đã ghi ở "Điều chỉnh hợp đồng" của báo cáo và phải chốt với CR-CV-031/020):

| Cần | Đề xuất tên | Lý do |
|---|---|---|
| Thay đổi theo diff | `ErdModel.changes: ErdChange[]`, `ErdChange {table, column?, kind: 'added'\|'removed'\|'modified', before?: {type, nullable, default}, after?: {...}, migrationFile, line?}` và RPC `GetErd` nhận `base`, `head?` | `ChangeOverlay.touchedTables[]` chỉ cho biết bảng nào bị chạm, không nói cột nào. Cột bị `DROP` không còn trong `columns[]` cuối, nên UI cần `removed` để vẽ "ma" |
| Op của symbol | `Table.accessedBy: {symbol: SymbolRef, op: 'read'\|'write'\|'readwrite'}[]` | 08 §5 nói có cạnh `reads/writes` nhưng trường `accessedBy: SymbolRef[]` không mang op |
| Đầu mút liên kết xuyên service | `Relation.to.service` (và `from.service`) khi `cross_service` | UI cần biết bảng "ma" thuộc service nào để chuyển ERD |
| Cảnh báo parse | `ErdModel.warnings: {file, line?, code, message}[]` | Migration có câu lệnh parser chưa hiểu (ví dụ `DO $$ … $$`); UI phải nói thật là ERD có thể thiếu |
| Danh sách service | RPC/field liệt kê `services[] {name, dialects[], tableCount}` (có thể nằm trong `ErdModel` rỗng hoặc `ChangeOverlay`) | Bộ chọn service cần biết service nào có migration (17 service, ví dụ `infra-fleet-service` có cả `postgres` và `mysql`) |

Nếu backend không trả `changes`, lens vẫn vẽ ERD tĩnh và tô theo `ChangeOverlay.touchedTables[]` ở mức bảng, kèm chú thích "chưa có chi tiết cột" (suy thoái có kiểm soát, xem 2.7).

Hook (mới): `components/review-map/erd/use-code-intel-erd.ts`:

```ts
useCodeIntelErd(args: { worktreeId: string; service: string | null; dialect: 'postgres' | 'mysql' | null }):
  { status: 'idle' | 'loading' | 'ready' | 'error'; stage?: string;
    result: CodeIntelResult<ErdModel> | null; error: CodeIntelUiError | null; reload: () => void }
```

Bọc `useCodeIntelQuery(worktreeId, 'erd', { service, dialect, base, head })` (CR-CV-050 2.7; gọi kênh `codeIntel.erd` qua `codeIntelClient.call`, không gọi `window.api` trực tiếp); kết quả vào `codeIntelResultsByWorktree` với `queryKey = "erd|<scopeKey>|<paramsHash>"` (LRU của slice), nên khoá cache theo commit đã được CR-CV-050 lo. Phạm vi `base/head` lấy từ phạm vi đang chọn ở thanh Review (CR-CV-051; mặc định O7: merge-base). Huỷ yêu cầu cũ khi đổi service (so sánh số thứ tự yêu cầu, không dùng kết quả trễ). Thay `codeIntel.changed` (push) thì đánh dấu cũ và hiện chip "Có dữ liệu mới", không tự tải lại giữa lúc người dùng đang kéo thả.

### 2.2 Cây component

Lens đăng ký vào `REVIEW_LENS_DEFINITIONS` (CR-CV-051 2.6) với `id: 'erd'`, `load: () => import('./erd/ErdLens')`; component nhận `ReviewLensProps` (`worktreeId, scope, overlay, selectedSymbolKey, chipFilter, onSelectSymbol, onOpenDiff`). Nội dung cột phải đi qua `ReviewDrawerContent` (CR-CV-051 2.6). Trạng thái chung (`unsupported`, chưa có index, `stale`, offline, lỗi toàn màn) do `ReviewViewStateScreen`/`ReviewStateBanners` xử lý; lens chỉ xử lý trạng thái riêng của ERD (service không có migration, `warnings[]`, rỗng sau lọc).

```
ErdLens                                  (erd/ErdLens.tsx)
├─ ErdToolbar                            (erd/ErdToolbar.tsx)
│    ├─ ServiceSelect + DialectToggle    (ui/select, ui/toggle-group; dialect chỉ hiện khi service có >1)
│    ├─ SchemaFilter                     (nhóm Badge bật/tắt theo Table.schema)
│    ├─ ErdSearchInput                   (ui/input; phím "/")
│    ├─ ErdScopeToggle                   ("Chỉ bảng bị đổi + liền kề" | "Tất cả")  (ui/toggle-group)
│    └─ ErdViewToggle                    (Đồ thị | Danh sách)
├─ ErdWarningsStrip / ErdTruncatedNotice (inline, không toast)
├─ ErdCanvas                             (erd/ErdCanvas.tsx; ReactFlow)
│    ├─ nodeTypes: erdTable = ErdTableNode, erdGhostTable = ErdGhostTableNode, erdSchemaGroup = ErdSchemaGroupNode
│    └─ edgeTypes: erdRelation = ErdRelationEdge
├─ ErdTableList                          (erd/ErdTableList.tsx; @tanstack/react-virtual; chế độ Danh sách và lối thoát a11y)
├─ ErdLegend                             (chú giải: thêm / đổi / xoá / PK / FK / nét đứt)
└─ (cột phải của khung Review) ErdTableDetail   (erd/ErdTableDetail.tsx; cắm vào SymbolDetailPanel của CR-CV-053)
```

Thành phần chung: `ErdColumnRow.tsx` (một dòng cột), `erd-flow-model.ts` (ErdModel → nodes/edges), `erd-layout.ts` (bố cục), `erd-table-filter.ts` (tìm kiếm, lọc, giới hạn), `erd-column-changes.ts` (gộp `changes` vào cột, sinh cột "ma"), `erd-repository-links.ts` (nhóm `accessedBy`).

### 2.3 Nút bảng (`ErdTableNode`)

```
┌───────────────────────────────────┐
│ ▣ infra.dev_servers         [Đổi] │  header: tên (schema nhạt), chip trạng thái bảng
├───────────────────────────────────┤
│ 🔑 id            uuid        PK   │  PK: icon KeyRound
│ 🔗 tenant_id     uuid        FK   │  FK: icon Link2 (lucide)
│ + vault_ssh_role text  null       │  cột thêm: vạch trái màu --git-decoration-added
│ ~ status         varchar(32)→text │  cột đổi: kiểu trước → sau, --git-decoration-modified
│ − old_flag       bool             │  cột xoá (ma, gạch ngang), --git-decoration-deleted
│ … +9 cột                          │  nút mở rộng
└───────────────────────────────────┘
   ○ handle trái/phải mỗi cột FK (id="col:<name>")
```

- Props: `data: { table: ErdTableView; highlight: 'none' | 'match' | 'dim'; expanded: boolean; onToggleExpand; onSelectRepoSymbol }`. `ErdTableView` là kiểu nội bộ của `erd-flow-model.ts`: `Table` + `columns: ErdColumnView[]` (`change?: 'added'|'removed'|'modified'`, `before?`) + `tableChange?: 'added'|'removed'|'modified'|'untouched'` + `accessCount {read, write}`.
- Trạng thái bảng: bảng mới (`CREATE TABLE` trong phạm vi) viền `--git-decoration-added`; bảng bị `DROP` hiện nút "ma" mờ gạch ngang (vẫn vẽ để review thấy cái bị xoá); bảng chỉ đổi cột thì viền accent trung tính và đánh dấu từng cột; bảng chỉ bị liên đới (liền kề) viền muted. Màu **không phải kênh duy nhất**: luôn kèm ký hiệu `+ ~ −` và nhãn chữ (a11y, đúng quy tắc "color is for state" của STYLEGUIDE).
- Chỉ ghi `PK/FK/UQ/IDX` bằng icon lucide + `Badge` nhỏ; `nullable=false` hiện `NOT NULL` chỉ khi cột là cột đổi (giảm nhiễu).
- Bảng rộng: hiển thị tối đa `ERD_COLLAPSED_COLUMN_LIMIT = 12` cột theo thứ tự ưu tiên: PK, FK, cột có `change`, cột khớp từ khoá tìm kiếm, phần còn lại theo thứ tự khai báo; còn lại gộp thành dòng "+N cột" (nút, `aria-expanded`). Mở rộng thay đổi chiều cao nút nên chạy lại bố cục cục bộ (2.5) có debounce, không nhảy các nút khác quá xa.
- Chọn nút (click hoặc Enter khi focus) đặt `selectedErdTable` vào slice `code-intel` và mở `ErdTableDetail` ở cột phải. Mỗi nút `tabIndex=0`, `role="group"`, `aria-label` tổng hợp ("Bảng dev_servers, 14 cột, 2 cột đã đổi").

### 2.4 Cạnh (`ErdRelationEdge`) và liên kết xuyên service

| Loại | Nét | Điểm đầu/cuối | Ghi chú |
|---|---|---|---|
| `fk` cùng service | liền | handle theo cột (`col:<name>`) | nhãn bản số `1..N` nếu có `cardinality` |
| `logical` cùng service | đứt | handle theo cột | không có ràng buộc DB; tooltip "Liên kết logic (không có FK)" |
| `logical` + `cross_service` | đứt, đậm hơn, đuôi mũi tên mở | bảng "ma" `ErdGhostTableNode` | nút ma chỉ có tên `service.schema.table` và nhãn service; bấm → `setErdService(to.service)` (chuyển ERD, lưu lịch sử để "Quay lại") |

Không vẽ FK chéo service như FK thật: README v7 mục 6 cấm FK chéo service, nên mọi cạnh xuyên service đều `logical`. Quan hệ thêm/xoá trong phạm vi phải tô được: cạnh mới (từ `ErdChange` hoặc `Relation` chỉ có ở `head`) dùng `--git-decoration-added`, cạnh bị bỏ dùng gạch ngang mờ. Nếu `ErdModel` không phân biệt thì chú thích "chưa phân biệt cạnh đổi" (suy thoái).

### 2.5 Bố cục không thêm thư viện (O5)

`erd-layout.ts` xuất hàm thuần:

```ts
layoutErdTables(input: { tables: ErdTableView[]; relations: ErdRelationView[];
  expandedTables: ReadonlySet<string>; groupBySchema: boolean }): {
  positions: Map<string, { x: number; y: number; width: number; height: number }>;
  groups: { schema: string; x: number; y: number; width: number; height: number }[] }
```

Thuật toán đề xuất (tự viết, tất định để test và để vị trí ổn định giữa lần mở): (1) chia thành thành phần liên thông theo FK; (2) trong mỗi thành phần, gán tầng bằng BFS theo hướng FK (bảng được tham chiếu ở tầng thấp hơn), sắp trong tầng theo chỉ số trung bình của hàng xóm để giảm cắt nhau (một lượt barycenter, không lặp tối ưu); (3) thành phần cô lập xếp lưới ở dưới; (4) chiều cao nút = `HEADER_H + rows * ROW_H` với `rows = min(columns, 12)` hoặc tất cả khi `expanded`. Không có animation lặp; chuyển vị trí khi mở rộng dùng `transition` ngắn và tắt khi `prefers-reduced-motion`. Vị trí người dùng kéo tay lưu theo `(worktreeId, service, dialect, tableKey)` trong state phiên (không lưu bền; chưa kiểm chứng nhu cầu).

Giới hạn thật của cách này: đồ thị dày FK vẫn cắt nhau nhiều. Đây là lý do có chế độ "Chỉ bảng bị đổi + liền kề" làm mặc định (2.6) và chế độ Danh sách. Nếu sau thử nghiệm bố cục không đủ tốt, thêm `elkjs` cần duyệt riêng (O5), không nằm trong CR này.

### 2.6 Bảng lớn, giới hạn và tìm kiếm

Số đo thô (đếm `CREATE TABLE` trong `*.up.sql`, chưa tính `ALTER`/`DROP`, chưa chạy parser): `infra-fleet-service` ≈ 40, `auth-service` ≈ 35, `project-service` ≈ 22, `task-service` ≈ 17. ERD theo service nhỏ hơn nhiều so với 588 file migration; rủi ro chính là bảng rộng và đồ thị toàn hệ thống nếu sau này có.

| Hằng (đề xuất, chưa đo) | Giá trị | Hành vi vượt ngưỡng |
|---|---|---|
| `ERD_FOCUS_DEFAULT_THRESHOLD` | 25 bảng | Nếu service có >25 bảng, mặc định bật "Chỉ bảng bị đổi + liền kề"; có nút "Hiển thị tất cả (N)" |
| `ERD_MAX_RENDERED_TABLES` | 150 | Vượt thì chỉ vẽ bảng ưu tiên (bị đổi, liền kề, khớp tìm kiếm, nhiều quan hệ nhất), hiện `ErdTruncatedNotice`: "Đang hiển thị 150 / 312 bảng — thu hẹp bằng schema hoặc tìm kiếm", không cắt im lặng; phần còn lại truy cập qua Danh sách |
| `ERD_COLLAPSED_COLUMN_LIMIT` | 12 | Gộp cột như 2.3 |
| `truncated` từ backend | — | Hiện thông báo riêng "Backend đã giới hạn kết quả", khác với giới hạn vẽ |

`ReactFlow` bật `onlyRenderVisibleElements`, `minZoom`/`maxZoom` hữu hạn, `nodesConnectable={false}`, `elementsSelectable`, không dùng `MiniMap` khi >80 nút (chi phí vẽ; chưa kiểm chứng). Chi tiết cột (kiểu, default, comment, index, RLS) **không** vẽ trong nút mà ở `ErdTableDetail`, để nút nhỏ.

Tìm kiếm (`erd-table-filter.ts`, hàm thuần): chuẩn hoá không dấu/chữ thường; khớp tên bảng, tên cột, kiểu (`uuid`), tên symbol repository. Kết quả: nút khớp `highlight='match'`, còn lại `dim` (giảm độ đậm, không ẩn) khi ở chế độ "Tất cả"; ở chế độ "liền kề" thì khớp được kéo vào tập hiển thị. `Enter` đưa khung nhìn tới khớp đầu (`setCenter`, tắt animation nếu reduced-motion); `Esc` xoá tìm kiếm.

Phím tắt (chỉ khi tiêu điểm trong lens, không ở ô nhập; không cần phím sửa đổi nên không đụng Mac/Windows; chip chỉ hiển thị khi đã cài): `/` tìm kiếm, `f` vừa khung nhìn, `g`/`l` chuyển Đồ thị/Danh sách, `Esc` bỏ chọn. Nếu CR-CV-052 đặt registry phím tắt chung thì đăng ký ở đó thay vì `onKeyDown` rời.

### 2.7 Chi tiết bảng và liên kết với code (`ErdTableDetail`)

Cột phải (cắm vào `SymbolDetailPanel`, CR-CV-053), gồm: tên đầy đủ, `comment`, bảng cột (`ui/table`: tên, kiểu, null, default, PK/FK, trạng thái thay đổi kèm "trước → sau"), index, chính sách RLS (chỉ tên và biểu thức, không có dữ liệu), quan hệ ra/vào, và mục **"Mã đọc/ghi bảng này"** từ `accessedBy`:

```
Mã đọc/ghi bảng này (7)           lọc: [Tất cả|Ghi|Đọc]
  ✎ write  DevServerRepository.Update      adapter/postgres/dev_server_repo.go:88   [Xem diff] [Mở]
  👁 read   DevServerRepository.GetByID     adapter/postgres/dev_server_repo.go:41
  ⚠ symbol đã đổi trong phạm vi review: hiện chấm "đã đổi" (từ ChangeOverlay.changedSymbols khớp SymbolRef.key)
```

- "Mở" gọi hành động mở symbol của CR-CV-053 (`SymbolRef.filePath/startLine`); "Xem diff" chỉ bật khi symbol nằm trong `changedFiles`.
- "Xem migration diff": mở diff file `migrationFile` của `ErdChange` bằng luồng diff hiện có (`openDiff`/`openBranchDiff` trong `store/slices/editor.ts`; **lưu ý** `EditorOpenTargetOptions` hiện không có tham số nhảy tới dòng, nên nhảy tới `ErdChange.line` phụ thuộc CR-CV-053 bổ sung khả năng đó).
- Cảnh báo liên đới (chỉ khi dữ liệu có): cột bị `removed`/`modified` mà `accessedBy` còn symbol **không** nằm trong thay đổi → dòng "Cột bị đổi nhưng N symbol đọc/ghi bảng này chưa được sửa trong phạm vi review", nhãn "gợi ý, có thể sai" vì `accessedBy` đến từ quét tên bảng (08 §5), không phải phân tích dòng dữ liệu. Không khẳng định "an toàn" khi không thấy (xem 10 §7).

### 2.8 Trạng thái và lỗi (theo STYLEGUIDE "UX rule 1")

| Tình huống | Hiển thị |
|---|---|
| Đang tải | <100 ms: giữ nguyên; 100 ms–1 s: vô hiệu hoá bộ chọn; ≥1 s: nhãn + `Loader2`; ≥3 s: nêu giai đoạn ("Đang đọc migration…" → "Đang dựng ERD…") nếu backend gửi `stage`. Qua SSH/dev server: trì hoãn hiển thị tải ~200 ms nhưng khoá điều khiển ngay |
| Service không có migration | Rỗng inline: "Service này không có thư mục migrations" + chọn service khác; không hiện lỗi |
| Chưa có index / công cụ thiếu | ERD không phụ thuộc GitNexus (chỉ bảng ↔ code cần). Nếu `accessedBy` rỗng vì thiếu index thì hiện dòng "Chưa có liên kết tới code (chưa lập chỉ mục)" kèm nút Lập chỉ mục của CR-CV-051; phần ERD vẫn dùng được |
| Parse có cảnh báo (`warnings[]`) | `ErdWarningsStrip` persistent inline, liệt kê file:dòng, "ERD có thể thiếu bảng/cột" |
| Index/HEAD cũ (`stale`) | Banner dùng chung của khung Review; dữ liệu vẫn hiển thị, nêu `headCommit` |
| Lỗi mã chuẩn (`CODEINTEL_PATH_NOT_ALLOWED`, `CODEINTEL_TIMEOUT`, `CODEINTEL_OUTPUT_TOO_LARGE`, …) | Lỗi persistent inline có "Thử lại" và hướng xử lý, **không toast** |
| Dev server mất kết nối | Dùng trạng thái `offline` của khung Review (CR-CV-051: có cache thì banner + dữ liệu cũ, không có thì màn); lens không tự vẽ banner kết nối và không dùng `ConnectionStatusBanner` (chỉ có ở target web) |
| Rỗng sau lọc | "Không bảng nào khớp" + nút "Xoá bộ lọc" |

### 2.9 Hai render target, i18n, token

- Chỉ đi qua `codeIntelClient`/`useCodeIntelQuery` (CR-CV-050) nên cùng mã cho Electron và web; lens không gọi IPC hay `fetch` trực tiếp. Lưu ý CR-CV-050 coi Electron local là `unsupported` cho tới khi preload desktop có `codeIntel`; lens không xử lý trường hợp đó (khung Review đã chặn).
- Mọi chuỗi qua `translate()` với khoá tiền tố `auto.components.review.map.erd.` (ví dụ `ErdToolbar.search`, `ErdTableNode.moreColumns`, `ErdLegend.logicalLink`, `ErdTruncatedNotice.body`, `ErdTableDetail.accessedBy`, `ErdTableDetail.staleAccessWarning`), đủ 5 locale `en/es/ja/ko/zh`. Không gọi `translate()` ở top-level module (có test `i18n/no-top-level-translate.test.ts`).
- Màu chỉ qua biến CSS. Nếu cần token "bị ảnh hưởng/liền kề" riêng thì CR-CV-050 thêm vào `main.css` (cả `:root` và `.dark`) rồi bind `@theme inline`; CR này không tự thêm hex. Không dùng `--git-decoration-*` cho thứ gì ngoài thêm/đổi/xoá.
- `prefers-reduced-motion`: tắt chuyển vị trí nút và animation `fitView`/`setCenter` (`duration: 0`).

## 3. Quyết định thiết kế

- **Mặc định "chỉ bảng bị đổi + liền kề"** thay vì vẽ hết: lens phục vụ review, nên bắt đầu từ thay đổi (nguyên tắc 1 của 10 §2), khớp với việc đồ thị tĩnh dày FK khó đọc khi không có thư viện bố cục.
- **Bố cục tự viết, tất định** (O5): chấp nhận chất lượng vừa phải để khỏi thêm phụ thuộc; có Danh sách làm lối thoát và để tiếp cận bằng bàn phím/trình đọc màn hình.
- **Nút ma cho bảng/cột bị xoá**: review cần thấy cái biến mất; chỉ dựa vào `columns[]` cuối thì mất thông tin. Đòi `ErdChange` từ backend thay vì UI tự diff hai `ErdModel` (tránh gọi hai lần và lệch quy tắc parse).
- **Ký hiệu + màu**, không chỉ màu.
- **Không hiển thị dữ liệu thật**: ERD chỉ có cấu trúc (từ SQL), không có giá trị dòng; không chỗ nào hiển thị `default` chứa secret nếu parser đánh dấu (chưa rõ, xem 6).
- **Chọn bảng không đổi route**: `selectedErdTable` nằm trong slice; liên kết sâu từ lens Hợp đồng (CR-CV-059) dùng cùng action.

## 4. Tiêu chí chấp nhận

- [ ] Lens ERD có trong `ReviewLensTabs` và chỉ hiện khi `useCodeIntelSupport().state === 'enabled'` (cờ `code_intel_enabled` ở backend); hoạt động ở Electron và web với cùng mã.
- [ ] Mỗi nút là một bảng: hiển thị tên, schema, PK/FK, kiểu cột; bảng >12 cột hiện "+N cột" và mở rộng được bằng chuột và bàn phím.
- [ ] ERD hiển thị theo từng service; service có hai dialect có bộ chọn dialect; service không có migration hiện trạng thái rỗng, không phải lỗi.
- [ ] Liên kết `fk` vẽ nét liền, `logical` nét đứt; liên kết xuyên service kết thúc ở nút "ma" mang tên service và bấm được để chuyển ERD.
- [ ] Cột thêm/đổi/xoá từ migration trong phạm vi review được tô kèm ký hiệu `+ ~ −` và (đổi) hiện kiểu trước → sau; bảng mới/bị xoá phân biệt được; không phụ thuộc màu để nhận ra.
- [ ] Khi backend không trả `changes`, lens vẫn chạy và tô ở mức bảng với chú thích rõ.
- [ ] `ErdTableDetail` liệt kê symbol đọc/ghi bảng với nhãn read/write, đánh dấu symbol đã đổi, và mở được symbol/diff qua CR-CV-053.
- [ ] Tìm kiếm khớp tên bảng/cột/kiểu/symbol, làm nổi và dim đúng; `Enter` đưa tới kết quả đầu; `Esc` xoá.
- [ ] Lọc theo schema hoạt động; vượt `ERD_MAX_RENDERED_TABLES` hiển thị thông báo "đang hiển thị X/Y" kèm cách thu hẹp, không cắt im lặng.
- [ ] Chế độ Danh sách ảo hoá liệt kê toàn bộ bảng, chọn từ Danh sách làm nổi/đưa tới nút tương ứng.
- [ ] `warnings[]` của parser hiển thị persistent inline; mọi lỗi `CODEINTEL_*` có UI inline và nút thử lại; không dùng toast cho lỗi cần đọc.
- [ ] Trạng thái tải theo ngưỡng thời lượng, có trì hoãn ~200 ms cho độ trễ SSH, điều khiển khoá ngay.
- [ ] Không có hex cứng trong mã mới; mọi chuỗi qua `translate()` đủ 5 locale; tắt animation khi reduced-motion.
- [ ] Không import từ `components/code-review/*`; không thêm thư viện vào `package.json`; không thêm `max-lines` disable.

## 5. Kiểm thử

Vitest, mẫu theo repo (`renderToStaticMarkup` cho component tĩnh như `DashboardAgentRow.test.tsx`; Testing Library cho tương tác; test hàm thuần cho model):

- Unit hàm thuần: `erd-flow-model` (ErdModel → nodes/edges, cạnh xuyên service sinh nút ma), `erd-layout` (tất định: cùng input cho cùng vị trí; thành phần cô lập; không chồng nút; chiều cao theo `expanded`), `erd-column-changes` (gộp `added/modified/removed`, cột ma, bảng `DROP`), `erd-table-filter` (chuẩn hoá không dấu, ưu tiên khi vượt `ERD_MAX_RENDERED_TABLES`, chế độ liền kề), `erd-repository-links` (nhóm read/write, đánh dấu symbol đã đổi, cảnh báo liên đới).
- Component: `ErdTableNode` (ký hiệu `+ ~ −` có mặt khi không có màu; "+N cột"; `aria-label`), `ErdToolbar` (dialect toggle chỉ khi >1; schema filter), `ErdTableDetail` (read/write, nút bị khoá khi symbol không đổi), `ErdLens` (ba trạng thái: tải, rỗng, lỗi có thử lại; `truncated`; `warnings`).
- Hook: `use-code-intel-erd` bỏ qua phản hồi cũ khi đổi service nhanh; không tự tải lại khi `codeIntel.changed` giữa lúc tương tác (chỉ hiện chip).
- Giả lập `codeIntelClient.call` theo mẫu mock của CR-CV-050; không gọi mạng.
- Tuỳ chọn (cần trình duyệt): kiểm thử hiệu năng vẽ 150 nút bảng; **chưa chạy**, đây là kế hoạch.
- Fixture: dùng ERD thật rút gọn từ `backend-go/services/infra-fleet-service/migrations/postgres` làm fixture vàng chung với CR-CV-070 (chưa kiểm tra tính khả thi của việc dùng chung).

## 6. Rủi ro và điểm chưa kiểm chứng

- **Hợp đồng `ErdModel` thiếu `changes`, `op`, `service` ở đầu mút, `warnings`** (2.1). Nếu CR-CV-031 không bổ sung, tính năng tô cột và cảnh báo liên đới bị cắt giảm. Cần chốt trước khi triển khai.
- **Chất lượng bố cục tự viết** chưa thử trên dữ liệu thật; đồ thị 40 bảng dày FK có thể rối. Mặc định "liền kề" và Danh sách là giảm thiểu, không phải giải pháp.
- **Hiệu năng xyflow với nút cao** (nhiều dòng cột) và ngưỡng 150/12/25 là đề xuất, chưa đo.
- **Độ chính xác `accessedBy`**: quét tên `schema.table` trong truy vấn adapter (08 §5) sẽ lỡ truy vấn dựng động hoặc ORM; mọi nhãn UI phải nói đây là gợi ý.
- **Số cột/bảng thực tế**: chỉ đếm thô `CREATE TABLE`; chưa đo cột lớn nhất mỗi bảng.
- **`--git-decoration-*` chưa bind trong `@theme inline`**: dùng `var()` hoặc thêm binding, cần thống nhất với CR-CV-050.
- Nhảy tới dòng trong diff chưa có sẵn (`EditorOpenTargetOptions` không có tham số dòng).
- Dữ liệu nhạy cảm: `default` hoặc `comment` trong SQL có thể chứa chuỗi giống secret; chưa rõ backend có che không (CR-CV-072). UI nên áp dụng cùng bộ che của CR-CV-058 cho `default`/`comment` (chưa quyết).
- Slice `code-intel` cần test chống rò rỉ khi xoá worktree (10 §9); khoá `erd` phải nằm trong danh sách dọn dẹp của CR-CV-050.

## 7. Câu hỏi mở

1. `GetErd` nhận `base/head` và trả `changes` hay UI gọi hai lần (base và head) rồi tự diff? Khuyến nghị: backend trả `changes`.
2. ERD toàn hệ thống (nhiều service cùng lúc) có cần ở MVP không? CR này chỉ làm theo service.
3. Có cho kéo thả vị trí nút và nhớ qua phiên (lưu ở O6 backend hay chỉ trong phiên)?
4. Cột có `default`/`comment` nghi chứa secret: che ở backend hay UI?
5. Nên thêm `elkjs`/`dagre` (O5) nếu thử nghiệm cho thấy bố cục tự viết không đủ?
6. Thứ tự áp migration khi `infra-fleet-service` có hai thư mục `postgres` và `mysql` có số thứ tự khác nhau: dialect nào là "chính" khi không chọn?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (3.4, 3.6, 3.7, O5, O7, O8)
- `/opt/repos/orca/docs/research/view-code/08-views-and-review-models.md` (§5 ERD, §7 R4)
- `/opt/repos/orca/docs/research/view-code/05-graph-schemas.md` (§3 `SymbolRef`)
- `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§4, §5, §7, §8, §9)
- `/opt/repos/orca/guides/STYLEGUIDE.md` (Git decoration colors, UX rule 1, SSH/latency, reduced-motion)
- `/opt/repos/orca/frontend/src/renderer/src/components/task/TaskDAGView.tsx`, `components/workflow/DAGPreview.tsx` (cách dùng `@xyflow/react`; không sao chép hex)
- `/opt/repos/orca/frontend/src/renderer/src/assets/main.css` (`--git-decoration-*`, `@theme inline`)
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/editor.ts` (`openDiff`, `openBranchDiff`, `EditorOpenTargetOptions`)
- `/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-050-review-frontend-foundation.md` (2.3 `codeIntelClient`, 2.7 `useCodeIntelQuery`) và `CR-CV-051-review-workspace-shell.md` (2.6 `REVIEW_LENS_DEFINITIONS`, `ReviewLensProps`, 2.7 trạng thái)
- `/opt/repos/orca/frontend/src/renderer/src/components/dashboard/DashboardAgentRow.test.tsx` (mẫu test)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/migrations/{postgres,mysql}`
- Mới: `components/review-map/erd/{ErdLens,ErdToolbar,ErdCanvas,ErdTableNode,ErdGhostTableNode,ErdSchemaGroupNode,ErdRelationEdge,ErdColumnRow,ErdTableList,ErdLegend,ErdTableDetail}.tsx`, `erd-flow-model.ts`, `erd-layout.ts`, `erd-table-filter.ts`, `erd-column-changes.ts`, `erd-repository-links.ts`, `use-code-intel-erd.ts`
