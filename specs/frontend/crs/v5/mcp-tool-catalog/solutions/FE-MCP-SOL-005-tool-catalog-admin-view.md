# FE-MCP-SOL-005: Tab "Tools" — danh mục tool MCP (admin, chỉ đọc)

> ✅ Implemented (unit/integration tests) — see Gaps. FE-MCP-SOL-001 (kiểu dùng chung, `window.api.mcp`, `mcp-slice`, section `mcp`, khung `McpSettingsPane`) được viết song song: solution này **chỉ dùng tên** do FE-001 chốt, không định nghĩa lại; chỗ phụ thuộc tên chưa chốt đánh dấu "(khớp FE-001 khi merge)". Tab chỉnh policy là FE-MCP-SOL-008; ở đây chỉ **liên kết** sang đó.

## CR Reference

- **CR:** [CR-MCP-007](../../../../../../docs/crs/v5/mcp-tool-catalog/CR-MCP-007-registry-introspection-and-descriptors.md) (mục E "catalog theo tenant") + [CR-MCP-008](../../../../../../docs/crs/v5/mcp-tool-catalog/CR-MCP-008-domain-tool-packs.md) (pack, deny-list) · **Mức độ:** 🟠 P1 (công cụ quan sát cho admin; không chặn BE)
- **Phạm vi (chỉ FE):** màn hình **đọc** catalog tool để admin biết agent có thể làm gì, rủi ro, quyết định hiệu lực (allow / cần phê duyệt / bị chặn) và nguồn quyết định. CR-008 ghi "UI cho người dùng cấu hình pack nào bật" là ngoài phạm vi BE — cấu hình thực hiện ở FE-MCP-SOL-008 (policy), **không** ở đây.

## Backend dependency

| Kênh (CONTRACT §2.2) | Tham số | Kết quả | BE | Khi BE chưa có |
|---|---|---|---|---|
| `mcp.admin.tool.list` | `{namespace?, risk?}` (arg[0] object; FE gửi `{}` rồi lọc phía client) | `McpToolView[]` | BE-MCP-SOL-007 §2.H / 008 (trường `pack`) | lỗi → trạng thái error + Retry; `MCP_DISABLED` → "MCP is turned off"; không crash |

Gọi qua `window.api.mcp.admin.tool.list(params)` — sub-namespace `admin.tool` **thêm** vào `createMcpApi()` của FE-001 (quy ước chung: tên kênh `mcp.a.b.c` → `window.api.mcp.a.b.c`, như peers FE-003/004/011): `callRuntimeResult<McpToolView[]>('mcp.admin.tool.list', params)`. Lỗi `Error("MCP_X: msg")` tách bằng `parseMcpError` (FE-MCP-SOL-003, `lib/mcp-error-code.ts`) hoặc hàm tương đương của FE-001. Mã dùng: `MCP_NOT_ADMIN`, `MCP_DISABLED`. Kiểu: `McpToolView`, `McpRisk`, `McpDecision` từ `frontend/src/shared/mcp-types.ts` (FE-001, đúng CONTRACT §1). Không có kênh/kiểu nào khác.

Ngữ nghĩa phải hiển thị đúng (CONTRACT §1 `McpToolView`): `effective` là quyết định **mặc định cho tenant hiện tại** (không theo user/client); `effectiveSource ∈ default | tenant_policy | hard_deny | kill_switch`; `hardDenied` = deny-list cứng, không policy nào mở được. Mục `hardDenied` có thể là tool được liệt kê chỉ để hiển thị khoá (BE-007 §2 quyết định 2) — chúng **không** xuất hiện trong `tools/list` giao thức.

## Impact analysis (gitnexus) — chưa chạy

| Symbol | Lệnh cần chạy trước khi sửa | Rủi ro dự kiến |
|---|---|---|
| `McpSettingsPane` (FE-001) | `gitnexus impact({target:"McpSettingsPane", direction:"upstream"})` — symbol chưa tồn tại tới khi FE-001 merge | LOW (thêm 1 `TabsContent`) |
| `createMcpApi` (FE-001) | `gitnexus impact({target:"createMcpApi", direction:"upstream"})` | LOW (thêm sub-namespace) |
| `PreloadApi` (`preload/api-types.ts`) | `gitnexus impact({target:"PreloadApi", direction:"upstream"})` | MEDIUM (type dùng rộng; chỉ thêm field optional trong `mcp.admin`) |

## Bối cảnh (đã xác nhận lại bằng mã thật, 2026-10-01)

- Admin UI hiện có: `components/settings/AdminOrgConsole.tsx` — `Tabs` + `TabsTrigger/TabsContent` với file `admin-org-console-<x>-tab.tsx`, nhãn tab hard-code tiếng Anh; gate `currentUser?.role === 'admin'` (mẫu `Settings.tsx`, `useSettingsNavigationMetadata.ts`). Tab MCP mới **không** nằm trong `AdminOrgConsole` mà trong `McpSettingsPane` (FE-001, section `mcp`, thư mục `components/settings/mcp/` theo peers); gate admin áp dụng khi FE-001 render `TabsTrigger value="tools"`.
- Primitive có thật (`ls components/ui`): `table`, `tabs`, `badge`, `select`, `input`, `checkbox`, `toggle-group`, `tooltip`, `skeleton`, `button`, `collapsible`. **Không có** `switch/alert/alert-dialog`. Badge variant thật: `default|secondary|dot|destructive|outline|ghost|link` — không có `warning` ⇒ trạng thái phải mang **icon + chữ**, không chỉ màu (STYLEGUIDE ở `guides/STYLEGUIDE.md`).
- Quy ước test: vitest + `happy-dom` + `createRoot` (không có testing-library; mẫu `components/terminal-pane/CloseTerminalDialog.test.tsx`); lệnh `cd frontend && npx vitest run --config config/vitest.config.ts <đường dẫn>`.
- i18n: `translate('auto.<path>.<id>', 'English')`, cấm gọi ở top-level (`i18n/no-top-level-translate.test.ts`) ⇒ bảng nhãn rủi ro/quyết định dựng **trong hàm/hook** (`useMemo`), không `const` module-level gọi `translate`.

## Giải pháp

### Bước 1 — Hook dữ liệu (không đưa vào slice)
**File:** `frontend/src/renderer/src/components/settings/mcp/use-mcp-tool-catalog.ts` (NEW)
```ts
type CatalogState =
  | { status: 'loading' }
  | { status: 'ready'; tools: McpToolView[] }
  | { status: 'error'; code: string | null; message: string }

export function useMcpToolCatalog(): { state: CatalogState; refresh: () => void } {
  // load(): window.api.mcp.admin.tool.list({}) -> sort (namespace, name)
  // refetch: khi mount, khi cửa sổ lấy lại focus, và khi `serverInfo.killSwitch.active` đổi (selector của mcp-slice, khớp FE-001)
  // chống race: giữ requestSeq; kết quả cũ bị bỏ
}
```
Quyết định: catalog (~vài trăm mục × ~300 byte) là dữ liệu đọc thuần, **không** vào `mcp-slice` (tránh đụng slice FE-001, tránh persist dư thừa). Refetch khi vào lại tab (TabsContent unmount) — nhờ vậy sau khi sửa policy ở FE-008 và quay lại, `effective` luôn mới.

### Bước 2 — Logic nhóm/lọc thuần (dễ test)
**File:** `.../settings/mcp/mcp-tool-catalog-grouping.ts` (NEW)
```ts
export type ToolGroupBy = 'namespace' | 'pack' | 'risk'
export type ToolFilters = { query: string; namespace: string | 'all'; risk: McpRisk | 'all'; pack: 1|2|3|4|'all'; decision: McpDecision | 'all'; hideHardDenied: boolean }

export function filterTools(tools: McpToolView[], f: ToolFilters): McpToolView[]    // query khớp name|title|description (không phân biệt hoa thường)
export function groupTools(tools: McpToolView[], by: ToolGroupBy): Array<{ key: string; label: string; tools: McpToolView[] }>
// thứ tự nhóm: namespace A→Z; pack 1→4 ("Pack 1 — Read" …); risk theo độ nguy hiểm read<write_reversible<exec<destructive<admin
export function summarizeTools(tools: McpToolView[]): { total: number; byDecision: Record<McpDecision, number>; hardDenied: number }
```
Hàm thuần, không import React/`translate`; nhãn nhóm trả `key` và component dịch.

### Bước 3 — Badge rủi ro / quyết định
**File:** `.../settings/mcp/McpToolRiskBadge.tsx`, `McpToolDecisionBadge.tsx` (NEW)

| `risk` | Badge `variant` | Icon (lucide) | Nhãn |
|---|---|---|---|
| `read` | `secondary` | `Eye` | Read |
| `write_reversible` | `outline` | `Pencil` | Write (reversible) |
| `exec` | `outline` | `SquareTerminal` | Execute |
| `destructive` | `destructive` | `Trash2` | Destructive |
| `admin` | `destructive` | `ShieldAlert` | Admin |

| `effective` | `variant` | Icon | Nhãn |
|---|---|---|---|
| `allow` | `secondary` | `Check` | Allowed |
| `require_approval` | `outline` | `UserCheck` | Needs approval |
| `deny` | `destructive` | `Ban` | Blocked |

`effectiveSource` hiển thị bên dưới badge quyết định, chữ nhỏ `text-muted-foreground`: `default` → "Default", `tenant_policy` → "Tenant policy", `kill_switch` → "Kill switch" (icon `Power`), `hard_deny` → icon `Lock` + "Always blocked". `hardDenied` ⇒ ô Tool có icon `Lock` (`aria-label` "Always blocked") + `Tooltip`: "Blocked by a built-in rule. No policy can enable this tool." Màu chỉ bổ trợ; mỗi trạng thái đều có icon + chữ.

### Bước 4 — Tab và bảng
**File:** `.../settings/mcp/McpToolsTab.tsx` (NEW) · `McpToolCatalogTable.tsx` (NEW)
```tsx
export function McpToolsTab(props: { onOpenPolicy?: (target: { tool: string }) => void }): React.JSX.Element
```
Cấu trúc:
1. Đoạn mô tả: "Tools that AI agents can call through MCP. This view is read-only; edit policies in the Policy tab." (`auto.mcp.tools.description`).
2. Banner kill switch (chỉ khi có mục `effectiveSource==='kill_switch'`): `div` token `border-destructive` + icon `OctagonX`: "The MCP kill switch is on: agents cannot call any tool." (không có nút bật/tắt — FE-008).
3. Thanh công cụ: `Input` tìm kiếm; `Select` namespace (từ dữ liệu); `Select` risk; `ToggleGroup` pack (All/1/2/3/4); `Select` decision; `Checkbox` "Hide always-blocked"; `ToggleGroup` "Group by" (Namespace | Pack | Risk, mặc định Namespace); nút `Refresh` (ghost, icon `RefreshCw`). Bộ lọc là state cục bộ; `Clear filters` khi có lọc.
4. Dòng tổng hợp: "N tools · A allowed · P need approval · D blocked · H always blocked" (từ `summarizeTools`).
5. Bảng `ui/table` theo nhóm; header nhóm là hàng có `button` (`aria-expanded`) thu/gọn, mặc định mở nhóm ≤ 12 mục, đóng nhóm lớn hơn (chống cuộn dài). Cột: **Tool** (title đậm + `name` monospace + mô tả cắt 1 dòng, `Tooltip` đầy đủ; render text thuần, không HTML/markdown) · **Namespace** · **Pack** (`Pack n`) · **Risk** · **Scope** (mã `requiredScope`; nhãn qua `describeScope` của FE-003 nếu có, nếu không hiển thị mã) · **Policy** (decision badge + source) · **Hints** (4 icon nhỏ có `Tooltip`: `readOnly` Eye, `destructive` Flame, `idempotent` Repeat, `openWorld` Globe; chỉ hiển thị icon `true`, kèm chú thích "Hints come from the tool author; enforcement is done by policy") · **Actions** (`Button variant="ghost" size="xs"` "Edit policy", gọi `onOpenPolicy({tool: name})`; ẩn nếu không có prop; **ẩn/disabled** cho `hardDenied` với tooltip lý do).
6. Không có ô nhập, không có nút bật/tắt tool ở đây.

`McpSettingsPane` truyền `onOpenPolicy` để chuyển sang tab policy của FE-008 và chọn tool (deep link nội bộ `?section=mcp&tab=<policy tab id của FE-008>&tool=<name>`, tên tab **khớp FE-008 khi merge**).

### Bước 5 — Đăng ký tab (file của FE-001, chỉ thêm)
**File:** `.../settings/mcp/McpSettingsPane.tsx` (FE-001; MODIFY, thêm): `TabsTrigger value="tools"` + `TabsContent` lazy `import('./McpToolsTab')`, chỉ khi `currentUser?.role === 'admin'` **và** `mcpServerInfo.enabled`. Không sửa `Settings.tsx`/`useSettingsNavigationMetadata.ts` (FE-001 đã đăng ký section `mcp`). Thêm mục tìm kiếm Settings (`*-search.ts`) "MCP tools" nếu FE-001 có cơ chế — (khớp FE-001).

### Bước 6 — i18n
Khoá `auto.mcp.tools.*` (mô tả, nhãn cột, nhãn rủi ro/quyết định/nguồn, trạng thái rỗng/lỗi) với fallback tiếng Anh trong `translate(...)`; thêm vào `src/renderer/src/i18n/locales/en.json` (các locale khác để lazy-fallback như quy ước repo). Tên tool/namespace/`name`/scope là định danh — **không dịch**.

## Trạng thái UI

| Trạng thái | Hiển thị |
|---|---|
| loading | 8 hàng `Skeleton`, thanh công cụ disabled, `aria-busy` |
| ready, có dữ liệu | bảng + tổng hợp |
| ready, không có mục khớp lọc | "No tools match these filters." + nút `Clear filters` |
| ready, catalog rỗng | "No tools are exposed yet. Tool packs are enabled on the server." (không phải lỗi) |
| error | khung lỗi + `Retry`; `MCP_DISABLED` ⇒ "MCP is turned off for this organization." (không nút Retry); khác ⇒ thông điệp server (text thuần) |
| forbidden | tab không render cho non-admin; nếu server vẫn trả `MCP_NOT_ADMIN` (role đổi lúc đang mở) ⇒ "You need admin rights to view tools." và ẩn dữ liệu cũ |
| disabled-by-flag | `serverInfo.enabled=false` ⇒ FE-001 không mount tab |
| kill switch | banner + mọi hàng `effective` hiển thị như BE trả (thường `deny/kill_switch`) |

## A11y & i18n & style

Bảng dùng `ui/table` (`<table>`); nhóm là `button aria-expanded aria-controls`; bộ lọc có `Label`/`aria-label`; thứ tự Tab: tìm kiếm → bộ lọc → bảng; không dùng màu làm kênh duy nhất; focus ring theo `Badge`/`Button` token. Token màu từ `assets/main.css` (không hard-code); Cancel/Refresh là ghost; `destructive` chỉ là *trạng thái* (badge), không phải nút hành động ở đây. Văn bản mô tả tool hiển thị bằng text node — không `dangerouslySetInnerHTML`, không markdown (mô tả do server tạo nhưng coi là không tin cậy). Responsive: bảng cuộn ngang trong container; ở viền hẹp ẩn cột Hints/Scope bằng `hidden md:table-cell`.

## Files cần sửa

| File | Loại |
|---|---|
| `renderer/src/components/settings/mcp/McpToolsTab.tsx` | NEW |
| `.../mcp/McpToolCatalogTable.tsx`, `McpToolRiskBadge.tsx`, `McpToolDecisionBadge.tsx` | NEW |
| `.../mcp/use-mcp-tool-catalog.ts`, `mcp-tool-catalog-grouping.ts` | NEW |
| `.../mcp/mcp-tool-catalog-grouping.test.ts`, `McpToolCatalogTable.test.tsx` | NEW |
| `.../mcp/McpSettingsPane.tsx` (FE-001) | MODIFY (thêm tab) |
| `renderer/src/web/web-preload-api.ts`, `preload/api-types.ts` (`createMcpApi`/`PreloadApi`) | MODIFY (thêm `mcp.admin.tool.list`; nếu FE-001 đã tạo `admin` thì chỉ thêm `tool`) |
| `renderer/src/i18n/locales/en.json` | MODIFY (khoá `auto.mcp.tools.*`) |

## Verification

```bash
cd frontend
npx vitest run --config config/vitest.config.ts src/renderer/src/components/settings/mcp/mcp-tool-catalog-grouping.test.ts
npx vitest run --config config/vitest.config.ts src/renderer/src/components/settings/mcp/McpToolCatalogTable.test.tsx
npx vitest run --config config/vitest.config.ts src/renderer/src/i18n/no-top-level-translate.test.ts
npx tsc --noEmit -p tsconfig.json
```
Test: `filterTools`/`groupTools`/`summarizeTools` (bảng); render hàng `hardDenied` có `Lock` + không có nút Edit; `effective=require_approval` hiển thị nhãn + nguồn; mô tả chứa `<img onerror>` render ra text; error `MCP_DISABLED` không có Retry; non-admin không mount (kiểm ở `McpSettingsPane`). E2E (Playwright, nếu FE-012 có hạ tầng): admin thấy tool pack 1 `allow`, pack 3 `require_approval`, mục khoá. Truy cập thủ công: Tab/Shift+Tab, Enter mở nhóm.

## Sửa TDD kèm theo
`specs/frontend/tdd/v5` (index/05-admin): ghi rằng admin MCP nằm trong `McpSettingsPane` (section `mcp`), không trong `AdminOrgConsole`; ghi `components/settings/mcp/` là thư mục tab MCP.

## Không làm ở solution này
Sửa policy, bật/tắt pack, kill switch (FE-MCP-SOL-008); phê duyệt (FE-MCP-SOL-009); xem `inputSchema` tool (CONTRACT `McpToolView` không có — ghi nhận đề xuất tương lai `inputSchema?` nếu cần); thay đổi catalog theo user/client (view chỉ theo tenant — dùng `mcp.admin.policy.explain` ở FE-008).
