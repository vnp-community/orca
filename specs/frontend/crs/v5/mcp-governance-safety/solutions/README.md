# frontend Solutions — MCP Governance & Safety (v5)

**CRs:** [docs/crs/v5/mcp-governance-safety](../../../../../../docs/crs/v5/mcp-governance-safety/README.md) (CR-MCP-012, CR-MCP-013 — phần UI mà CR ghi là "frontend CR riêng")
**Hợp đồng bắt buộc:** [CONTRACT-mcp-ui-api.md](../../../../../backend-go/crs/v5/CONTRACT-mcp-ui-api.md) · **Quy ước FE v5:** [crs/v5/README.md](../../README.md) · **Phía backend:** [specs/backend-go/crs/v5/mcp-governance-safety/solutions](../../../../../backend-go/crs/v5/mcp-governance-safety/solutions/README.md)
**TDD tham chiếu:** [v5/00-index](../../../../tdd/v5/00-index.md) (nguyên tắc 11, 12, 19), [v4/03-admin-spa](../../../../tdd/v4/03-admin-spa.md), [v4/10-web-push-ui](../../../../tdd/v4/10-web-push-ui.md)
**Feature này CÓ UI** (ba solution bên dưới); không áp dụng ngoại lệ "không có UI".

## Solutions

| Solution | CR | Service / Area (FE) | Effort | Status |
|---|---|---|---|---|
| [FE-MCP-SOL-008](./FE-MCP-SOL-008-admin-policies-and-killswitch.md) | CR-MCP-012 (+013 §C) | Settings › MCP › tab "Tool policies" (match builder, explain, cài đặt tenant), panel Kill switch, banner | Medium | 🔲 Designed — chưa implement |
| [FE-MCP-SOL-009](./FE-MCP-SOL-009-approval-dialog-and-inbox.md) | CR-MCP-013 §A | Lớp toàn cục `McpGlobalLayer` + hộp thoại phê duyệt (additive vào `App.tsx`), tab "Approvals", deep link | Large | 🔲 Designed — chưa implement |
| [FE-MCP-SOL-010](./FE-MCP-SOL-010-mcp-audit-log-viewer.md) | CR-MCP-013 §B | Settings › MCP › tab "MCP audit" (lọc, con trỏ, chi tiết, CSV) | Medium | 🔲 Designed — chưa implement |

## Re-verify: khẳng định (CR / README FE v5 / CONTRACT) vs mã thật (2026-10-01)

| Khẳng định | Thực tế | Lệch? | Xử lý |
|---|---|---|---|
| Admin UI = tab trong `AdminOrgConsole.tsx` (README FE v5) | Đúng cho cơ chế, **nhưng** console đã có tab **"Policies"** (access-policy, `admin.listPolicies`) và **"Audit Log"** | Trùng tên khái niệm | Tab MCP sống trong section Settings `mcp`, nhãn "Tool policies" / "MCP audit"; không sửa `AdminOrgConsole` |
| Shim API ở `runtime/web/web-preload-api.ts` (README FE v5) | File thật: `frontend/src/renderer/src/web/web-preload-api.ts` (`callRuntimeResult` dòng ~3498, `createAdminApi` ~2668, `nativeChat.subscribe` ~1182); kiểu `PreloadApi` ở `frontend/src/preload/api-types.ts:927` | Đường dẫn sai trong README FE | Các solution dùng đường dẫn thật |
| Gate admin | `useAppStore((s) => s.currentUser?.role === 'admin')` (`Settings.tsx:283`, `DepartmentGate.tsx:29`) | Khớp | Dùng nguyên |
| Settings điều hướng bằng URL (deep link CONTRACT §4 nay là `/?section=mcp&tab=approvals&approval=<id>`, D5) | Settings điều hướng bằng store: `openSettingsTarget({pane, repoId, sectionId?})` + `openSettingsPage()` (`store/slices/ui.ts:779-786`); `SettingsNavTarget` ở `lib/settings-navigation-types.ts` | Không có router URL cho Settings | `mcp-deep-link.ts` đọc query → store lúc boot (cold start) và khi nhận `orca:navigate`; dùng đường dẫn gốc `/` nên không cần SPA-fallback riêng (parser không phụ thuộc pathname) |
| Có điểm gắn overlay toàn cục không cần sửa sâu `App.tsx` | Có tiền lệ đúng loại: `sshCredentialQueue` (`store/slices/ssh.ts`) + `SshPassphraseDialog` mount có điều kiện trong `App.tsx` (~2816) bên trong `ConfirmationDialogProvider`; `lazy = lazyWithRetry` | Khớp | `McpGlobalLayer` gắn additive (3 phần) theo khuôn này |
| Web cũng render `<App/>` | Đúng (`web/main-web-bootstrap.tsx` WebRoot; `web/pair-code-app-entry.tsx:75`) | Khớp | Một điểm gắn phủ web + Electron-remote |
| Web Push `notificationclick` điều hướng deep link | `main-web-bootstrap.tsx:455` đăng ký `/service-worker.js` nhưng (trước D5) nguồn SW **không có** trong repo; TDD v4/10 mô tả SW chỉ `focus()`/`openWindow('/')`, và BE gửi `deepLink` ở cấp cao nhất payload (không trong `data`) | **D5 (đã chốt):** nguồn SW được tạo tại `frontend/src/renderer/public/service-worker.js` (push ⇒ notification từ `{title, body, deepLink, tag}`; click ⇒ focus + `postMessage({type:'orca:navigate'})` hoặc `openWindow`) | FE-009 thiết kế phía SPA (parse query lúc boot + nghe `orca:navigate`); hộp thoại trong app vẫn là đường chính. **Phụ thuộc thật còn lại:** CR-NOTIF-002 (`DeliverPush` chưa có ⇒ push chưa được gửi) |
| Không có `switch`/`alert`/`alert-dialog` primitive | Đúng (`ls components/ui`); có `BrowserUseEnableSwitch.tsx` (bespoke, nhãn hard-code) | Khớp | Dùng `Checkbox`+`Label`; khối lỗi `role="alert"` tự dựng bằng token |
| Tab Audit hiện có dùng được làm khuôn | `admin-org-console-audit-tab.tsx` (144 dòng): chỉ `since`, **không** `to`, **không** phân trang (bỏ `nextPageToken`), chuỗi hard-code | Thiếu cho nhu cầu MCP | FE-010 tái dùng bố cục, viết logic mới (con trỏ, chi tiết, CSV); không sửa file v4 (FE-SOL-003) |
| `QueryAuditLog` trả theo thời gian | BE sắp theo `id` UUID ngẫu nhiên (xem BE-013 §E) | **Drift ảnh hưởng FE-010** | FE-010 phụ thuộc BE thêm thứ tự keyset; FE không tự sắp lại |
| Primitives/Style | `guides/STYLEGUIDE.md` (không phải `docs/STYLEGUIDE.md`): `destructive` chỉ cho mất dữ liệu/không hoàn tác; Cancel/Dismiss ghost; Dialog cho quyết định bắt buộc, Sheet cho panel cạnh | Khớp README FE | Xem quyết định ở FE-008 (kill switch) và FE-009 (Deny ghost, focus vào Deny) |
| Store có middleware persist | Không thấy `persist`/`partialize` trong `store/index.ts`; lưu bền theo từng slice qua `window.api.ui.set` (**chưa xác minh** mọi đường) | — | Slice approval cố ý không persist, có test khẳng định |

## Thứ tự thực thi & phụ thuộc

```
FE-MCP-SOL-001 (kiểu, window.api.mcp, mcp-slice, section `mcp`, event stream ref-counted, parseMcpError, isMcpSurfaceAvailable)
   ├─▶ FE-MCP-SOL-008  (đăng ký tab + banner; cần BE-012 cho policy/settings, BE-013 cho kill switch)
   ├─▶ FE-MCP-SOL-009  (cần BE-013: approval.list/decide + events; mở đầu tiên vì chặn "bật tool exec")
   └─▶ FE-MCP-SOL-010  (cần BE-013 §E keyset thời gian)
```
- **Mốc an toàn:** FE-009 phải xong **trước** khi bật tool `exec/destructive` cho người dùng thật (không có UI duyệt ⇒ approval chỉ hết hạn, an toàn nhưng vô dụng). FE-008 cần trước khi admin phải vận hành sự cố (kill switch). FE-010 có thể sau.
- Ba solution dùng chung: `shared/mcp-governance-types.ts` (FE-008 tạo; 009/010 thêm additive), `components/settings/mcp/mcp-governance-tabs.tsx` (FE-008), tên method `window.api.mcp.*` (camelCase hoá kênh — **nếu FE-001 chốt tên khác thì đổi theo**).
- Điểm phối hợp với FE-001 cần chốt khi merge: (1) `onMcpEvent(listener)` — **một** stream `mcp.events.subscribe` mỗi tab; (2) `parseMcpError`; (3) `isMcpSurfaceAvailable()`; (4) `McpSettingsSection` nhận tab từ `useMcpGovernanceTabs()` + render `McpKillSwitchBanner` + đọc `mcpUiIntent`; (5) `SettingsNavTarget` thêm `'mcp'`.
- Yêu cầu thay đổi CONTRACT liên quan tới FE: R-1 (mã `MCP_INVALID_ARGUMENT/INTERNAL/TIMEOUT`), R-2 (tham số = một object ở `args[0]`), R-3/R-4 (tuỳ chọn), R-5 (`tenantEnabled`), R-6 (`mcp.admin.killswitch.list`) — xem [README backend](../../../../../backend-go/crs/v5/mcp-governance-safety/solutions/README.md).
