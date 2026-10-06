# FE-CV-SOL-057: Lens ERD (bảng, cột, khoá, liên kết logic giữa service, tô thay đổi từ migration)

> 📋 Proposed. Chưa triển khai. Viết ngày 2026-10-06 từ khảo sát code `frontend/src` và hợp đồng v7; chưa chạy test hay ứng dụng.

**CR:** [CR-CV-057](../../../../../../docs/crs/v7/review-frontend/CR-CV-057-erd-lens.md)
**Area:** frontend (`frontend/src/renderer/src/components/review-map/erd/`, hook, khoá slice)
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (U1 một object `args[0]`, U3 định danh `{projectId, worktreeId}`, U5/§2.3 lỗi theo tiền tố `message`, U6 phong bì phẳng, U7 `CODEINTEL_DISABLED`, §3.1 kênh `erd`, §4.4 `ErdModel`/`ErdServiceInfo`, §4.3 `ChangeOverlay.touchedTables`, §5 push `changed`, §6 cờ), [CONTRACT-codeintel-proto-and-data-map.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (**PQ-02, PQ-03, PQ-04, PQ-12, PQ-13, PQ-28, PQ-29, PQ-32**; §7 G3/G4; §8.2; §8.3; §9 O-1, O-7). **TDD tham chiếu:** [v5/02-state-management](../../../../tdd/v5/02-state-management.md), [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md); [api/rpc-catalog](../../../../api/rpc-catalog.md).

## 1. Trạng thái hiện tại (re-verify)

**Đã xác minh trong phiên soạn này (đọc/ls/grep):**

- `frontend/src/renderer/src/components/review-map/` **chưa tồn tại** (`ls components | grep review` chỉ thấy `code-review`, `github-pr-reviewer-display*`, `task-page-github-review-cells.tsx`); `code-review` là code chết, không dùng.
- `frontend/package.json` có `@xyflow/react ^12.11.2`, `@tanstack/react-virtual ^3.13.24`, `zod`, `yaml`, `mermaid`; **không** có `elkjs`/`dagre` (O5/O12: không thêm). `@testing-library/react`, `@testing-library/user-event`, `happy-dom` có sẵn; `frontend/config/vitest.config.ts` đặt `environment: 'node'`, `include: src/**/*.test.ts(x)`.
- `assets/main.css` có `--git-decoration-added|modified|deleted|renamed|untracked|copied|ignored` ở `:root` (dòng 191–197) và `.dark` (dòng 279–283); **không** có binding `--color-git-decoration-*` trong `@theme inline` (grep trống). STYLEGUIDE chỉ cho phép các token này cho trạng thái git.
- Không có `maskSensitiveText` nào trong `frontend/src` (grep trống).
- `components/ui/` có `table.tsx`, `badge.tsx`, `popover.tsx`, `toggle-group.tsx`, `command.tsx`, `collapsible.tsx`, `dialog.tsx`, `textarea.tsx` (đã ls).

**Theo CR-CV-057 (chưa kiểm lại từng dòng):** `TaskDAGView.tsx`/`DAGPreview.tsx` dùng `@xyflow/react` và hardcode hex (không sao chép style); `EditorOpenTargetOptions` trong `store/slices/editor.ts` không có tham số nhảy dòng (CR-CV-053 bổ sung `pendingDiffReveal`).

### Lệch giữa CR và hợp đồng (hợp đồng thắng)

| # | CR-057 ghi | Hợp đồng | Quyết định trong solution |
|---|---|---|---|
| 1 | `useCodeIntelQuery(worktreeId, 'erd', {service, dialect, base, head})` | Mọi kênh nhận `{projectId, worktreeId, ...}` (U3, PQ-04) | Hook lấy `projectId` qua selector của SOL-050 (O-1: giả định có); không tự ghép |
| 2 | Cần RPC/field liệt kê `services[]` | `erd` **không có `service`** trả `Env<{services: ErdServiceInfo[]}>` (§3.1) | Hai pha: gọi không `service` để dựng bộ chọn, rồi gọi có `service` |
| 3 | `Relation.cross_service`, `to.service` | `ErdRelation.crossService`, `ErdEndpoint.service?` (camelCase, §4.4) | Dùng tên hợp đồng |
| 4 | `Column.isFk/isPk`, `Table.rls?` | `ErdColumn {canonicalType, generated, …}`, `ErdTable.rlsState`, `rls[]`, `tenantScoped`, `degraded` | Hiển thị `canonicalType` ở chi tiết; `degraded` → nhãn "ERD của bảng này có thể thiếu" |
| 5 | `accessedBy: {symbol, op}` | Có `op: read|write|readwrite` + `confidence` (§4.4); chỉ có khi `includeAccess` (mặc định `true`) | Gửi `includeAccess:true`; không có thì chỉ vẽ ERD tĩnh |
| 6 | Quan hệ có `kind fk|logical` | Thêm `source: ddl|declared|comment|naming`, `confidence`, `note`; liên kết khai báo ở `docs/code-intel/erd-links.yaml` (PQ-28, không có UI sửa) | Tooltip nêu `source`; `naming` hiển thị nhãn "suy luận" |
| 7 | `warnings[]` | `ErdModel.warnings {file, line?, code, message}[]` | Dùng nguyên; `message` là chuỗi tự do → văn bản thuần (U9) |
| 8 | `touchedTables[]` chỉ mức bảng | `TouchedTable {table, service, via, migrations[], accessors[]}` (§4.3) | Làm đường lui khi `changes` rỗng |
| 9 | Tên khoá i18n `auto.components.review.map.erd.*` | README nhóm: `auto.components.reviewMap.<Thành phần>.<tên>` | Theo README nhóm (khớp CR-050 2.9) |
| 10 | Hook `use-code-intel-erd.ts` (kebab) | README nhóm chọn `useCodeIntel*.ts` | `useCodeIntelErd.ts` |
| 11 | Câu hỏi 6: dialect "chính" khi không chọn | `ErdServiceInfo.dialects[]` | Mặc định `dialects[0]`; ghi vào câu hỏi mở 3 |
| 12 | "Che `default`/`comment` chưa quyết" | Backend đã che; frontend vẫn che lớp hai (§4.4 cuối) | Áp `maskSensitiveText` (task 057-01) cho `defaultExpr`, `comment`, `checks.expr`, `rls.*Expr` |

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/renderer/src/components/review-map/
  sensitive-text-masking.ts          (mới) maskSensitiveText — dùng chung ERD/Storage/Contract (task 057-01)
  erd/
    erd-view-model.ts                (mới) ErdModel + changes + touchedTables → ErdTableView/ErdRelationView
    erd-column-changes.ts            (mới) gộp ErdChange vào cột, sinh cột/bảng "ma"
    erd-table-filter.ts              (mới) tìm kiếm, schema, chế độ "đổi + liền kề", trần 150
    erd-layout.ts                    (mới) layoutErdTables (tất định, không thêm thư viện)
    erd-repository-links.ts          (mới) accessedBy → nhóm read/write, cờ "symbol đã đổi", cảnh báo liên đới
    ErdLens.tsx, ErdToolbar.tsx, ErdCanvas.tsx, ErdTableNode.tsx, ErdGhostTableNode.tsx,
    ErdSchemaGroupNode.tsx, ErdRelationEdge.tsx, ErdColumnRow.tsx, ErdTableList.tsx,
    ErdLegend.tsx, ErdTableDetail.tsx, ErdWarningsStrip.tsx          (mới)
frontend/src/renderer/src/hooks/useCodeIntelErd.ts          (mới)
frontend/src/renderer/src/store/slices/code-intel.ts        (sửa: SOL-050 sở hữu; thêm khoá `selectedErdTable`, `erdService`, `erdServiceHistory`)
frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json (sửa, khoá auto.components.reviewMap.Erd*)
frontend/src/renderer/src/i18n/code-intel-locale-coverage.test.ts (SOL-050; thêm KEYS)
```
Đăng ký lens: `REVIEW_LENS_DEFINITIONS` (SOL-051) với `id: 'erd'`, `load: () => import('./erd/ErdLens')`, nhận `ReviewLensProps` (`worktreeId, scope, overlay, selectedSymbolKey, chipFilter, onSelectSymbol, onOpenDiff`). Khung (`ReviewViewStateScreen`, `ReviewStateBanners`, `ReviewDrawerContent`) do SOL-051 lo; lens chỉ xử lý trạng thái riêng của ERD.

### 2.2 Dữ liệu và hook

```ts
// hooks/useCodeIntelErd.ts
type ErdQueryArgs = { worktreeId: string; service: string | null; dialect: 'postgres' | 'mysql' | null }
type UseCodeIntelErd = {
  services: { status: QueryStatus; data: ErdServiceInfo[] | null; error: CodeIntelUiError | null }
  model:    { status: QueryStatus; stage?: string; envelope: CodeIntelEnvelope<ErdModel> | null; error: CodeIntelUiError | null }
  reload: () => void
}
```
- Bọc `useCodeIntelQuery(worktreeId, 'erd', params, {enabled})` của SOL-050-store-and-query-hooks: lần 1 `params = {}` (danh sách service), lần 2 `{service, dialect?, base, head, includeAccess:true, includeInferred:false}`. `base`/`head` lấy từ `scope` của khung (O7 merge-base). Lỗi được phân loại bởi bộ phân loại SOL-050 (đọc tiền tố `message`, PQ-02); lens không đọc mã thô.
- Bỏ phản hồi cũ khi đổi `service` (số thứ tự yêu cầu). Push `changed` (§5): đánh dấu cũ + chip "Có dữ liệu mới", **không** tải lại giữa lúc tương tác; `codeIntelResyncCounter` tăng thì tải lại.
- `CODEINTEL_TIMEOUT` có hậu tố `{"inProgress":true,"retryAfterMs":3000}` (PQ-13): hook của SOL-050 tự thử lại ≤ 90 s; lens chỉ hiển thị nhãn "Đang dựng ERD…".
- `ErdModel.asOfMigration`, `schema`, `dialect` hiển thị ở thanh công cụ ("Tính đến migration …").

### 2.3 Mô hình hiển thị (hàm thuần)

```ts
type ErdColumnChange = 'added' | 'removed' | 'modified'
type ErdColumnView = ErdColumn & { change?: ErdColumnChange; before?: ErdChange['before']; ghost?: boolean }
type ErdTableView = Omit<ErdTable, 'columns'> & {
  columns: ErdColumnView[]; tableChange: 'added' | 'removed' | 'modified' | 'untouched' | 'adjacent'
  accessCount: { read: number; write: number }; fromTouchedOnly?: boolean }
type ErdRelationView = ErdRelation & { id: string; change?: 'added' | 'removed' }
buildErdViewModel(model: ErdModel, overlay: ChangeOverlay | null): {
  tables: ErdTableView[]; relations: ErdRelationView[]; ghosts: { service: string; table: string }[]
  degradedToTableLevel: boolean }
```
- `ErdChange` không có `column` ⇒ thay đổi mức bảng (`added` = CREATE TABLE, `removed` = DROP TABLE → bảng "ma" gạch ngang, vì bảng đã khỏi `tables[]`). Có `column` ⇒ cột; `removed` sinh cột "ma" (không có trong `columns[]` cuối), `modified` mang `before/after`.
- `changes` rỗng **và** `overlay.touchedTables` có bảng thuộc service này ⇒ tô mức bảng (`fromTouchedOnly`), nhãn "chưa có chi tiết cột" (suy thoái có kiểm soát; không tự suy diễn cột).
- `externalRefs` + `ErdRelation.crossService` ⇒ nút ma `service.table`; mọi cạnh xuyên service là `logical` (README v7 mục 6 cấm FK chéo service); nếu backend trả `fk` xuyên service thì vẫn vẽ nét đứt và ghi chú "dữ liệu bất thường".
- Cạnh thêm/xoá: nếu thiếu thông tin thì chú thích "chưa phân biệt cạnh đổi" (không bịa).

### 2.4 Bố cục, lọc, tìm kiếm

`layoutErdTables({tables, relations, expandedTables, groupBySchema})` → `{positions, groups}`: thành phần liên thông theo FK, BFS tầng, một lượt barycenter, cô lập xếp lưới; tất định (cùng input ⇒ cùng vị trí). Chiều cao nút = `HEADER_H + min(columns, ERD_COLLAPSED_COLUMN_LIMIT) * ROW_H` hoặc đầy đủ khi `expanded`.

| Hằng (đề xuất, chưa đo) | Giá trị | Vượt ngưỡng |
|---|---|---|
| `ERD_FOCUS_DEFAULT_THRESHOLD` | 25 bảng | mặc định chế độ "Chỉ bảng bị đổi + liền kề"; nút "Hiển thị tất cả (N)" |
| `ERD_MAX_RENDERED_TABLES` | 150 | chỉ vẽ bảng ưu tiên (đổi, liền kề, khớp tìm kiếm, nhiều quan hệ); thông báo "Đang hiển thị X / Y" (không cắt im lặng); phần còn lại ở Danh sách |
| `ERD_COLLAPSED_COLUMN_LIMIT` | 12 | gộp "+N cột" theo thứ tự PK, FK, có `change`, khớp tìm kiếm, còn lại |
| `Envelope.truncated` | — | thông báo riêng "Backend đã giới hạn kết quả" |

Tìm kiếm chuẩn hoá không dấu/chữ thường; khớp tên bảng, cột, `type`/`canonicalType`, tên symbol trong `accessedBy`. `ReactFlow`: `onlyRenderVisibleElements`, `nodesConnectable={false}`, không `MiniMap` khi > 80 nút (chưa kiểm chứng chi phí). Animation `fitView`/`setCenter` đặt `duration: 0` khi `prefers-reduced-motion`.

### 2.5 Wireframe

```
┌ ERD · [service ▾ infra-fleet] [dialect: postgres|mysql] [schema: ▣infra ▢ops] [🔍 /] [Đổi+liền kề|Tất cả] [Đồ thị|Danh sách] ┐
│ Tính đến migration 0042 · ⚠ 2 cảnh báo parse (ErdWarningsStrip)            Chú giải: + thêm ~ đổi − xoá ─ FK ┄ logic │
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ┌ infra.dev_servers ─────[Đổi]┐        ┌ infra.agents ──────────┐      ┌┄ ghost: auth.tenants (auth-service) ┄┐  │
│ │ KeyRound id        uuid  PK │◀──FK───│ Link2 dev_server_id    │┄┄┄┄▶│ bấm → chuyển ERD sang auth-service    │  │
│ │ + vault_ssh_role text null  │        │ …                      │      └─────────────────────────────────────┘  │
│ │ ~ status varchar(32)→text   │        └────────────────────────┘                                                   │
│ │ − old_flag (xoá)            │                                         ┌ Chi tiết (cột phải, SymbolDetailPanel) ┐  │
│ │ … +9 cột                    │                                         │ cột/index/RLS/quan hệ/Mã đọc-ghi (7)   │  │
└─┴─────────────────────────────┴─────────────────────────────────────────┴────────────────────────────────────────┴──┘
```
Ký hiệu `+ ~ −` và nhãn chữ luôn đi cùng màu (không chỉ màu). Màu thêm/đổi/xoá dùng `var(--git-decoration-added|modified|deleted)` (hoặc binding `--color-git-decoration-*` nếu SOL-050 thêm); "liền kề" dùng viền muted; "bị ảnh hưởng" dùng token `--review-affected` (SOL-050 2.10). Không dùng `--git-decoration-*` cho thứ gì ngoài trạng thái git.

### 2.6 Chi tiết bảng và liên kết với code

`ErdTableDetail` cắm vào `SymbolDetailPanel` (SOL-053): `ui/table` cột (tên, `type`→`canonicalType`, null, default (đã mask), PK/FK, "trước → sau"), index (`emulatesPartialUnique` nhãn "mô phỏng unique cục bộ"), `checks`, RLS (`rlsState`, chỉ tên + biểu thức), quan hệ ra/vào, và "Mã đọc/ghi bảng này" (`accessedBy`, lọc Tất cả|Ghi|Đọc, chấm "đã đổi" nếu `symbol.key` ∈ `overlay.changedSymbols`). "Mở" gọi hành động mở symbol của SOL-053; "Xem diff" chỉ bật khi tệp ∈ `overlay.changedFiles`; "Xem migration diff" mở `ErdChange.migrationFile` (nhảy dòng phụ thuộc `pendingDiffReveal` của SOL-053; chưa có thì mở đúng tệp và ghi rõ). Cảnh báo liên đới (nhãn "gợi ý, có thể sai"): cột `removed|modified` mà `accessedBy` còn symbol không nằm trong `changedSymbols`. Không viết "an toàn". `tenantScoped=false` hiển thị trung tính (không gắn kết luận bảo mật; việc đó của CR-038).

### 2.7 Trạng thái và lỗi

Tải theo ngưỡng 100 ms/1 s/3 s (`usePerceivedLoadingStage` của SOL-051; qua SSH trì hoãn hiển thị ~200 ms, khoá điều khiển ngay). Service không có migration: rỗng inline (không phải lỗi). `degraded` hoặc `warnings[]`: `ErdWarningsStrip` persistent inline. Lỗi `CODEINTEL_*` ánh xạ `kind` theo §2.3 hợp đồng: `index-missing`/`tool-unavailable` **không** chặn ERD (ERD đọc file, D6) nhưng `accessedBy` rỗng thì hiện "Chưa có liên kết tới code (chưa lập chỉ mục)" + nút Lập chỉ mục của SOL-051; `no-binding`, `offline`, `forbidden`, `too-large`, `timeout`: dùng trạng thái chung của khung; lỗi riêng ERD là inline có "Thử lại", **không toast**. `CODEINTEL_DISABLED` (`kind:'disabled'`): khung đã ẩn lens; lens không tự xử lý.

### 2.8 i18n, hai render target, a11y

Khoá `auto.components.reviewMap.<Thành phần>.<tên>` (vd `ErdToolbar.search`, `ErdTableNode.moreColumns`, `ErdLegend.logicalLink`, `ErdTableDetail.staleAccessWarning`) đủ en/es/ja/ko/zh; không gọi `translate()` cấp module (`i18n/no-top-level-translate.test.ts`). Chỉ qua `codeIntelClient`/`useCodeIntelQuery` nên cùng mã cho Electron và web (Electron local còn `unsupported` tới khi preload có `codeIntel`, do SOL-050/khung chặn). Mỗi nút `tabIndex=0`, `role="group"`, `aria-label` tổng hợp; Danh sách ảo hoá (`@tanstack/react-virtual`) là lối thoát bàn phím/trình đọc màn hình. Phím cục bộ khi tiêu điểm trong lens và không ở ô nhập: `/`, `f`, `g`/`l`, `Esc`; không phím bổ trợ nên không có nhánh Mac/Windows; chỉ hiển thị chip phím cho phím đã cài.

## 3. Quyết định thiết kế

- **Mặc định "chỉ bảng bị đổi + liền kề"** khi > 25 bảng: lens phục vụ review, đồ thị dày FK khó đọc khi không có thư viện bố cục.
- **Bố cục tự viết, tất định** (O5); chất lượng vừa phải, Danh sách làm lối thoát.
- **Đòi `changes` từ backend** (đã có trong hợp đồng) thay vì UI tự diff hai `ErdModel`.
- **`maskSensitiveText` là mô-đun chung** đặt ở `components/review-map/sensitive-text-masking.ts`, tạo ở 057-01 (lens sớm nhất cần nó) và dùng lại ở SOL-058/059/060; không đặt tên `utils`.
- **Không hiển thị dữ liệu thật**: ERD chỉ có cấu trúc; chuỗi tự do (comment, default, message) là văn bản thuần.

## 4. Phụ thuộc chéo khu vực

| Cần | Solution đối ứng (§8.2) | Ghi chú |
|---|---|---|
| Parse SQL → `ErdModel`, `changes`, `warnings`, `accessedBy` | `BE-CV-SOL-031-sql-migration-parser`; `BE-CV-SOL-031-erd-model-and-access-scan` | Chưa có thì dùng fake backend G4 |
| `touchedTables[]`, `changedSymbols`, `changedFiles` | `BE-CV-SOL-036-change-overlay-pipeline` | Cho đường lui và chấm "đã đổi" |
| Kênh `codeIntel.erd` (đăng ký, lỗi, giới hạn) | `BE-CV-SOL-040-codeintel-view-channels` (nền: `BE-CV-SOL-040-codeintel-channel-foundation`, cổng **G3**) | Frontend thật chỉ sau G3 |
| Bridge, store, `useCodeIntelQuery`, fake backend | `FE-CV-SOL-050-types-and-runtime-bridge`, `FE-CV-SOL-050-store-and-query-hooks`, G4 = `FE-CV-SOL-073-flag-gating-and-web-e2e` (task 073-02) | **Bắt đầu bằng fake backend G4** |
| Khung, `ReviewLensProps`, chip index, `usePerceivedLoadingStage` | `FE-CV-SOL-051-review-workspace-shell` | |
| `SymbolDetailPanel`, mở diff đúng dòng | `FE-CV-SOL-053-impact-lens-and-symbol-detail` | |
| Liên kết sâu từ Hợp đồng/Lưu trữ | `FE-CV-SOL-059-…`, `FE-CV-SOL-058-…` | Dùng chung action `setErdService`/`selectErdTable` |
| Không có việc ở agent | `AG-*`: — | ERD đọc file qua `RepoSourceReader` (D6) |

Thứ tự (§7.2): 050 → 051 → 053 → **057** (đợt 3 MVP). Chạy được sau G3 + `BE-CV-SOL-031`; trước đó dùng fake backend.

## 5. Tiêu chí chấp nhận

- [ ] Lens `erd` có trong `ReviewLensTabs` chỉ khi `effective.codeIntelEnabled` (cờ §6); không gọi kênh nào khi cờ tắt; cùng mã chạy Electron và web.
- [ ] Bộ chọn service dựng từ `ErdServiceInfo[]` (gọi không `service`); service hai dialect có bộ chọn dialect; service không có migration hiện rỗng, không phải lỗi.
- [ ] Nút bảng: tên, schema, PK/FK, kiểu; > 12 cột có "+N cột" mở được bằng chuột và bàn phím.
- [ ] `fk` nét liền, `logical` nét đứt; liên kết xuyên service kết thúc ở nút ma bấm được để chuyển ERD (có "Quay lại").
- [ ] Cột thêm/đổi/xoá tô kèm `+ ~ −` và (đổi) "trước → sau"; bảng mới/bị xoá phân biệt được; khi không có `changes` vẫn chạy, tô mức bảng với chú thích.
- [ ] `ErdTableDetail` liệt kê symbol đọc/ghi với nhãn, đánh dấu "đã đổi", mở symbol/diff qua SOL-053; cảnh báo liên đới có nhãn "gợi ý".
- [ ] Tìm kiếm, lọc schema, giới hạn 150 ("đang hiển thị X/Y"), Danh sách ảo hoá hoạt động.
- [ ] `warnings[]`, `degraded`, `truncated`, `stale`, lỗi `CODEINTEL_*` có UI inline; không toast cho lỗi cần đọc.
- [ ] `defaultExpr`/`comment`/biểu thức RLS đi qua `maskSensitiveText` trước khi render; test chứng minh với dữ liệu giả có DSN.
- [ ] Không hex cứng, mọi chuỗi `translate()` đủ 5 locale, reduced-motion tắt animation, không dùng `components/code-review/*`, không thêm thư viện, không `max-lines` disable.

## 6. Kiểm thử (Vitest + Testing Library)

Hàm thuần (môi trường `node`): `erd-view-model`, `erd-column-changes`, `erd-table-filter`, `erd-layout` (tất định, không chồng nút, nút ma, `expanded`), `erd-repository-links`, `sensitive-text-masking` (bảng DSN/`password=`/token/PEM; chuỗi vô hại như UUID không bị che; idempotent). Component (`// @vitest-environment happy-dom`, mock `@xyflow/react` như mẫu test `TaskDAGView`): `ErdTableNode` (ký hiệu không cần màu, "+N cột", `aria-label`), `ErdToolbar` (dialect chỉ khi > 1), `ErdTableDetail`, `ErdLens` (tải/rỗng/lỗi có thử lại/`truncated`/`warnings`). Hook `useCodeIntelErd`: hai pha, bỏ phản hồi cũ, không tự tải lại khi `changed`. Slice: khoá ERD bị dọn khi xoá worktree (mẫu `generation-records-worktree-removal-leak.test.ts`; `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` của SOL-050). Fixture: ERD rút gọn từ `backend-go/services/infra-fleet-service/migrations/postgres` qua fake backend (dùng chung tệp vàng CR-070 nếu khả thi; chưa kiểm chứng). Kế hoạch hiệu năng vẽ 150 nút cần trình duyệt: **chưa chạy**.

## 7. Rủi ro và điểm chưa kiểm chứng

- Chất lượng bố cục tự viết trên dữ liệu thật (40 bảng dày FK) chưa thử; mặc định "liền kề" + Danh sách là giảm thiểu.
- Hiệu năng xyflow với nút cao, ngưỡng 150/12/25 là đề xuất chưa đo; O-7 (tự viết parser SQL) có thể làm `ErdModel.warnings` nhiều.
- `accessedBy` đến từ quét tên bảng (PQ/§4.4 `confidence`): truy vấn dựng động/ORM bị lỡ; mọi nhãn UI nói đây là gợi ý.
- Cần `projectId` của worktree (O-1) ở mọi lời gọi; nếu SOL-050 không cung cấp được, lens không chạy.
- `--git-decoration-*` chưa bind trong `@theme inline`: dùng `var()` hoặc SOL-050 thêm binding.
- Nhảy dòng trong diff phụ thuộc SOL-053 (`pendingDiffReveal`).
- `ErdChange.line` có thể thiếu; fallback mở đầu tệp.

## 8. Câu hỏi mở

1. `ErdModel.changes` có bao giờ mô tả thay đổi **quan hệ** (FK thêm/xoá)? Hợp đồng chỉ có `table/column`; hiện UI không tô cạnh đổi.
2. ERD toàn hệ thống (nhiều service) có cần ở MVP không? (CR này chỉ theo service.)
3. Dialect mặc định khi service có hai dialect (`infra-fleet-service`): `dialects[0]` hay người dùng nhớ lựa chọn (state phiên)?
4. Có nhớ vị trí kéo thả nút theo `(worktreeId, service, dialect, tableKey)` trong phiên không (chưa kiểm chứng nhu cầu)?
5. `maskSensitiveText` đặt ở 057-01: nếu Phần A (SOL-050..056) đã có mô-đun che tương đương thì dùng lại, tránh hai bản.

## 9. Tham chiếu

`/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-057-erd-lens.md`, `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/frontend/package.json`, `/opt/repos/orca/frontend/config/vitest.config.ts`, `/opt/repos/orca/frontend/src/renderer/src/assets/main.css`, `/opt/repos/orca/frontend/src/renderer/src/components/ui/`, `/opt/repos/orca/frontend/src/renderer/src/store/slices/editor.ts`, `/opt/repos/orca/backend-go/services/infra-fleet-service/migrations/`.
