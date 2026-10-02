# FE-MCP-SOL-010: Tab "MCP audit" — nhật ký hành động của agent (lọc, phân trang con trỏ, chi tiết, xuất CSV)

> 🔲 Designed — chưa implement.

## CR Reference

- **CR:** [CR-MCP-013](../../../../../../docs/crs/v5/mcp-governance-safety/CR-MCP-013-approvals-audit-killswitch.md) §B ("Truy vấn qua `QueryAuditLog` + export"; UI audit là frontend CR riêng). **Mức độ:** 🟠 P1 (không chặn bật tool, nhưng cần để vận hành an toàn). **Phạm vi:** chỉ FE, chỉ admin tenant.

## Backend dependency (CONTRACT)

| Kênh | BE | Ghi chú |
|---|---|---|
| `mcp.admin.audit.query {from?, to?, userId?, tool?, decision?, cursor?, limit?}` → `{entries: McpAuditEntry[], nextCursor?}` | BE-MCP-SOL-013 §E | lỗi: `MCP_NOT_ADMIN`, `MCP_DISABLED`; tham số sai ⇒ lỗi `MCP_*` (đề xuất R-1 `MCP_INVALID_ARGUMENT`) |
| `adminToolList({})` (chỉ đọc, gợi ý tên tool cho ô lọc) | BE-007/008 | tuỳ chọn; lỗi thì ô lọc là text thuần |
**Điều kiện BE bắt buộc (đã nêu ở BE-013):** `QueryAuditLog` hiện sắp xếp theo `id` (UUID ngẫu nhiên), không theo thời gian — BE-013 thêm thứ tự keyset `(occurred_at DESC, id DESC)`. FE **không tự sắp xếp lại** (sẽ phá ngữ nghĩa `cursor`); nếu BE chưa đổi, danh sách sẽ không theo thứ tự thời gian ⇒ không phát hành tab này trước phần đó. `userName` có thể vắng (BE v1 không tra tên) ⇒ hiển thị `userId` rút gọn. Khi BE chưa có kênh: lỗi + Retry, không mock.

## Phụ thuộc FE-MCP-SOL-001 (tên đã chốt) & FE-MCP-SOL-008

`shared/mcp-types.ts` (`McpAuditEntry`, `McpRisk`), `window.api.mcp`, section `mcp`, `parseMcpError`; tab được đăng ký qua `useMcpGovernanceTabs()` của FE-008 (`value:'audit'`, chỉ admin). Kiểu bổ sung đặt trong `shared/mcp-governance-types.ts` (file của FE-008, thêm additive).

## Impact analysis (gitnexus) — chưa chạy

| Symbol | Lệnh | Rủi ro dự kiến |
|---|---|---|
| `PreloadApi` (`preload/api-types.ts`) | `impact({target:"PreloadApi", direction:"upstream"})` | MEDIUM — chỉ thêm 1 method vào `McpApi` |
| `AuditTab` (`admin-org-console-audit-tab.tsx`) | **không sửa** — không cần impact | — |
Sau khi sửa: `detect_changes({scope:"compare", base_ref:"main"})`.

## Bối cảnh (đã xác nhận lại)

- `components/settings/admin-org-console-audit-tab.tsx` (144 dòng, đã đọc): bộ lọc `from` (date) / `actorId` / `outcome`, bảng 5 cột, gọi `window.api.admin.queryAuditLog({sinceUnixMs, actorId, outcome})`, dùng `Table*`, `Input`, `Select`, `Badge` (`destructive` cho `denied`), `toast.error` khi lỗi, **không** phân trang (không dùng `nextPageToken`), **không** có `to`, chuỗi hard-code tiếng Anh. Khuôn bố cục (`flex items-end gap-2`, nhãn `text-xs text-muted-foreground`, `h-8` input) được tái dùng cho đồng bộ giao diện; **logic** thì viết mới vì cần con trỏ, chi tiết, CSV, i18n.
- **Cùng tồn tại với FE-SOL-003 (v4 team-rbac):** FE-SOL-003 chỉ sửa tab "Audit Log" của `AdminOrgConsole` (filter `actor`/`outcome`, kênh `admin.queryAuditLog`, kiểu `AdminAuditEntry` ở `shared/admin-audit-types.ts`). Solution này **không chạm** các file đó; tab, kênh, kiểu và tên file khác nhau (`mcp-audit-*`/`McpAudit*`). Hệ quả cần biết: các hàng agent cũng xuất hiện ở tab Org Audit (BE ghi chung `auth.audit_log`, `action = mcp.tool_call`) nhưng không có nhãn agent/ chi tiết — chấp nhận; người dùng được hướng dẫn dùng tab "MCP audit" (mô tả một dòng ở đầu tab).
- Nhãn `McpAuditEntry.decision`: `allow|deny` = quyết định chính sách của một lời gọi không cần duyệt; `approved|denied|expired` = kết cục của lời gọi đã qua phê duyệt (BE-013: **một dòng/lời gọi ở trạng thái cuối**). `result` = `ok|error|null` (`null` khi bị chặn/hết hạn trước khi chạy).
- Primitives có: `table`, `badge`, `select`, `input`, `sheet`, `skeleton`, `tooltip`, `button`, `sonner`. Không có `date-picker` ⇒ dùng `<Input type="date">` như tab hiện có.

## Giải pháp

### Bước 1 — API + kiểu

```ts
// shared/mcp-governance-types.ts (MODIFY, additive)
export type McpAuditQueryParams = { from?: string; to?: string; userId?: string; tool?: string
  decision?: McpAuditEntry['decision']; cursor?: string; limit?: number }
export type McpAuditPage = { entries: McpAuditEntry[]; nextCursor?: string }
// McpApi (FE-001) — preload/api-types.ts + web/web-preload-api.ts createMcpApi
adminAuditQuery: (p: McpAuditQueryParams) => callRuntimeResult<McpAuditPage>('mcp.admin.audit.query', p)
```

### Bước 2 — Hook truy vấn (chống đua, hợp nhất theo `id`)

**File:** `renderer/src/components/settings/mcp/useMcpAuditQuery.ts` (NEW)

```ts
type Status = 'loading' | 'ready' | 'loadingMore' | 'error' | 'forbidden'
export function useMcpAuditQuery(filters: McpAuditFilters) {   // filters đã debounce 300ms ở component
  const [state, setState] = useState<{ entries: McpAuditEntry[]; nextCursor?: string; status: Status; error?: string }>(…)
  const seq = useRef(0)                                          // mỗi lần đổi bộ lọc/ reload tăng; phản hồi cũ bị bỏ
  const run = useCallback(async (cursor?: string) => {
    const my = ++seq.current   // (loadMore dùng seq hiện tại, không tăng, và bỏ nếu bộ lọc đã đổi)
    try {
      const page = await window.api.mcp.adminAuditQuery({ ...toParams(filters), cursor, limit: 50 })
      if (my !== seq.current) return
      setState((s) => ({ entries: mergeById(cursor ? s.entries : [], page.entries), nextCursor: page.nextCursor, status: 'ready' }))
    } catch (e) {
      if (my !== seq.current) return
      const { code, message } = parseMcpError(e)
      setState((s) => ({ ...s, status: code === 'MCP_NOT_ADMIN' ? 'forbidden' : 'error', error: message }))
    }
  }, [filters])
  // đổi bộ lọc ⇒ xoá entries + nextCursor + gọi run(undefined); loadMore = run(state.nextCursor)
}
```
`toParams`: `from = startOfLocalDay(fromDate).toISOString()`, `to = endOfLocalDay(toDate).toISOString()` (RFC 3339 UTC, CONTRACT C7); bỏ trường rỗng; `userId` chỉ gửi khi khớp UUID (sai ⇒ hiện gợi ý inline và **không** truy vấn, tránh lỗi BE); `from > to` ⇒ lỗi inline, không truy vấn. Dữ liệu **chỉ ở state component**, mất khi rời tab (không đưa vào store/persist; có chứa `argsSummary` đã che nhưng vẫn là dữ liệu nhạy cảm).

### Bước 3 — Tab

**File:** `renderer/src/components/settings/mcp/McpAuditTab.tsx` (NEW), `McpAuditFilters.tsx` (NEW)

- Dòng mô tả đầu tab: "Actions taken by AI agents through MCP. Direct actions by people are in Organization › Audit Log."
- **Bộ lọc** (nhãn thật bằng `<Label htmlFor>`, không placeholder thay nhãn): *From*/*To* (`Input type=date`) + nút ghost nhanh *24h / 7 days / 30 days*; *User ID* (Input, mô tả "UUID of the person the agent acted for"); *Tool* (Input + `<datalist>` tên tool từ `adminToolList`, rơi về text thuần nếu lỗi); *Decision* (`Select`: All, Allowed, Denied, Approved, Rejected (denied after prompt), Expired — giá trị gửi đúng `allow|deny|approved|denied|expired`); nút *Clear filters* (ghost, chỉ hiện khi có bộ lọc). Text debounce 300 ms; thay đổi giữ nguyên tiêu điểm (SSH-latency).
- **Bảng** (`Table` trong container `overflow-x-auto` — nhiều cột, không tràn trang): **Time** (`<time dateTime={at}>`, `toLocaleString()`, tooltip UTC), **User** (`userName ?? userId.slice(0,8)`+`title` đầy đủ), **Client** (`<Badge variant="outline"><Bot/> Agent</Badge>` + `clientName`), **Tool** (`font-mono text-xs`), **Risk** (Badge: read=`secondary`, write_reversible=`outline`, exec=`outline`+icon, destructive/admin=`destructive`), **Decision** (allow/approved=`secondary`; deny/denied=`destructive`; expired=`outline`; luôn có chữ), **Approver**, **Result** (`ok`/`error`/—), **Duration** (`{ms} ms`), nút chi tiết. Hàng có `tabIndex`/Enter mở chi tiết **và** nút "Details" (`aria-label="Open details for {tool} at {time}"`).
- **Phân trang:** nút `Load more` (không cuộn vô hạn — ổn định cho a11y/SSH) khi `nextCursor`; trạng thái `loadingMore` khoá nút + `aria-busy`; cuối danh sách: "End of results · {N} entries loaded"; lỗi ở trang kế hiện inline dưới bảng với `Retry` mà **giữ** các hàng đã tải. Mục trùng `id` giữa các trang bị loại.
- **Trạng thái UI:** loading (5 hàng `Skeleton`); empty không lọc ("No agent activity yet. Actions taken by connected AI clients will appear here."); empty có lọc ("No entries match these filters" + *Clear filters*); error đầu tiên (`role="alert"` + Retry; không `toast` vì người dùng cần đọc/ sao chép); forbidden (`MCP_NOT_ADMIN` ⇒ "Admin access required"); tab chỉ mount khi `isMcpSurfaceAvailable()` và là admin.

### Bước 4 — Chi tiết hàng

**File:** `McpAuditRowDetail.tsx` (NEW) — `Sheet side="right"` (bề mặt "panel từ cạnh" theo STYLEGUIDE), `SheetTitle` = tool, mô tả = thời điểm. Danh sách trường (`<dl>`): ID, Time (local + ISO UTC), Acting user, Agent (client), MCP session, Tool, Risk, Decision (+ giải thích một dòng theo bảng "allow/deny/approved/denied/expired"), Approver, Result, Duration, Trace ID (nút *Copy* → clipboard, toast "Copied"; **không** gắn liên kết ngoài), **Arguments summary**: `<pre className="font-mono text-xs whitespace-pre-wrap break-all …">{argsSummary}</pre>` (text node, không HTML) kèm chú thích "Secrets are redacted before logging. Full arguments are never stored." Esc/nút đóng trả tiêu điểm về nút kích hoạt (Radix).

### Bước 5 — Xuất CSV (chỉ các hàng đã tải)

**File:** `mcp-audit-csv.ts` (NEW)

```ts
const COLUMNS = ['id','at','userId','userName','clientName','sessionId','tool','risk','decision','approver','result','durationMs','traceId','argsSummary'] as const
const FORMULA_PREFIX = /^[=+\-@\t\r]/   // chống CSV/formula injection khi mở bằng Excel/Sheets (args do agent kiểm soát)
export function toCsv(rows: McpAuditEntry[]): string   // BOM '﻿' + CRLF; ô chứa , " \n \r được bao "…" và nhân đôi "; ô bắt đầu bằng ký tự công thức ⇒ thêm tiền tố '
export function downloadCsv(csv: string, filenameStamp = new Date()) {  // Blob + <a download>, revokeObjectURL sau khi click
  // tên: mcp-audit-YYYYMMDD-HHmm.csv (không dấu ':' — hợp lệ trên Windows)
}
```
Nút "Export loaded rows (CSV)" (outline) `disabled` khi 0 hàng; tooltip/label: "Exports only the {N} rows currently loaded. Load more or narrow the date range first." Không gọi BE thêm (đúng yêu cầu "chỉ các hàng đã tải"). Hoạt động ở web và Electron renderer (anchor-download); không phụ thuộc đường dẫn file (đa nền tảng). Không có xuất toàn bộ phía server trong CONTRACT ⇒ nêu rõ trong "Không làm".

## A11y & i18n & style
- Bảng ngữ nghĩa đủ `TableHead`; mọi badge có chữ (không chỉ màu); bộ lọc có nhãn; vùng kết quả `aria-live="polite"` thông báo "N entries loaded" sau khi tải (không đọc từng hàng); Sheet có `SheetTitle/Description`; phím Tab đi qua bộ lọc → bảng → Load more → Export hợp lý.
- i18n: `translate('auto.mcp.audit.*', 'English')` trong thân component; ngày giờ theo `toLocaleString()`/`Intl.DateTimeFormat` của locale hiện hành. Style chỉ dùng token; `font-mono` cho tool/ID/args.

## Files cần sửa
| File | Action |
|---|---|
| `renderer/src/components/settings/mcp/{McpAuditTab.tsx, McpAuditFilters.tsx, McpAuditRowDetail.tsx, useMcpAuditQuery.ts, mcp-audit-csv.ts}` | NEW |
| `frontend/src/shared/mcp-governance-types.ts` | MODIFY (additive) — `McpAuditQueryParams`, `McpAuditPage` |
| `preload/api-types.ts`, `renderer/src/web/web-preload-api.ts` | MODIFY — `adminAuditQuery` |
| `renderer/src/components/settings/mcp/mcp-governance-tabs.tsx` (FE-008) | đã trỏ tới `McpAuditTab` |
| `renderer/src/i18n/locales/en.json` | MODIFY — `auto.mcp.audit.*` |

## Verification
```bash
cd frontend && npx vitest run src/renderer/src/components/settings/mcp/mcp-audit-csv.test.ts \
  src/renderer/src/components/settings/mcp/useMcpAuditQuery.test.ts src/renderer/src/components/settings/mcp/McpAuditTab.test.tsx
cd frontend && npx vitest run src/renderer/src/i18n/no-top-level-translate.test.ts
cd frontend && npx tsc --noEmit -p tsconfig.json
```
Ca bắt buộc: bộ lọc → tham số đúng (ISO UTC đầu/cuối ngày địa phương, bỏ trường rỗng, `userId` không phải UUID ⇒ không gọi); đổi bộ lọc giữa chừng ⇒ phản hồi cũ bị bỏ (fake promise đảo thứ tự); `Load more` nối trang & loại trùng `id`; lỗi trang kế giữ hàng đã tải; `MCP_NOT_ADMIN` ⇒ forbidden; CSV: dấu phẩy/ngoặc kép/xuống dòng được escape, `=cmd|…`, `+1`, `-1`, `@x`, tab/CR được thêm tiền tố `'`, có BOM và CRLF, tiêu đề cột đúng thứ tự, tên file không chứa `:`; `argsSummary` chứa `<script>` render thành chữ; không dữ liệu audit nào vào store (`useAppStore.getState()` không đổi). Thủ công (cần BE-013): gọi vài tool từ một MCP client (allow, deny, approved) ⇒ các hàng xuất hiện **mới nhất trước**, lọc `decision=approved` chỉ còn hàng đó, `Load more` qua 50 hàng không lặp/không bỏ sót.

## Sửa TDD kèm theo
`specs/frontend/tdd/v4/03-admin-spa.md §8 (AuditPage)` đã lỗi thời so với `AdminOrgConsole`; ghi nhận thêm tab "MCP audit" trong section `mcp` (không thuộc `AdminOrgConsole`). Không sửa FE-SOL-003.

## Không làm ở solution này
Xuất toàn bộ phía server/lưu trữ dài hạn (cần kênh mới), biểu đồ/thống kê, tìm kiếm full-text trong `argsSummary`, hiển thị `userName` (BE v1 chưa cấp), thao tác từ nhật ký (thu hồi/chặn client — thuộc FE-MCP-SOL-003), sửa tab Audit Log của Organization.
