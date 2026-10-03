# FE-MCP-SOL-011: Tab "External servers" — registry MCP server ngoài (Settings > MCP)

> ✅ Implemented (unit/integration tests) — see Gaps. FE-MCP-SOL-001 (kiểu dùng chung, `window.api.mcp`, `mcp-slice`, section `mcp`) đang được viết song song: solution này **chỉ dùng tên** do FE-001 chốt, không định nghĩa lại; chỗ nào phụ thuộc tên chưa chốt được đánh dấu "(khớp FE-001 khi merge)".

## CR Reference

- **CR:** [CR-MCP-014](../../../../../../docs/crs/v5/mcp-client-registry/CR-MCP-014-external-mcp-server-registry.md) · **Mức độ:** 🟡 P2
- **Phạm vi (chỉ FE):** UI quản lý registry server ngoài. CR-014 ghi "UI quản lý registry (frontend)" là ngoài phạm vi phía BE — đây là phần đó. Không làm phía cấp cấu hình cho agent (BE-014 mục F/G).
- **Không đụng** `components/settings/McpConfigSection.tsx` (khối "MCP Configs" theo repo, gắn ở `RepositoryPane.tsx:393`, đọc/ghi file `.mcp.json`/Cursor/Claude **cục bộ trên máy dev**). Khác bản chất: registry này là **cấu hình do tổ chức kiểm soát, lưu ở server**, được duyệt bởi admin và cấp cho agent do Orca chạy. UI copy phải nói rõ (xem "A11y, i18n & copy").

## Backend dependency

| Kênh (CONTRACT §2.2) | Dùng cho | BE | Khi BE chưa có |
|---|---|---|---|
| `mcp.externalServer.list{scope?}` | bảng danh sách | BE-MCP-SOL-014 | `MCP_DISABLED`/lỗi mạng ⇒ trạng thái error + Retry; không crash |
| `.upsert` | thêm/sửa | BE-014 | — |
| `.setSecret{serverId,kind,name,value}` | đặt secret env/header; `value` là **plaintext** gửi một lần qua WS/TLS (D1 đã chốt — không dùng phong bì client) | BE-014 | BE chưa có ⇒ nút Set hiển thị lỗi `MCP_DISABLED`/mạng, không crash |
| `.probe{serverId}` | liệt kê tool (admin) | BE-014 | — |
| `.review{serverId,{decision,toolsDigest}}` | duyệt/từ chối (admin) | BE-014 | — |
| `.delete{serverId}` | xoá | BE-014 | — |
Gọi qua `window.api.mcp.externalServer.*` — sub-namespace **thêm** vào `createMcpApi()`/`PreloadApi` của FE-001 (chỉ thêm kênh/namespace, đúng bất biến "không sửa `web-preload-api.ts` ngoài thêm kênh"). Mỗi hàm là `callRuntimeResult<T>('mcp.externalServer.<x>', params)` (mẫu `createAdminApi`, `web/web-preload-api.ts:2668`). Lỗi = `Error("MCP_X: msg")`, tách bằng hàm của FE-001 (`^([A-Z0-9_]+): `).
Mã lỗi dùng: `MCP_DISABLED`, `MCP_NOT_ADMIN`, `MCP_NOT_FOUND`, `MCP_SERVER_SSRF_BLOCKED`, `MCP_SERVER_NOT_APPROVED`. **Đề xuất thêm vào CONTRACT** (BE-014 đã dùng): `MCP_SERVER_INVALID`, `MCP_SERVER_STDIO_NOT_ALLOWED`, `MCP_SERVER_DIGEST_MISMATCH`, `MCP_SERVER_NAME_CONFLICT`; trước khi được chấp nhận, mã lạ rơi về thông điệp server trong banner (không map thành chuỗi i18n riêng).
**Trạng thái đề xuất đổi CONTRACT:** CR-1 (`iv` cho `setSecret`) **bị thay thế bởi quyết định D1** — CONTRACT nay chốt `setSecret{serverId,kind,name,value}` với `value` plaintext (không `encryptedBlob`/`iv`); CR-2 (`probe` thêm `approvedTools?`/`transport?`), CR-4 (chủ sở hữu `probe` server `scope:'user'` của mình) và CR-5 (`updatedAt?`) đã nằm trong CONTRACT §1/§2.2; CR-3 (mã `MCP_SERVER_*`) đã nằm trong §2.3. Phương án dự phòng cho từng chỗ phụ thuộc (nếu BE chưa theo kịp) ghi bên dưới.

## Impact analysis (gitnexus) — chưa chạy

| Symbol | Lệnh cần chạy trước khi sửa | Rủi ro dự kiến |
|---|---|---|
| `createMcpApi` (FE-001) | `gitnexus impact({target:"createMcpApi", direction:"upstream"})` | LOW (symbol mới của FE-001; chỉ thêm sub-namespace) |
| `useSettingsNavigationMetadata` / `Settings.tsx` | không sửa ở solution này (FE-001 đã đăng ký section `mcp`) | — |
| `mcp-slice.ts` (FE-001) | `gitnexus impact({target:"createMcpSlice"})` nếu quyết định thêm state vào slice | LOW |
| `encryptCredential` (`lib/credential-crypto.ts`) | **không dùng** cho MCP (D1) — không import, không sửa | — |

## Bối cảnh (đã xác nhận lại bằng mã thật, 2026-10-01)

- **Cơ chế secret của AI Provider** = `components/ai-provider/CredentialInput.tsx` + `lib/credential-crypto.ts` (`encryptCredential(plaintext, sessionToken) → {encryptedBlob, iv}`, AES-GCM, PBKDF2) + `ProviderForm.tsx` gọi `aiProvider.writeCredential{accountId, encryptedBlob, iv}`; plaintext được xoá khỏi state ngay sau khi mã hoá.
- **Không dùng `CredentialInput` / `encryptCredential` cho MCP (D1 đã chốt):** secret MCP gửi plaintext một lần qua WS/TLS đã xác thực; server mã hoá khi lưu (Vault Transit qua credential-broker). Lý do không dùng phong bì client: xem cảnh báo bên dưới. `CredentialInput` cũng không tái dùng được nguyên xi: (1) prop `provider: AIProviderType` quyết định nhãn, `ollama` ⇒ trả `null`; (2) ngưỡng `value.length >= 10` mã hoá *theo từng phím* rồi xoá ô nhập ⇒ secret ngắn (header/env < 10 ký tự) không bao giờ được ghi nhận và người dùng mất ký tự đang gõ; (3) gọi `useAppStore` **sau** `return null` có điều kiện (vi phạm thứ tự hook). ⇒ Component mới `McpExternalServerSecretField` (uncontrolled input, commit bằng nút, gửi plaintext trực tiếp qua `setSecret`); khuyến nghị ticket riêng tổng quát hoá `CredentialInput` — không làm ở đây.
- **Cảnh báo bảo mật phải ghi rõ:** `auth` slice **không có** `sessionToken` (grep `store/slices/auth.ts` không khớp) ⇒ `CredentialInput` luôn rơi về hằng số `'fallback-dev-token'` ⇒ khoá AES có thể suy ra công khai; envelope chỉ là che mắt, và broker coi envelope là bytes mờ (không giải mã được) nên không thể cấp lại plaintext cho agent. Vì vậy D1: **không** dùng phong bì client cho secret MCP; bảo vệ là TLS/WSS + mã hoá at-rest phía server. UI **không** được tuyên bố "mã hoá đầu cuối"; copy cố định (`translate`): "Sent over TLS and encrypted at rest by the server." (không dùng cụm "end-to-end"). *Follow-up riêng, ngoài phạm vi:* `'fallback-dev-token'` trong `CredentialInput` của AI Provider là điểm yếu có sẵn, không bị đụng ở đây.
- Admin gate: `useAppStore((s) => s.currentUser?.role === 'admin')` (mẫu `Settings.tsx:283`, `useSettingsNavigationMetadata.ts:609`). Danh sách team cho admin: `window.api.admin.listTeams()` (`AdminTeam{id,name}`, `web-preload-api.ts:2696`).
- Primitive có sẵn: `ui/{dialog,sheet,table,tabs,badge,select,checkbox,input,textarea,skeleton,tooltip,sonner}`; **không có** `switch/alert/alert-dialog` ⇒ banner cảnh báo viết bằng `div` token + icon `lucide-react`; xác nhận xoá dùng `useConfirmationDialog()` (`components/confirmation-dialog.tsx:120`). Badge variant thật: `default|secondary|dot|destructive|outline|ghost|link` (không có `warning`) ⇒ trạng thái dùng `secondary`/`outline`/`destructive`/`dot` kèm icon+chữ, không dựa vào màu (STYLEGUIDE `guides/STYLEGUIDE.md`: màu chỉ cho trạng thái; Cancel là ghost; `destructive` chỉ cho mất dữ liệu).
- `Sources` của profile **không** có trong response `GetResolvedProfile` (chỉ `resolved_settings_json`); `types/profile-types.ts` có `McpServerConfig{name,command,args?,env?}` và `ProfileSourceBadge` (company/dept/user/concat) nhưng CONTRACT `McpExternalServer` **không** có trường nào cho biết lớp profile ⇒ xem Bước 7.

## Giải pháp

### Bước 1 — Kiểu bổ sung (không đụng `mcp-types.ts` của FE-001)
**File:** `frontend/src/shared/mcp-external-server-types.ts` (NEW)
```ts
import type { McpExternalServer } from './mcp-types'   // FE-001, đúng CONTRACT §1

export type McpExternalServerUpsertInput = Omit<
  McpExternalServer,
  'id' | 'status' | 'toolsDigest' | 'toolsChanged' | 'health' | 'createdBy' | 'reviewedBy' | 'envRefs' | 'headerRefs'
> & { id?: string; envRefs: Array<{ name: string }>; headerRefs: Array<{ name: string }> } // CONTRACT: upsert không mang hasSecret
export type McpExternalServerSetSecretInput = {
  serverId: string; kind: 'env' | 'header'; name: string
  value: string      // plaintext, gửi một lần qua WS/TLS (D1); KHÔNG encryptedBlob/iv; không bao giờ lưu vào state/store
}
export type McpExternalServerProbeResult = {
  tools: Array<{ name: string; description: string }>; digest: string
  approvedTools?: Array<{ name: string; description: string }>  // đã có trong CONTRACT §2.2 (tuỳ chọn)
}
export type McpExternalServerReviewInput = { decision: 'approve' | 'reject'; toolsDigest: string }
```

### Bước 2 — Runtime client
**File:** `frontend/src/renderer/src/web/web-preload-api.ts` (MODIFY — chỉ trong `createMcpApi()` của FE-001) và type `PreloadApi.mcp.externalServer` (file type của FE-001):
```ts
externalServer: {
  list: (p?: { scope?: McpExternalServer['scope'] }) => callRuntimeResult<McpExternalServer[]>('mcp.externalServer.list', p ?? {}),
  upsert: (s: McpExternalServerUpsertInput) => callRuntimeResult<McpExternalServer>('mcp.externalServer.upsert', s),
  setSecret: (p: McpExternalServerSetSecretInput) => callRuntimeResult<{ hasSecret: true }>('mcp.externalServer.setSecret', p),
  probe: (serverId: string) => callRuntimeResult<McpExternalServerProbeResult>('mcp.externalServer.probe', { serverId }),
  review: (serverId: string, r: McpExternalServerReviewInput) => callRuntimeResult<McpExternalServer>('mcp.externalServer.review', { serverId, ...r }),
  delete: (serverId: string) => callRuntimeResult<{ ok: true }>('mcp.externalServer.delete', { serverId })
}
```
(Theo quy tắc C10 của CONTRACT: mỗi kênh nhận đúng **một** object ở `args[0]`; các hàm bọc nhận `serverId` rời cho tiện gọi nhưng gói thành `{serverId, …}` khi gửi.)

### Bước 3 — Hook dữ liệu (không đưa secret vào slice)
**File:** `frontend/src/renderer/src/components/settings/mcp/use-external-servers.ts` (NEW)
State cục bộ `{status:'loading'|'ready'|'error', servers, errorCode}`; `refresh()`; mutate = gọi API rồi `refresh()`. Danh sách là dữ liệu không nhạy cảm nên có thể đưa vào `mcp-slice` (`externalServers`) nếu FE-001 cần cache — **mặc định không** để tránh đụng slice của FE-001. Không bao giờ có trường `value/plaintext` ở bất kỳ state/store nào. Sau `delete`/`upsert` bắt `MCP_NOT_FOUND` ⇒ `refresh()` (đã bị xoá/đổi quyền).

### Bước 4 — Tab và danh sách
**File:** `components/settings/mcp/McpExternalServersTab.tsx` (NEW) — mount làm tab `external-servers` trong pane MCP của FE-001 (deep link `?section=mcp&tab=external-servers`); lazy `import()` cùng cơ chế lazy-load của FE-001 (chỉ mount khi `enabled`).
**File:** `components/settings/mcp/McpExternalServerList.tsx` (NEW) — `Table`: Name · Scope (`Tenant`/`Team`/`Just me`) · Transport · Target (`url` hoặc `command args…`, `truncate` + tooltip) · Status · Health · Actions.
- Status badge: `pending_review` ⇒ `outline` + icon `Clock` "Pending review"; `approved` ⇒ `secondary` + `ShieldCheck` "Approved"; `disabled` ⇒ `dot` + `Ban` "Disabled".
- `toolsChanged` ⇒ hàng có chip `destructive` "Tools changed — re-review required" + tooltip "This server's tool descriptions differ from the version an admin approved. It is not given to agents until re-approved." (khớp BE: bị loại khi đổi).
- Health (`health?`): `ok` ⇒ "Healthy · checked <relative time>"; `!ok` ⇒ "Unreachable" + `error` (render text thuần); vắng ⇒ "Not checked yet" (không hiển thị cho `stdio`/`pending_review`).
- Hành động theo vai trò: **admin**: Probe, Review (khi `pending_review` hoặc `toolsChanged`), Edit, Disable (= Review→reject), Delete. **User thường**: chỉ Edit/Delete cho server `scope:'user'` của mình; Probe/Review **ẩn**; hàng `pending_review` hiển thị ghi chú "Waiting for an admin to review".
- Lọc: `Select` scope (chỉ admin), ô tìm theo name. Phân trang không cần (CONTRACT `list` trả mảng).

### Bước 5 — Dialog thêm/sửa
**File:** `components/settings/mcp/McpExternalServerDialog.tsx` (NEW) — `Dialog` giữa màn hình; form: Name (`^[a-z0-9][a-z0-9_-]{0,62}$`, khớp BE) · Scope (`Select`; user thường bị khoá ở "Just me"; admin: Tenant / Team (+`Select` team từ `admin.listTeams()`, nhãn `AdminTeam.name`, giá trị `id` → `scopeId`) / Just me; **không gửi** `scopeId` cho scope user/tenant — server tự gán) · Transport (`ToggleGroup` HTTP | stdio).
- **HTTP:** URL (`https://` bắt buộc theo ràng buộc BE; kiểm nhẹ phía client: scheme, không userinfo, không IP-literal ⇒ hiển thị lỗi inline *gợi ý*; quyết định cuối là `MCP_SERVER_SSRF_BLOCKED` từ BE); **Header names** editor (danh sách dòng: tên + trạng thái secret).
- **stdio:** khối cảnh báo mạnh (`div` viền `border-destructive`, icon `TriangleAlert`): "This runs a program on the machine where the agent runs, with the permissions of the agent. Only an admin can approve it, and it only runs if your organization has enabled stdio servers." + checkbox bắt buộc "I understand this will execute code" (disable Save tới khi tick). Fields: Command (một tên lệnh, không đường dẫn/khoảng trắng), **Args editor** (danh sách dòng, thêm/xoá/sắp xếp bằng nút; không một ô textarea để tránh lỗi quote; cảnh báo inline nếu arg giống secret theo `SENSITIVE_ENV_VALUE_PATTERN`-style: "Don't put secrets in arguments — use an environment variable"), **Env names** editor.
- **Env/header editor** (`SecretRefRows`): mỗi dòng = tên (đã lưu ⇒ chỉ đọc, đổi tên = xoá+thêm) + chip `hasSecret ? "Secret set" (icon Lock) : "No value yet"` + nút **Set / Replace** mở ô nhập secret (Bước 6) + nút xoá dòng. **Không bao giờ hiển thị hay tiền điền giá trị** (CONTRACT C6; `hasSecret` là boolean).
- Lưu (create): `upsert` → (nếu có secret chờ) `setSecret` tuần tự; thất bại từng secret hiển thị trạng thái từng dòng + "Retry" chỉ cho dòng lỗi (server đã tồn tại ⇒ không upsert lại). Sửa spec (url/command/args/tên ref) trên server `approved` ⇒ hiện thông báo trước khi lưu: "Changing this will send the server back to review." (BE đặt lại `pending_review`).
- Lỗi theo mã: `MCP_SERVER_SSRF_BLOCKED` ⇒ lỗi inline ở URL: "This address isn't allowed (private, local or cloud-metadata addresses are blocked)"; `MCP_NOT_ADMIN` ⇒ banner forbidden; `MCP_SERVER_INVALID/NAME_CONFLICT/STDIO_NOT_ALLOWED` (khi được chấp nhận) ⇒ lỗi theo field/banner; còn lại ⇒ thông điệp server.

### Bước 6 — Nhập secret (plaintext qua TLS, không phong bì client)
**File:** `components/settings/mcp/McpExternalServerSecretField.tsx` (NEW)
```tsx
// Why: secret MCP gửi plaintext một lần qua WS/TLS (D1); server mã hoá at-rest. Không dùng envelope client
// vì broker không giải mã được nó. Commit bằng nút, và KHÔNG giữ plaintext trong React state.
export function McpExternalServerSecretField(props: {
  label: string; onCommit: (value: string) => void; onCancel: () => void
}) {
  const inputRef = useRef<HTMLInputElement>(null)           // uncontrolled: không có state chứa plaintext
  const commit = () => {
    const v = inputRef.current?.value ?? ''
    if (!v) return
    try { props.onCommit(v) }                               // dialog giữ giá trị trong useRef tới khi Save
    finally { if (inputRef.current) inputRef.current.value = '' }   // xoá plaintext khỏi DOM ngay
  }
  return (/* <Input type="password" autoComplete="new-password" spellCheck={false} data-testid="mcp-secret-input"/> + Set / Cancel
             + dòng chú thích: translate('auto.mcp.external.secretNote', 'Sent over TLS and encrypted at rest by the server.') */)
}
```
`onCommit` lưu giá trị vào `useRef` của dialog (không `useState`); gửi bằng `setSecret{serverId, kind, name, value}` khi Save; **xoá ref khi đóng/unmount dialog** và ngay sau mỗi `setSecret` (thành công hay lỗi); `hasSecret` được lấy lại từ `list`, không tự suy; phản hồi `{hasSecret:true}` **không** chứa giá trị. Không `console.log`/telemetry giá trị; không đưa vào `Tracers`/`localStorage`/slice; không đưa vào thông điệp lỗi hiển thị. `autoComplete="new-password"` như `CredentialInput`. Copy không được nói "end-to-end encrypted".

### Bước 7 — Probe (admin) và hiển thị văn bản không tin cậy
**File:** `components/settings/mcp/McpExternalServerReviewDialog.tsx` (NEW) — mở từ "Probe" hoặc "Review". Luồng: mở ⇒ tự gọi `probe(serverId)` (loading skeleton) ⇒ hiển thị:
- Header: tên server, target, `digest` (monospace, rút gọn 12 ký tự + nút copy), **transport**.
- Danh sách tool: tên + mô tả qua `UntrustedToolText` — **chỉ** `<pre className="whitespace-pre-wrap break-words">{text}</pre>`/text node; cấm `dangerouslySetInnerHTML`, cấm markdown renderer, cấm tự biến URL thành link; cắt hiển thị 500 ký tự + "Show more"; ký tự điều khiển/bidi (`‪-‮`, `⁦-⁩`, ZWSP) được thay bằng glyph hiển thị `⟦U+202E⟧` để chống đánh lừa thị giác. Khung có nhãn cố định: "Text below comes from the external server and is untrusted. Read it for hidden instructions." Test chặn: `no-dangerous-html` (grep trong file) + render `<img onerror>` phải ra text.
- **Diff kể từ lần duyệt** (`mcp-tool-diff.ts`, hàm thuần): nếu `approvedTools` có: nhóm *Added* / *Removed* / *Description changed* (hiển thị trước↔sau cạnh nhau, đánh dấu cụm khác bằng diff từ-ký-tự nhưng vẫn là text thuần). **Dự phòng khi BE chưa trả `approvedTools`:** chỉ hiển thị toàn bộ tool hiện tại + banner "This server was approved before; compare with the previous version manually" nếu `server.toolsChanged`, và ẩn diff.
- Stdio: probe trả `tools: []` (BE không chạy stdio) ⇒ hiển thị thay thế: "Orca doesn't run stdio servers to list their tools. Review the command and arguments carefully." + hiển thị command/args/tên env (text thuần) — đây là thứ admin duyệt.
- Hành động (chỉ admin): **Approve** (gửi `review(id,{decision:'approve', toolsDigest: probe.digest})`) và **Reject** (`decision:'reject'`; ghi chú: "Rejecting disables the server"). Approve đòi tick "I reviewed the tools/command". `MCP_SERVER_DIGEST_MISMATCH` (nếu có) hoặc digest đổi ⇒ thông báo "The server changed while you were reviewing — probing again" và tự probe lại. `MCP_SERVER_SSRF_BLOCKED` ⇒ trạng thái lỗi trong dialog, nút Approve disable.
- Quan hệ profile `mcp.servers` / `ProfileSourceBadge`: **chỉ hiển thị khi dữ liệu có** — hiện **không có** (xem Bối cảnh). Do đó v1: hàng không có badge lớp profile; chỉ hiển thị `Scope`. Mô tả trong tab (copy): "Servers a profile lists under `mcp.servers` are matched by name against this registry. Only approved servers are given to agents." Ghi nhận *tương lai*: khi BE đưa `Sources` (hoặc `McpExternalServer.profileSource?`) vào API, tái dùng `ProfileSourceBadge` (`components/profile/ProfileSourceBadge.tsx`, giá trị `company|dept|user|concat`) trong cột Scope. Cũng ghi nhận cho FE-profile: `McpServerConfig` cho phép `command/env` inline — nên có cảnh báo ở editor profile (ngoài phạm vi).

### Bước 8 — Trạng thái màn hình
| Trạng thái | Hiển thị |
|---|---|
| loading | `Skeleton` 3 hàng; nút Add disable |
| empty | "No external MCP servers yet" + giải thích 2 câu + nút "Add server" (user: "Add my server") |
| error | khung lỗi `border-destructive` + mã/thông điệp + Retry (`MCP_DISABLED` ⇒ ẩn cả tab; FE-001 đã ẩn UI khi `enabled=false`) |
| forbidden | `MCP_NOT_ADMIN` khi thao tác admin ⇒ toast + ẩn nút admin; user thường không thấy tab Probe/Review/scope tenant |
| disabled-by-flag | `mcp.server.info.enabled=false` ⇒ tab không mount (FE-001) |
| kill switch | `killSwitch.active` (từ slice FE-001) ⇒ banner "MCP is paused by an admin"; nút Add/Edit/Approve disable (BE trả `MCP_KILL_SWITCH_ACTIVE`) |
| row busy | spinner trong nút đang chạy, các nút khác của hàng disable (tránh double-submit) |
Bất biến: không dùng `window.confirm`; xoá bằng `useConfirmationDialog({confirmVariant:'destructive'})` với nội dung "Agents will stop receiving this server on their next start. Stored secrets are deleted."

## A11y, i18n & copy, style
- Mọi chuỗi qua `translate('auto.mcp.external.<id>', 'English')` **bên trong component/hàm** (cấm top-level — `no-top-level-translate.test.ts`), thêm vào `i18n/locales/en.json`; locale khác rơi về tiếng Anh. Nhãn trạng thái luôn có chữ + icon (không chỉ màu); dialog có `DialogDescription` (đã từng là bug ở AI provider dialog — commit `da67f0b33`); focus vào ô đầu tiên khi mở, `aria-live="polite"` cho kết quả probe/lỗi; editor dòng có `aria-label` "Remove argument 2".
- **Phân biệt với MCP Configs theo repo (copy cố định, hiển thị dưới tiêu đề tab):** "External servers are managed by your organization and given to agents that Orca starts. They are separate from the MCP config files in a repository's settings (`.mcp.json`, Cursor, Claude), which only describe files on your machine."
- Nút Cancel = `ghost`; `destructive` chỉ cho Delete (mất dữ liệu). Reject không mất dữ liệu (chỉ chuyển `disabled`) nên dùng `outline`. Token màu từ `assets/main.css`, không hex.
- Mọi thao tác chạy cả Electron (runtime remote) lẫn web; nút "Copy digest" dùng `navigator.clipboard` có try/catch. Không có đường dẫn file hay phím tắt riêng nền tảng nên không cần nhánh Mac/Windows; ví dụ lệnh stdio trong placeholder dùng `npx` (không `npx.cmd`).

## Files cần sửa
| File | Action |
|---|---|
| `frontend/src/shared/mcp-external-server-types.ts` | NEW |
| `frontend/src/renderer/src/web/web-preload-api.ts` | MODIFY — thêm `externalServer` vào `createMcpApi()` (FE-001) |
| `frontend/src/renderer/src/components/settings/mcp/use-external-servers.ts` | NEW |
| `.../mcp/McpExternalServersTab.tsx`, `McpExternalServerList.tsx`, `McpExternalServerDialog.tsx`, `McpExternalServerSecretField.tsx`, `McpExternalServerReviewDialog.tsx`, `UntrustedToolText.tsx` | NEW |
| `.../mcp/mcp-tool-diff.ts`, `mcp-external-server-validation.ts` | NEW (hàm thuần) |
| `frontend/src/renderer/src/i18n/locales/en.json` | MODIFY — khoá `auto.mcp.external.*` |
| Pane MCP của FE-001 | MODIFY — đăng ký tab `external-servers` (một dòng) |
| `McpConfigSection.tsx`, `RepositoryPane.tsx`, `App.tsx`, `main.tsx` | **KHÔNG sửa** |

## Verification
```bash
cd frontend && npx vitest run src/renderer/src/components/settings/mcp
cd frontend && npx tsc --noEmit -p tsconfig.json
cd frontend && npx vitest run src/renderer/src/i18n/no-top-level-translate.test.ts
```
Test cần có (cạnh component, theo mẫu `ai-provider/__tests__/`):
- `McpExternalServerSecretField.test.tsx`: sau Set, `input.value === ''`; `onCommit` nhận đúng giá trị một lần; secret 3 ký tự vẫn được ghi nhận (khác `CredentialInput`); spy `localStorage`/store/`console` không chứa plaintext; body `setSecret` đúng `{serverId, kind, name, value}` (không `encryptedBlob`/`iv`); ref bị xoá sau Save/đóng dialog; copy chứa "encrypted at rest" và **không** chứa "end-to-end".
- `McpExternalServerDialog.test.tsx`: user thường bị khoá scope "Just me"; stdio cần tick xác nhận; không có `value` hiển thị cho dòng `hasSecret`; upsert body **không** có `hasSecret/status/toolsDigest`; thất bại một secret ⇒ Retry chỉ dòng đó.
- `McpExternalServerReviewDialog.test.tsx`: mô tả `<img src=x onerror=alert(1)>`/`[click](javascript:…)`/RLO char hiển thị là text; approve gửi đúng `toolsDigest` của lần probe; stdio hiển thị khối command thay tool list; `MCP_SERVER_SSRF_BLOCKED` khoá Approve.
- `mcp-tool-diff.test.ts` (added/removed/changed/không đổi), `McpExternalServerList.test.tsx` (badge theo status, `toolsChanged`, ẩn nút admin với user thường, empty/error/loading).
- Tĩnh: test grep `dangerouslySetInnerHTML` = 0 trong thư mục `settings/mcp/`.
- Playwright/e2e: (chưa xác minh hạ tầng Playwright cho settings web) — nếu có, 1 kịch bản: user thêm server http → admin probe+approve → trạng thái `Approved`.

## Sửa TDD kèm theo
`frontend/tdd/v5/11-profile-ui.md` §2/§3: ghi chú `mcp.servers` là tham chiếu tên tới registry (secret/lệnh inline không được cấp cho agent) và `ProfileSourceBadge` chưa áp cho server MCP vì API không trả nguồn; `13-ai-provider-ui.md` §3: ghi nhận giới hạn của `CredentialInput` (ngưỡng 10 ký tự, `sessionToken` không tồn tại ⇒ fallback hằng số `'fallback-dev-token'`) — MCP **không** dùng lại cơ chế này (D1); sửa điểm yếu đó là follow-up riêng; `00-index.md`/danh mục settings: thêm tab `external-servers`.

## Không làm ở solution này
Sửa `McpConfigSection` hoặc đọc/ghi `.mcp.json`; cấp cấu hình cho agent; hiển thị lớp profile của server (chờ dữ liệu BE); tổng quát hoá `CredentialInput`; UI cho OAuth của server ngoài (BE chỉ hỗ trợ header/env tĩnh); thu hồi tiến trình agent đang chạy; mobile; trang Admin SPA riêng.
