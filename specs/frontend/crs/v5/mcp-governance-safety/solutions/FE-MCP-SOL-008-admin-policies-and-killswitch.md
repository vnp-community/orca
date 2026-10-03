# FE-MCP-SOL-008: Tab "Policies" (policy tool, cài đặt tenant) + bảng Kill switch + banner

> ✅ Implemented (unit/integration tests) — see Gaps.

## CR Reference

- **CR:** [CR-MCP-012](../../../../../../docs/crs/v5/mcp-governance-safety/CR-MCP-012-tool-policy-and-annotations.md) §E (UI admin do frontend CR riêng) + [CR-MCP-013](../../../../../../docs/crs/v5/mcp-governance-safety/CR-MCP-013-approvals-audit-killswitch.md) §C (kill switch).
- **Mức độ:** 🔴 P0 (cùng BE-012/013 là điều kiện bật tool ghi). **Phạm vi:** chỉ phần FE — không có logic quyết định nào ở client; mọi kiểm tra client chỉ để báo sớm, **server là nguồn quyết định** (`MCP_POLICY_HARD_DENY` vẫn xử lý).

## Backend dependency (CONTRACT)

| Kênh (`window.api.mcp.*`, camelCase hoá kênh) | BE | Dùng cho |
|---|---|---|
| `adminSettingsGet` / `adminSettingsSet` (`mcp.admin.settings.get/set`) | BE-MCP-SOL-012 | form cài đặt tenant |
| `adminPolicyList` / `adminPolicyUpsert` / `adminPolicyDelete` / `adminPolicyExplain` | BE-MCP-SOL-012 | tab Policies |
| `adminKillswitchSet` (`mcp.admin.killswitch.set`) | BE-MCP-SOL-013 | panel kill switch |
| `adminToolList({})`, `adminClientList`, `adminGrantList({})`, `adminSessionList` | BE-007/008, 005, 004 | **chỉ đọc** để dựng bộ chọn tool/client/grant/session |
| `serverInfo` + sự kiện `killswitch.changed` | BE-003 / BE-013 | banner; làm tươi `mcpServerInfo.killSwitch` |

Mã lỗi dùng: `MCP_NOT_ADMIN`, `MCP_DISABLED`, `MCP_POLICY_HARD_DENY`, `MCP_POLICY_VERSION_CONFLICT`, `MCP_NOT_FOUND`. Mã lạ (vd. `MCP_INVALID_ARGUMENT` nếu R-1 được duyệt) ⇒ hiển thị `message` dưới ô liên quan, không crash. **Khi BE chưa có**: `callRuntimeResult` ném lỗi (kênh `notImplementedHandler`) ⇒ tab hiện trạng thái lỗi + nút Retry; không mock dữ liệu.
**Giới hạn CONTRACT cần biết:** không có kênh *liệt kê* kill switch theo client/grant/session ⇒ panel chỉ hiển thị được trạng thái **tenant** (từ `serverInfo.killSwitch`); công tắc phạm vi hẹp chỉ gỡ được bằng cách chọn lại đúng đối tượng (đề xuất R-6: `mcp.admin.killswitch.list`).

## Phụ thuộc FE-MCP-SOL-001 (đã chốt tên, **không** định nghĩa lại)

`frontend/src/shared/mcp-types.ts` (mọi `Mcp*` trong CONTRACT §1), `window.api.mcp` (+ kiểu trong `preload/api-types.ts:927 PreloadApi`), slice `store/slices/mcp-slice.ts` (cần: `mcpServerInfo`, `refreshMcpServerInfo()`), section Settings `mcp`, và 3 thứ FE-001 phải cung cấp (giả định, chốt khi merge): `parseMcpError(err): {code: string|null; message: string}` (tách `^([A-Z0-9_]+): (.*)$`, CONTRACT C4), `isMcpSurfaceAvailable()`, container `McpSettingsSection` nhận danh sách tab từ `useMcpGovernanceTabs()` bên dưới (một dòng additive).

## Impact analysis (gitnexus) — chưa chạy, chạy ngay trước khi sửa

| Symbol | Lệnh | Rủi ro dự kiến |
|---|---|---|
| `PreloadApi` (`preload/api-types.ts`) | `impact({target:"PreloadApi", direction:"upstream"})` | MEDIUM (kiểu dùng rộng) — FE-001 đã sửa; solution này chỉ thêm method vào `McpApi` |
| `McpSettingsSection` (FE-001, chưa tồn tại) | chạy sau khi FE-001 merge | LOW |
Sau khi sửa: `detect_changes({scope:"compare", base_ref:"main"})`.

## Bối cảnh (đã xác nhận lại trong code)

- Tab admin hiện có: `components/settings/AdminOrgConsole.tsx` với `TabsTrigger value="policies"` (access-policy `admin.listPolicies`) và `audit` — **trùng tên** "Policies"/"Audit" ⇒ tab MCP **không** thêm vào console đó mà sống trong section `mcp` với nhãn **"Tool policies"** để không nhầm hai khái niệm.
- Gate admin: `useAppStore((s) => s.currentUser?.role === 'admin')` (`Settings.tsx:283`, `DepartmentGate.tsx:29`).
- Primitives có: `dialog`, `tabs`, `table`, `badge`, `select`, `checkbox`, `input`, `textarea`, `toggle-group`, `popover`, `command`, `tooltip`, `skeleton`, `sonner`; **không có** `switch`/`alert`/`alert-dialog`. Có `useConfirmationDialog()` (`components/confirmation-dialog.tsx:120`, `confirmVariant: 'default'|'destructive'`).
- `STYLEGUIDE.md` thật ở `guides/STYLEGUIDE.md`: `destructive` chỉ cho "lose data or can't be undone"; Cancel/Dismiss = ghost (dòng 296); Dialog cho quyết định bắt buộc; lỗi cần đọc/ sửa thì **inline**, toast cho thoáng qua.
- Gọi RPC: `callRuntimeResult(method, params)` (`web/web-preload-api.ts:3498`; **không** nằm ở `runtime/web/` như README FE v5 ghi) — một `params`, lỗi `Error(message)`.

## Quyết định thiết kế (có lý do)

1. **Kill switch dùng `destructive` cho nút *Activate* và chỉ nút đó.** Theo STYLEGUIDE, destructive dành cho hành động mất dữ liệu/không hoàn tác: kích hoạt **huỷ tool đang chạy** (mất công việc dở), **đóng phiên** và **thu hồi OAuth refresh token** (client phải ủy quyền lại) ⇒ đủ điều kiện. *Resume* là `default` (khôi phục, không mất gì); *Cancel* ghost. Xoá policy cũng `destructive` (mất một quy tắc). Banner dùng màu trạng thái lỗi vì là *trạng thái hệ thống*, không phải nút.
2. **Xác nhận gõ chữ `STOP`** (hằng, không dịch — tránh lệch locale) cho mọi phạm vi khi *kích hoạt*; *Resume* chỉ cần một cú click xác nhận. Lý do: tác động lên mọi người dùng tenant, không có undo.
3. Quy tắc "strictest wins" (BE-012 QĐ3) hiển thị cố định cạnh ô Decision; **không** đưa khái niệm "độ ưu tiên/thứ tự" vì server không có.
4. Tab chỉ render khi admin; người thường không thấy (BE vẫn trả `MCP_NOT_ADMIN` nếu bị gọi trực tiếp).

## Giải pháp

### Bước 1 — Kiểu bổ sung (không đụng `mcp-types.ts` của FE-001)

**File:** `frontend/src/shared/mcp-governance-types.ts` (NEW)

```ts
import type { McpDecision, McpToolPolicy } from './mcp-types'

export type McpAdminSettings = {
  enabled: boolean; dcrEnabled: boolean; maxTokenDays: number; approvalTtlSeconds: number
  killSwitch: { active: boolean; reason?: string; at?: string }
}
export type McpAdminSettingsPatch = Partial<Omit<McpAdminSettings, 'killSwitch'>> // CONTRACT: không gửi killSwitch
export type McpKillSwitchScope = 'tenant' | 'client' | 'grant' | 'session'
export type McpKillSwitchRequest = { scope: McpKillSwitchScope; targetId?: string; active: boolean; reason: string }
export type McpPolicyExplainRequest = { tool: string; userId?: string; clientId?: string }
export type McpPolicyExplainResult = { decision: McpDecision; reasons: string[] }
// Tạo mới: không id/version (CONTRACT: "không id ⇒ tạo mới"); sửa: id + version hiện tại.
export type McpToolPolicyDraft = Pick<McpToolPolicy, 'match' | 'decision' | 'note'> & Partial<Pick<McpToolPolicy, 'id' | 'version'>>
```

### Bước 2 — `window.api.mcp` (thêm method vào `McpApi` của FE-001; mỗi kênh một dòng)

**File:** `frontend/src/preload/api-types.ts` (MODIFY, `McpApi`) · `frontend/src/renderer/src/web/web-preload-api.ts` (MODIFY, `createMcpApi`)

```ts
adminSettingsGet: () => callRuntimeResult<McpAdminSettings>('mcp.admin.settings.get')
adminSettingsSet: (patch: McpAdminSettingsPatch) => callRuntimeResult<McpAdminSettings>('mcp.admin.settings.set', patch)
adminPolicyList: () => callRuntimeResult<McpToolPolicy[]>('mcp.admin.policy.list')
adminPolicyUpsert: (p: McpToolPolicyDraft) => callRuntimeResult<McpToolPolicy>('mcp.admin.policy.upsert', p)
adminPolicyDelete: (policyId: string) => callRuntimeResult<{ ok: true }>('mcp.admin.policy.delete', { policyId }) // một object ở args[0] (R-2)
adminPolicyExplain: (r: McpPolicyExplainRequest) => callRuntimeResult<McpPolicyExplainResult>('mcp.admin.policy.explain', r)
adminKillswitchSet: (r: McpKillSwitchRequest) => callRuntimeResult<{ ok: true }>('mcp.admin.killswitch.set', r)
```
Bản Electron không kết nối runtime từ xa: các method trả lỗi "not available" như các namespace web-only khác (theo FE-001) — tab không mount khi `!isMcpSurfaceAvailable()`.

### Bước 3 — Logic thuần (test được, không React)

**File:** `frontend/src/renderer/src/components/settings/mcp/mcp-policy-form.ts` (NEW)

```ts
export const EXACT_TOOL_REQUIRED_RISKS: ReadonlySet<McpRisk> = new Set(['exec', 'destructive'])

// Phản chiếu cách server mở rộng `match` (tool/namespace/risk thu hẹp; clientId/roles không thu hẹp tập tool).
export function expandMatch(match: McpToolPolicy['match'], tools: McpToolView[]): McpToolView[] {
  return tools.filter((t) =>
    (!match.tool || t.name === match.tool) && (!match.namespace || t.namespace === match.namespace) && (!match.risk || t.risk === match.risk))
}
export type PolicyIssue =
  | { kind: 'no_dimension' } | { kind: 'hard_deny'; tools: string[] }
  | { kind: 'exact_tool_required'; risks: McpRisk[] } | { kind: 'admin_still_needs_admin_role' }
export function validatePolicyDraft(d: McpToolPolicyDraft, tools: McpToolView[]): { errors: PolicyIssue[]; warnings: PolicyIssue[]; matched: McpToolView[] } { /* … */ }
// errors: no_dimension; hard_deny khi decision !== 'deny' ∧ matched có hardDenied; exact_tool_required khi
// decision==='allow' ∧ !match.tool ∧ matched có exec/destructive. warnings: admin_still_needs_admin_role.
export function buildUpsertPayload(draft: McpToolPolicyDraft): McpToolPolicyDraft // bỏ chiều rỗng/'' (BE: JSON null làm Rego sai), roles [] → bỏ
export function reasonLabel(code: string): { text: string; raw: string } // ánh xạ từ vựng reasons của BE-012; mã lạ → {raw, raw}
```
Từ vựng `reasons` (BE-012): `kill_switch_active, hard_deny, tool_descriptor_incomplete, mcp_disabled_for_tenant, client_not_allowed, scope_missing, admin_role_required, depth_limit, policy:<id>:<decision>, risk_default:<risk>=<decision>, risk_floor:exact_tool_policy_required, open_world_after_untrusted_read, policy_undefined, policy_unavailable`. Văn bản qua `translate('auto.mcp.reason.<code>', 'English')` (gọi **trong** hàm, không top-level).

### Bước 4 — Tab "Tool policies"

**File:** `components/settings/mcp/McpPoliciesTab.tsx` (NEW) — chứa ba khối theo thứ tự: `<McpTenantSettingsForm />`, `<McpKillSwitchPanel />`, bảng policy.

- Tải song song `adminPolicyList` + `adminToolList({})` (để mở rộng match & hiển thị khoá hard-deny). Bảng: cột **Match** (chip cho từng chiều đặt: `tool`, `namespace`, `risk`, `client` (tên tra từ `adminClientList`), `roles`), **Decision** (`Badge`: allow=`secondary`, require_approval=`outline`, deny=`destructive`), **Updated** (`updatedAt` tương đối + `updatedBy`), **v{version}**, hành động (Edit, Explain, Delete — `Button variant="ghost" size="icon-sm"` có `aria-label`+Tooltip).
- Nút đầu bảng: `Add policy` (primary) và `Explain a tool call` (ghost/outline).
- Trạng thái UI: loading (`Skeleton` 3 hàng); empty ("No custom policies. Tools follow Orca's defaults: read/write allowed, exec/destructive need approval, admin denied." + nút Add); error (div `role="alert"` + `Retry`); forbidden (`MCP_NOT_ADMIN` ⇒ "Admin access required"); disabled-by-flag (không mount); khi `enabled=false` của tenant: banner thông tin "MCP is turned off for your organization" phía trên (từ `adminSettingsGet`).
- Xoá: `useConfirmationDialog()({ title, description:'This rule stops applying immediately…', confirmLabel:'Delete', confirmVariant:'destructive' })` → `adminPolicyDelete` → reload. `MCP_NOT_FOUND` ⇒ coi như đã xoá, reload.

### Bước 5 — Dialog tạo/sửa policy (match builder)

**File:** `components/settings/mcp/McpPolicyEditorDialog.tsx` (NEW)

- Trường (đều tuỳ chọn "Any" mặc định, **ít nhất một** — nếu không `no_dimension` chặn Save): **Tool** (Popover+Command có tìm kiếm; mục hard-deny hiện biểu tượng khoá + chú thích "Permanently denied", chọn được nhưng sẽ kích `hard_deny`), **Namespace** (`Select` suy ra từ catalog), **Risk** (`Select`: 5 mức), **Client** (`Select` từ `adminClientList`, value = `clientId`), **Roles** (hai `Checkbox`: Admins, Users). **Decision**: `ToggleGroup` 3 lựa chọn, mỗi lựa chọn có mô tả ngắn; dòng cố định bên dưới: *"If several rules match, the strictest decision wins (Deny › Require approval › Allow)."* **Note** (`Textarea`, ≤500, đếm ký tự).
- Khối "Impact" tính bằng `expandMatch`: "Matches N tools" + tối đa 5 tên + "(M permanently denied)". `validatePolicyDraft` chạy mỗi lần đổi; `errors` ⇒ Save `disabled` + giải thích bằng văn bản liên kết `aria-describedby`; `warnings` hiển thị nhưng cho lưu.
- Lưu: `adminPolicyUpsert(buildUpsertPayload(draft))` (sửa: kèm `id` + `version` đang giữ). Disable nút ngay khi gửi (SSH-latency); Esc/Cancel (ghost) đóng, không gửi.
- Lỗi theo mã (`parseMcpError`): `MCP_POLICY_HARD_DENY` ⇒ inline đỏ tại khối Impact (server là chuẩn — kể cả khi client không phát hiện, vd. catalog cũ); `MCP_POLICY_VERSION_CONFLICT` ⇒ khối `role="alert"` "This rule was changed by someone else." với hai nút: **Reload latest** (gọi lại `adminPolicyList`, nạp bản mới vào form, giữ nháp của người dùng ở dạng "Your draft" để sao chép tay) và **Overwrite with my changes** (upsert lại với `version` mới nhất — chỉ khi người dùng bấm); `MCP_NOT_FOUND` ⇒ "This rule no longer exists" + đóng và reload; `MCP_NOT_ADMIN` ⇒ đóng + trạng thái forbidden; mã khác ⇒ hiển thị `message`.
- A11y: `DialogTitle`/`DialogDescription` đủ; focus ban đầu vào ô Decision (hành động quan trọng nhất); thông báo lỗi nối `aria-describedby`; `ToggleGroup type="single"` có `aria-label`.

### Bước 6 — Dialog "Explain"

**File:** `McpPolicyExplainDialog.tsx` (NEW). Trường: Tool (bắt buộc, cùng Command combobox), User ID (tuỳ chọn, `Input` + helper "Leave empty to evaluate as a regular user"), Client (tuỳ chọn). Nút `Explain` → `adminPolicyExplain`. Kết quả: `Badge` quyết định + danh sách reasons (`reasonLabel`: câu dịch + mã gốc `font-mono text-xs text-muted-foreground`; `policy:<id>:<decision>` → liên kết cuộn tới hàng policy đó). Dòng chú thích: "Rate limits and a pending approval's state are not part of this result." Lỗi `MCP_NOT_FOUND` ⇒ "Unknown tool". Không bao giờ gửi kết quả này đi đâu (nội bộ admin).

### Bước 7 — Form cài đặt tenant

**File:** `McpTenantSettingsForm.tsx` (NEW). `adminSettingsGet` → trạng thái form. Trường: **Enable MCP for this organization** (`Checkbox`; **D6:** tenant mới có giá trị mặc định `true` theo `MCP_TENANT_DEFAULT_ENABLED` của mcp-service nên form thường mở ra ở trạng thái đã bật — admin vẫn tắt/bật lại được; FE không giả định mặc định, luôn đọc từ `adminSettingsGet`), **Allow dynamic client registration** (`Checkbox`; mô tả rủi ro "Any MCP client can register itself and ask users for consent"), **Max token lifetime (days)** (`Input type=number` 1–90, mô tả "Applies to personal access tokens"), **Approval expiry (seconds)** (`Input type=number` 30–900). Chỉ gửi **trường đã đổi** (`McpAdminSettingsPatch`). Validate biên ở client; vượt ⇒ lỗi dưới ô. Tắt `enabled` ⇒ `useConfirmationDialog` ("AI clients will be rejected until re-enabled; existing sessions are not closed — use the kill switch for that.") — *confirmVariant default* (không mất dữ liệu). Save/Reset; thành công ⇒ nạp lại từ phản hồi + toast ngắn "Settings saved". Trạng thái `enabled=false` hiển thị empty-state với mô tả và checkbox ở vị trí nổi bật. Không có optimistic-lock cho settings (CONTRACT không có `version`) ⇒ ghi chú nhỏ "Last save wins".

### Bước 8 — Panel Kill switch

**File:** `McpKillSwitchPanel.tsx` (NEW) + `McpKillSwitchConfirmDialog.tsx` (NEW)

```tsx
// Trạng thái tenant từ store (không tự giữ bản sao): useAppStore((s) => s.mcpServerInfo?.killSwitch)
<section aria-labelledby="mcp-ks-title">
  <h3 id="mcp-ks-title">{translate('auto.mcp.killswitch.title', 'Kill switch')}</h3>
  <p>{translate('auto.mcp.killswitch.desc', 'Immediately stop AI access. Running tools are cancelled and sessions are closed.')}</p>
  <Select value={scope} …>{/* Entire organization | OAuth client | Grant | Session */}</Select>
  {scope !== 'tenant' && <Select value={targetId} …>{/* adminClientList | adminGrantList({}) | adminSessionList */}</Select>}
  <Textarea aria-label="Reason" minLength={3} maxLength={500} />   {/* bắt buộc 3–500 ký tự (cả khi Resume) */}
  <Button variant="destructive" disabled={!valid}>Activate kill switch…</Button>   {/* mở confirm dialog */}
  {tenantActive || scope!=='tenant' ? <Button variant="default">Resume…</Button> : null}
</section>
```
Confirm dialog (Activate): nêu **tác động theo phạm vi** (tenant: "all MCP access stops within about a minute; running tools are cancelled; sessions closed; OAuth refresh tokens revoked; personal access tokens are blocked until you resume"), ô nhập `STOP` (so khớp sau `trim()`, phân biệt hoa thường), nút **Activate** `variant="destructive"` `disabled` tới khi khớp; Enter trong ô chỉ submit khi đã khớp; Cancel = ghost; focus ban đầu vào ô nhập. Gửi `adminKillswitchSet({scope, targetId?, active:true, reason})`. Sau thành công: toast "Kill switch activated", `refreshMcpServerInfo()`. Resume dialog: không gõ chữ; nút `default` "Resume access" gửi `active:false` với cùng `reason` (lý do khôi phục). `MCP_NOT_FOUND` (mục tiêu đã biến mất) ⇒ lỗi inline; `MCP_NOT_ADMIN` ⇒ forbidden.
Tác động ≤60s ở server: UI ghi rõ "within about a minute" — không hứa tức thời.

### Bước 9 — Banner

**File:** `McpKillSwitchBanner.tsx` (NEW) — đọc `mcpServerInfo?.killSwitch`; `active` ⇒ `<div role="status">` (icon `ShieldAlert`, "MCP access is suspended for your organization", lý do + thời điểm `at` định dạng theo locale; admin thêm nút ghost "Manage" chuyển tab Policies). Màu bằng token (`border-destructive/40 bg-destructive/10`), không hard-code. **Làm tươi**: FE-001 chuyển sự kiện `killswitch.changed` thành `refreshMcpServerInfo()` (nguồn sự thật là `serverInfo`, không dùng payload sự kiện trực tiếp). Đặt ở đầu `McpSettingsSection` — hiển thị trên **mọi tab**; `McpApprovalPrompt` (FE-009) cũng nhúng bản rút gọn.

### Bước 10 — Đăng ký tab vào container của FE-001

**File:** `components/settings/mcp/mcp-governance-tabs.tsx` (NEW)

```tsx
export function useMcpGovernanceTabs(): Array<{ value: 'policies'|'approvals'|'audit'; label: string; element: React.ReactNode }> {
  const isAdmin = useAppStore((s) => s.currentUser?.role === 'admin')
  return [
    { value: 'approvals', label: translate('auto.mcp.tab.approvals','Approvals'), element: <McpApprovalsTab /> },        // FE-009
    ...(isAdmin ? [
      { value: 'policies', label: translate('auto.mcp.tab.policies','Tool policies'), element: <McpPoliciesTab /> },
      { value: 'audit', label: translate('auto.mcp.tab.audit','MCP audit'), element: <McpAuditTab /> },                  // FE-010
    ] : []),
  ]
}
```
`McpSettingsSection` (FE-001) render `<McpKillSwitchBanner />` + map các tab này vào `Tabs` (một dòng additive).

## Trạng thái UI (tóm tắt)
Loading: `Skeleton`; Empty: policy trống / MCP tắt; Error: inline `role="alert"` + Retry; Forbidden: "Admin access required"; Disabled-by-flag: không mount (`isMcpSurfaceAvailable()` false hoặc `serverInfo.enabled=false`); Conflict/HardDeny: inline trong dialog; Submitting: nút disabled + `aria-busy`.

## A11y & i18n & style
- Mọi `Button` icon có `aria-label` + Tooltip; bảng dùng `Table` với `TableHead` đủ; lỗi nối `aria-describedby`; banner `role="status"` (không `alert` để không ngắt người đọc màn hình mỗi lần polling); dialog dùng `Dialog` (quyết định bắt buộc), focus quản lý như trên; Esc đóng không kèm trang trí.
- i18n: `translate('auto.mcp.policies.*' | 'auto.mcp.killswitch.*' | 'auto.mcp.reason.*' | 'auto.mcp.settings.*', 'English')` **chỉ trong thân hàm/component** (`no-top-level-translate.test.ts`); thêm key vào `i18n/locales/en.json` (4 locale còn lại rơi về fallback tiếng Anh theo cơ chế sẵn có). Chuỗi `STOP` là hằng không dịch.
- Style: chỉ token (`text-muted-foreground`, `border-border`, `bg-destructive/10`…), `font-mono` cho tên tool/mã reason; không thêm màu/shadow mới.

## Files cần sửa
| File | Action |
|---|---|
| `frontend/src/shared/mcp-governance-types.ts` | NEW |
| `frontend/src/preload/api-types.ts`, `renderer/src/web/web-preload-api.ts` | MODIFY — thêm 7 method vào `McpApi`/`createMcpApi` (FE-001) |
| `renderer/src/components/settings/mcp/{mcp-policy-form.ts, McpPoliciesTab.tsx, McpPolicyEditorDialog.tsx, McpPolicyExplainDialog.tsx, McpTenantSettingsForm.tsx, McpKillSwitchPanel.tsx, McpKillSwitchConfirmDialog.tsx, McpKillSwitchBanner.tsx, mcp-governance-tabs.tsx}` | NEW |
| `renderer/src/components/settings/mcp/McpSettingsSection.tsx` (FE-001) | MODIFY — render banner + tab từ `useMcpGovernanceTabs()` |
| `renderer/src/i18n/locales/en.json` | MODIFY — key `auto.mcp.*` |

## Verification
```bash
cd frontend && npx vitest run src/renderer/src/components/settings/mcp/mcp-policy-form.test.ts \
  src/renderer/src/components/settings/mcp/McpPoliciesTab.test.tsx \
  src/renderer/src/components/settings/mcp/McpKillSwitchPanel.test.tsx \
  src/renderer/src/components/settings/mcp/McpTenantSettingsForm.test.tsx \
  src/renderer/src/components/settings/mcp/McpKillSwitchBanner.test.tsx
cd frontend && npx vitest run src/renderer/src/i18n/no-top-level-translate.test.ts
cd frontend && npx tsc --noEmit -p tsconfig.json
```
Ca kiểm tra bắt buộc: `expandMatch`/`validatePolicyDraft` (hard-deny, exact-tool, không chiều); payload không chứa chiều rỗng; hard-deny + `allow` ⇒ Save disabled; mô phỏng server `MCP_POLICY_HARD_DENY` khi client không phát hiện; xung đột phiên bản (Reload / Overwrite chỉ khi bấm); Delete dùng `confirmVariant:'destructive'`; Kill switch: nút Activate disabled tới khi gõ đúng `STOP`, payload đúng shape CONTRACT, `data-variant="destructive"` trên nút Activate và **không** trên Cancel/Resume; `reason` <3 ký tự bị chặn; banner chỉ hiện khi `active`; settings chỉ gửi trường đổi, biên 1–90/30–900; gate admin (người thường không có tab).
Thủ công (cần BE-012/013 chạy): tạo policy `allow` cho `credentials_set` ⇒ bị chặn ở client **và** server trả `MCP_POLICY_HARD_DENY`; hai trình duyệt sửa cùng policy ⇒ `MCP_POLICY_VERSION_CONFLICT`; kích hoạt kill switch tenant ⇒ banner xuất hiện ở tab khác trong vài giây.

## Sửa TDD kèm theo
`specs/frontend/tdd/v5/05-ui-components.md` (đã lỗi thời so với mã) cần ghi nhận: tab admin MCP nằm trong Settings section `mcp` (không phải `AdminOrgConsole`); đường thật của shim là `renderer/src/web/web-preload-api.ts`; `v4/03-admin-spa` không thay đổi.

## Không làm ở solution này
UI duyệt/inbox (FE-MCP-SOL-009), nhật ký audit (FE-MCP-SOL-010), danh mục tool (FE-MCP-SOL-005), quản trị client/grant/session (FE-MCP-SOL-002/003) — chỉ **đọc** danh sách của chúng cho bộ chọn. Không bật "thu hồi PAT khi kill switch" (cần R-3), không liệt kê công tắc phạm vi hẹp (cần R-6), không chỉnh sửa/ưu tiên thứ tự policy.
