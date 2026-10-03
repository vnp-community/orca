# FE-MCP-SOL-007: Tab "Prompts" — prompt MCP built-in (chỉ đọc) và prompt tuỳ chỉnh (CRUD, admin)

> ✅ Implemented (unit/integration tests) — see Gaps. FE-MCP-SOL-001 (kiểu dùng chung, `window.api.mcp`, `mcp-slice`, section `mcp`, `McpSettingsPane`) được viết song song: solution này **chỉ dùng tên** do FE-001 chốt, không định nghĩa lại; chỗ phụ thuộc tên chưa chốt đánh dấu "(khớp FE-001 khi merge)".

## CR Reference

- **CR:** [CR-MCP-011](../../../../../../docs/crs/v5/mcp-resources-prompts/CR-MCP-011-prompts.md) mục B ("tenant bổ sung prompt riêng… chỉ admin tạo") · **Mức độ:** 🟡 P2
- **Phạm vi (chỉ FE):** UI soạn/quản lý prompt (CR ghi "UI soạn prompt (frontend)" là ngoài phạm vi BE). Resources (CR-MCP-010) không có UI — xem README feature.

## Backend dependency

| Kênh (CONTRACT §2.2) | Tham số | Kết quả | BE | Khi BE chưa có |
|---|---|---|---|---|
| `mcp.admin.prompt.list` | — | `McpPrompt[]` (gồm built-in `builtin:true`) | BE-MCP-SOL-011 §C | lỗi → trạng thái error + Retry; `MCP_DISABLED` → "MCP is turned off" |
| `mcp.admin.prompt.upsert` | `McpPrompt` (không builtin; không `id` = tạo; `version` = chống ghi đè) | `McpPrompt` | BE-011 | — |
| `mcp.admin.prompt.delete` | `promptId` | `{ok:true}` | BE-011 | — |

Gọi qua `window.api.mcp.admin.prompt.list/upsert/delete` (sub-namespace `admin.prompt` **thêm** vào `createMcpApi()` của FE-001; quy ước tên kênh → namespace như FE-003/004/011): `callRuntimeResult<T>('mcp.admin.prompt.<x>', arg0)`. Lỗi `Error("MCP_X: msg")` tách bằng `parseMcpError` (FE-003, `lib/mcp-error-code.ts`). Kiểu: `McpPrompt` (CONTRACT §1; `frontend/src/shared/mcp-types.ts` của FE-001): `{id,name,description,version,updatedAt,arguments:[{name,description,required}],template,builtin}`.

Mã lỗi **trong CONTRACT**: `MCP_NOT_ADMIN`, `MCP_DISABLED`, `MCP_NOT_FOUND`. **Đề xuất thêm vào CONTRACT §2.3 (BE-011 đã dùng):** `MCP_PROMPT_INVALID` (định dạng `MCP_PROMPT_INVALID: <field>: <lý do>`), `MCP_PROMPT_NAME_CONFLICT`, `MCP_PROMPT_VERSION_CONFLICT`, `MCP_PROMPT_BUILTIN_READONLY`. Trước khi CONTRACT chấp nhận: FE xử lý **mọi** mã `MCP_*` lạ bằng cách hiển thị thông điệp server trong banner của dialog (không map sang chuỗi i18n riêng); các nhánh riêng cho `MCP_PROMPT_*` chỉ kích hoạt khi mã khớp (không làm gì sai nếu server chưa trả).

## Impact analysis (gitnexus) — chưa chạy

| Symbol | Lệnh cần chạy trước khi sửa | Rủi ro dự kiến |
|---|---|---|
| `McpSettingsPane` (FE-001) | `gitnexus impact({target:"McpSettingsPane", direction:"upstream"})` | LOW (thêm 1 `TabsContent`) |
| `createMcpApi` (FE-001) | `gitnexus impact({target:"createMcpApi", direction:"upstream"})` | LOW (thêm sub-namespace) |
| `PreloadApi` (`preload/api-types.ts`) | `gitnexus impact({target:"PreloadApi", direction:"upstream"})` | MEDIUM (type rộng; chỉ thêm field) |

## Bối cảnh (đã xác nhận lại bằng mã thật, 2026-10-01)

- Mẫu form CRUD admin có sẵn: `components/settings/admin-org-console-policies-tab.tsx` (form tạo + danh sách, `Input`/`Textarea`, `toast` từ `sonner`, `window.api.admin.*`, gate admin) — nhưng nhãn hard-code tiếng Anh và gọi API trong component; solution này tách hook + hàm thuần và dùng `translate`.
- Primitive có: `ui/{dialog (Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter), sheet, table, tabs, badge, select, checkbox, input, textarea, button, tooltip, skeleton, sonner}`. **Không có** `switch/alert/alert-dialog`. Xác nhận xoá: `useConfirmationDialog()` (`components/confirmation-dialog.tsx:120`, cần `ConfirmationDialogProvider` ở gốc). **Mọi `Dialog` phải có `DialogDescription`** — commit `da67f0b33` mới sửa đúng lỗi thiếu `DialogDescription` ở dialog AI Provider.
- Quy tắc style (`guides/STYLEGUIDE.md`): Dialog giữa/Sheet cạnh; Cancel là ghost; `destructive` chỉ cho mất dữ liệu (xoá prompt đúng nghĩa này); màu theo token.
- i18n: `translate('auto.<path>', 'English')` trong hàm/component, cấm top-level (`i18n/no-top-level-translate.test.ts`). Test: vitest + happy-dom + `createRoot` (mẫu `CloseTerminalDialog.test.tsx`).
- Cú pháp template **do BE chốt** (BE-011 quyết định 1): prompt tuỳ chỉnh chỉ có `{{name}}` (thay thế biến thuần, tên `^[a-z][a-z0-9_]{0,31}$`), không điều kiện/vòng lặp; built-in là Go `text/template` (chỉ đọc). Giới hạn BE: tên `^[a-z][a-z0-9_]{2,47}$` không trùng built-in (`review_pull_request`, `triage_issue`, `plan_task`, `summarize_worktree`, `handoff_to_agent`), `description ≤ 500`, `template 1..8192` ký tự, `≤ 10` tham số, `≤ 50` prompt/tenant; mỗi `{{x}}` phải khai báo, mỗi tham số `required` phải được dùng; lint chống chèn chỉ dẫn vượt quyền (BE-011 §B.3).
- Locale: `McpPrompt` **không có** trường locale ⇒ prompt tuỳ chỉnh là một ngôn ngữ do admin viết; built-in có en/vi phía server nhưng `mcp.admin.prompt.list` chỉ trả bản nguồn `en` ở `template`.

## Giải pháp

### Bước 1 — Hàm thuần: biến & kiểm tra (mirror quy tắc BE để báo lỗi sớm; BE là nguồn quyết định)
**File:** `frontend/src/renderer/src/components/settings/mcp/mcp-prompt-template.ts` (NEW)
```ts
export const PROMPT_NAME_RE = /^[a-z][a-z0-9_]{2,47}$/
export const PROMPT_ARG_NAME_RE = /^[a-z][a-z0-9_]{0,31}$/
export const PROMPT_LIMITS = { template: 8192, description: 500, args: 10, customPerTenant: 50 } as const
export const BUILTIN_PROMPT_NAMES = ['review_pull_request','triage_issue','plan_task','summarize_worktree','handoff_to_agent'] as const

const VAR_RE = /\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}/g
export function extractTemplateVariables(template: string): string[]            // thứ tự xuất hiện, duy nhất
export function findUnsupportedSyntax(template: string): string[]               // vd {{.x}}, {{#if}}, {{ x | f }}, `{{`/`}}` lệch -> mô tả ngắn
export function renderTemplatePreview(template: string, samples: Record<string,string>): string  // thay {{x}} bằng mẫu hoặc ⟨x⟩; chỉ để xem, text thuần
```
**File:** `.../mcp/mcp-prompt-validation.ts` (NEW)
```ts
export type PromptFieldErrors = { name?: string; description?: string; template?: string; arguments?: Record<number, { name?: string }>; form?: string }
export function validatePromptDraft(d: PromptDraft, existingNames: ReadonlySet<string>): PromptFieldErrors
//  kiểm: tên (regex, trùng built-in/đã có), mô tả, độ dài template, số tham số, tên tham số hợp lệ & không trùng,
//  biến trong template chưa khai báo, tham số required không dùng, cú pháp không hỗ trợ
export function parseServerPromptError(code: string | null, message: string): PromptFieldErrors  // 'MCP_PROMPT_INVALID: template: …' -> { template: '…' }
```
Các hàm không import React/`translate`; trả mã lỗi + tham số, component dịch (tránh top-level `translate`).

### Bước 2 — Hook dữ liệu (không đưa vào slice)
**File:** `.../mcp/use-mcp-prompts.ts` (NEW)
```ts
type PromptsState = { status:'loading' } | { status:'ready'; prompts: McpPrompt[] } | { status:'error'; code: string|null; message: string }
export function useMcpPrompts(): {
  state: PromptsState; refresh(): void
  save(draft: McpPrompt): Promise<McpPrompt>          // window.api.mcp.admin.prompt.upsert -> cập nhật tại chỗ
  remove(id: string): Promise<void>                    // window.api.mcp.admin.prompt.delete -> loại tại chỗ
}
```
Quyết định: danh sách prompt không nhạy cảm nhưng không cần chia sẻ giữa component ⇒ state cục bộ, **không** vào `mcp-slice` (tránh đụng slice FE-001). Chống race bằng `requestSeq`. Sau `delete`/`save` gặp `MCP_NOT_FOUND` ⇒ `refresh()`. Refetch khi vào tab và khi cửa sổ lấy lại focus.

### Bước 3 — Tab & danh sách
**File:** `.../mcp/McpPromptsTab.tsx` (NEW), `McpPromptList.tsx` (NEW)
- Mô tả: "Prompts are reusable instructions users can pick in their MCP client. Prompts cannot grant permissions — tools are still controlled by policies and approvals." (`auto.mcp.prompts.description`).
- Thanh công cụ: ô lọc theo tên/mô tả; nút **New prompt** (`Button`, disabled + tooltip khi đã đủ 50 prompt tuỳ chỉnh hoặc `status!=='ready'`).
- Bảng (`ui/table`): **Name** (monospace) · **Type** (`Badge variant="secondary"` + icon `Lock` "Built-in" / `outline` "Custom") · **Description** (cắt 1 dòng + tooltip) · **Arguments** (số; tooltip liệt kê `name*` cho required) · **Version** (`v3`) · **Updated** (thời gian tương đối từ `updatedAt`, `title` = RFC 3339) · **Actions**. Built-in trước, custom sau, theo tên. Hành động built-in: `View`, `Duplicate`; custom: `Edit`, `Duplicate`, `Delete`.
- Xoá: `useConfirmationDialog()`: tiêu đề "Delete prompt {name}?", nội dung "Users will no longer see it in their MCP clients.", nút `destructive` "Delete". Thành công → `toast.success`; lỗi → `toast.error(message)` + `refresh()` nếu `MCP_NOT_FOUND`.

### Bước 4 — Dialog soạn thảo
**File:** `.../mcp/McpPromptEditorDialog.tsx` (NEW), `McpPromptArgumentsEditor.tsx` (NEW), `McpPromptTemplateField.tsx` (NEW)
```tsx
type Mode = { kind: 'create' } | { kind: 'edit'; prompt: McpPrompt } | { kind: 'view'; prompt: McpPrompt } | { kind: 'duplicate'; from: McpPrompt }
export function McpPromptEditorDialog(props: { mode: Mode; existingNames: ReadonlySet<string>; onSave(p: McpPrompt): Promise<void>; onClose(): void })
```
- `Dialog` giữa màn hình, có `DialogTitle` + **`DialogDescription`** ("Name and arguments are shown to users in their MCP client."). Chế độ `view` (built-in): mọi field `readOnly`/`disabled`, huy hiệu "Built-in — read only", dòng ghi chú: "Built-in prompts are maintained by Orca and change with releases. Use Duplicate to start a custom copy." và ghi chú template built-in dùng cú pháp nâng cao.
- Field: **Name** (`Input`, monospace; ở `edit` đổi tên cho phép? — BE upsert theo `id`, đổi tên được; hiển thị cảnh báo "Clients that saved this prompt by name will no longer find it"), **Description** (`Input`, đếm ký tự), **Arguments** (editor dòng: tên · mô tả · `Checkbox` "Required" · nút lên/xuống · xoá; nút "Add argument" disabled khi đủ 10; không dùng textarea JSON), **Template** (`Textarea` monospace `rows=14`, bộ đếm `n / 8192`).
- `McpPromptTemplateField`: dưới textarea hiển thị **chip biến** từ danh sách tham số — bấm chip chèn `{{name}}` tại con trỏ (đọc `selectionStart/End` từ ref, không giữ state chứa toàn bộ); hàng "Detected variables: `{{a}}`, `{{b}}`"; gợi ý `Use {{name}} to insert an argument. Conditions and loops are not supported.`; biến chưa khai báo hiển thị dưới dạng danh sách lỗi (không tô màu trong textarea).
- **Preview**: khối `<pre className="whitespace-pre-wrap break-words">` từ `renderTemplatePreview` với giá trị mẫu `⟨name⟩`; nhãn "Preview (placeholders shown as ⟨name⟩)". Chỉ text thuần — cấm `dangerouslySetInnerHTML`/markdown.
- **Version**: ở `edit`/`view` hiển thị `Version 3 · updated <time>`; khi lưu gửi `version` hiện có. `create`/`duplicate`: không có `id`/`version`; duplicate đặt `name = <name>_copy` (cắt cho vừa regex), `description`, `arguments`, `template` sao chép; nếu `findUnsupportedSyntax(template)` khác rỗng (built-in dùng `{{.x}}`/điều kiện) thì hiển thị banner "This text uses advanced syntax that custom prompts don't support. Edit it before saving." và chặn Save tới khi hết.
- **Lưu**: nút `Save` chỉ bật khi `validatePromptDraft` sạch; gọi `save()` → đóng dialog + `toast.success`. Lỗi server: `parseServerPromptError` gắn lỗi vào field (`MCP_PROMPT_INVALID: template: …`), `MCP_PROMPT_NAME_CONFLICT` → lỗi ở Name, `MCP_PROMPT_VERSION_CONFLICT` → banner "This prompt was changed by someone else." + nút `Reload` (refetch và nạp lại, giữ bản nháp của người dùng trong khối "Your changes" để so sánh/sao chép), `MCP_PROMPT_BUILTIN_READONLY` → banner, `MCP_NOT_ADMIN` → banner forbidden và khoá form, mã lạ → banner thông điệp server. Dialog **không đóng** khi lỗi; dữ liệu nhập được giữ.
- Đóng khi có thay đổi chưa lưu: `useConfirmationDialog()` "Discard changes?" (Cancel ghost).

### Bước 5 — Đăng ký tab (FE-001, chỉ thêm)
`McpSettingsPane.tsx` (FE-001; MODIFY): `TabsTrigger value="prompts"` + `TabsContent` lazy `import('./McpPromptsTab')`, chỉ khi `currentUser?.role==='admin'` và `mcpServerInfo.enabled`. Không sửa `Settings.tsx`/`useSettingsNavigationMetadata.ts`.

### Bước 6 — i18n
`auto.mcp.prompts.*` + fallback tiếng Anh (`translate` trong component/hook); thêm `en.json`. Tên prompt/biến là định danh — không dịch. Thông điệp lỗi từ server hiển thị nguyên văn (tiếng Anh).

## Trạng thái UI

| Trạng thái | Hiển thị |
|---|---|
| loading | 6 hàng `Skeleton`, nút New disabled |
| ready, chỉ có built-in | bảng 5 built-in + dòng gợi ý "No custom prompts yet. Create one to share a workflow with your team." |
| ready, lọc không khớp | "No prompts match." + Clear |
| error | khung lỗi + Retry; `MCP_DISABLED` ⇒ "MCP is turned off for this organization." (không Retry) |
| forbidden | non-admin không thấy tab; `MCP_NOT_ADMIN` giữa chừng ⇒ banner + khoá form + ẩn dữ liệu cũ |
| saving | `Save` disabled + spinner; các field `disabled` |
| validation error | lỗi theo field (`aria-invalid`, `aria-describedby`) + tóm tắt trên cùng dialog |
| conflict | banner xung đột phiên bản + Reload |
| disabled-by-flag | `enabled=false` ⇒ tab không mount |
| limit | 50 custom ⇒ New disabled + tooltip "Limit reached (50)" |

## A11y & i18n & style

Dialog có `DialogTitle`+`DialogDescription`, focus vào Name khi mở, trả focus về nút gọi khi đóng, `Esc` hỏi huỷ nếu có thay đổi; lỗi field có `role="alert"` ngắn gọn; nút di chuyển tham số có `aria-label` ("Move argument up"); bảng dùng `<table>`; huy hiệu Built-in/Custom có icon+chữ. Token màu, không hard-code. Mọi nội dung template/mô tả là text node.

## Files cần sửa

| File | Loại |
|---|---|
| `renderer/src/components/settings/mcp/McpPromptsTab.tsx`, `McpPromptList.tsx`, `McpPromptEditorDialog.tsx`, `McpPromptArgumentsEditor.tsx`, `McpPromptTemplateField.tsx` | NEW |
| `.../mcp/use-mcp-prompts.ts`, `mcp-prompt-template.ts`, `mcp-prompt-validation.ts` | NEW |
| `.../mcp/mcp-prompt-template.test.ts`, `mcp-prompt-validation.test.ts`, `McpPromptEditorDialog.test.tsx`, `McpPromptList.test.tsx` | NEW |
| `.../mcp/McpSettingsPane.tsx` (FE-001) | MODIFY (thêm tab) |
| `renderer/src/web/web-preload-api.ts`, `preload/api-types.ts` | MODIFY (thêm `mcp.admin.prompt.*`) |
| `renderer/src/i18n/locales/en.json` | MODIFY |

## Verification

```bash
cd frontend
npx vitest run --config config/vitest.config.ts src/renderer/src/components/settings/mcp/mcp-prompt-template.test.ts
npx vitest run --config config/vitest.config.ts src/renderer/src/components/settings/mcp/mcp-prompt-validation.test.ts
npx vitest run --config config/vitest.config.ts src/renderer/src/components/settings/mcp/McpPromptEditorDialog.test.tsx
npx vitest run --config config/vitest.config.ts src/renderer/src/i18n/no-top-level-translate.test.ts
npx tsc --noEmit -p tsconfig.json
```
Test chính: `extractTemplateVariables`/`findUnsupportedSyntax` (bảng: `{{a}}`, `{{ a }}`, `{{.a}}`, `{{#if a}}`, `{{a`, trùng biến); `validatePromptDraft` (mọi quy tắc ở "Bối cảnh"); `parseServerPromptError`; dialog: built-in ⇒ chỉ đọc + không có nút Save; duplicate built-in có cú pháp nâng cao ⇒ Save bị chặn; lưu gửi đúng `version`; lỗi `MCP_PROMPT_INVALID: template: …` gắn đúng field và không đóng dialog; xung đột phiên bản hiện Reload; template chứa `<img onerror>` hiển thị thành text ở preview; xoá cần xác nhận. Thủ công: Tab/Shift+Tab qua toàn dialog, chèn biến bằng chip.

## Sửa TDD kèm theo
`specs/frontend/tdd/v5` (05-admin): tab MCP prompts nằm trong `McpSettingsPane`, không phải `AdminOrgConsole`.

## Không làm ở solution này
Prompt cá nhân của user thường; chọn locale cho prompt tuỳ chỉnh (CONTRACT không có `locale`); chỉnh sửa built-in; chạy thử prompt với dữ liệu thật (cần `prompts/get` của giao thức MCP, không phải kênh UI); hiển thị lịch sử phiên bản.
