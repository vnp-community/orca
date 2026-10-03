# frontend — Solutions cho CR v5 (MCP)

> **CR gốc:** [docs/crs/v5](../../../../docs/crs/v5/README.md) · **TDD áp dụng:** [specs/frontend/tdd/v5](../../tdd/v5/00-index.md) (+ [v4/03-admin-spa](../../tdd/v4/03-admin-spa.md), [v4/10-web-push-ui](../../tdd/v4/10-web-push-ui.md))
> **Hợp đồng bắt buộc với backend-go:** [CONTRACT-mcp-ui-api.md](../../../backend-go/crs/v5/CONTRACT-mcp-ui-api.md) · **Phía backend:** [specs/backend-go/crs/v5](../../../backend-go/crs/v5/README.md)
> Trạng thái: ✅ Implemented — see Gaps.

## 1. Cấu trúc & ánh xạ

```
specs/frontend/crs/v5/<feature>/solutions/
    README.md
    FE-MCP-SOL-NNN-<slug>.md
```

| Feature | FE solution | Gắn với CR / BE solution |
|---------|-------------|--------------------------|
| mcp-service-foundation | FE-MCP-SOL-001 nền tảng: kiểu dùng chung, runtime client, slice, mục Settings > MCP, lazy-load | CR-001/002 ↔ BE-002, BE-003 |
| mcp-protocol-server | FE-MCP-SOL-002 panel "Kết nối" + phiên MCP đang hoạt động | CR-003/004 ↔ BE-003, BE-004 |
| mcp-authorization | FE-MCP-SOL-003 trang consent `/oauth/consent` + ứng dụng đã kết nối + quản trị client; FE-MCP-SOL-004 Access tokens (PAT) | CR-005/006 ↔ BE-005, BE-006 |
| mcp-tool-catalog | FE-MCP-SOL-005 danh mục tool (admin); FE-MCP-SOL-006 nhãn "Tạo bởi agent" trên terminal/agent | CR-007/008/009 ↔ BE-007..009 |
| mcp-resources-prompts | FE-MCP-SOL-007 quản lý prompt tuỳ chỉnh (admin). *Resources (CR-010): không có UI — ghi rõ trong README feature* | CR-010/011 ↔ BE-010, BE-011 |
| mcp-governance-safety | FE-MCP-SOL-008 policy + kill switch (admin); FE-MCP-SOL-009 hộp thoại & hộp thư phê duyệt; FE-MCP-SOL-010 nhật ký kiểm toán MCP | CR-012/013 ↔ BE-012, BE-013 |
| mcp-client-registry | FE-MCP-SOL-011 registry MCP server ngoài | CR-014 ↔ BE-014 |
| mcp-quality-rollout | FE-MCP-SOL-012 kiểm thử (vitest/Playwright), telemetry, tài liệu, rollout | CR-015 ↔ BE-015 |

## 2. Hiện trạng frontend đã xác nhận (khảo sát mã thật, 2026-10-01)

| Hạng mục | Thực tế | Hệ quả thiết kế |
|----------|---------|-----------------|
| "Settings > MCP" | docs/ui/page-tree.md:50 liệt kê nhưng **không có** section toàn cục trong code; chỉ có khối **MCP Configs theo repo** (`components/settings/McpConfigSection.tsx`, gắn ở `RepositoryPane.tsx`) đọc file `.mcp.json`… | Tạo **section Settings mới `mcp`**; **không** đụng `McpConfigSection` (khác bản chất: file cấu hình cục bộ) |
| Thêm section Settings | 3 chỗ: entry trong `hooks/useSettingsNavigationMetadata.ts`, `<SettingsSection id="mcp">` trong `components/settings/Settings.tsx` (lazy `isSectionMounted`), file `*-search.ts` | FE-001 làm đủ 3 chỗ |
| Admin | Tab trong `components/settings/AdminOrgConsole.tsx` (`admin-org-console-<x>-tab.tsx`), gate `currentUser?.role === 'admin'`; nhãn tab hiện hard-code tiếng Anh | Thêm tab theo cùng khuôn; nhãn mới dùng `translate()` |
| Gọi backend | WS RPC `callRuntimeResult(method, params)` qua shim `window.api` (`web/web-preload-api.ts`), lỗi = `Error(message)`; stream bằng `getClientForEnvironment(env).subscribe(...)` + teardown (mẫu `nativeChat.subscribe`) | Dùng đúng CONTRACT §0 C1/C4/C5; thêm namespace `window.api.mcp` + type vào `PreloadApi` |
| Store | Zustand slice `StateCreator<AppState, [], [], XSlice>` (`store/slices/*`), đăng ký ở `store/index.ts` + `store/types.ts`; test cạnh slice | FE-001 thêm `mcp-slice.ts` |
| UI primitives | `components/ui/` **không** có `switch`, `alert`, `alert-dialog`, `avatar`; có `dialog`, `sheet`, `table`, `tabs`, `badge`, `select`, `checkbox`, `sonner`; xác nhận bằng `components/confirmation-dialog.tsx` | Không giả định primitive thiếu; dùng `Checkbox`/`Button` hoặc mẫu `BrowserUseEnableSwitch.tsx` |
| Style | Chuẩn thật ở `guides/STYLEGUIDE.md` (không phải docs/STYLEGUIDE.md); token ở `assets/main.css`; không hard-code màu; Dialog giữa/Sheet cạnh; nút Cancel là ghost, `destructive` chỉ cho mất dữ liệu | Mọi solution trích quy tắc liên quan |
| i18n | `translate('auto.<path>.<id>', 'English')`; **cấm** gọi `translate` ở top-level (`no-top-level-translate.test.ts`); locale `en/es/ja/ko/zh` | Chuỗi mới có khoá `auto.mcp.*` + fallback tiếng Anh; ghi vào `en.json` |
| Web Push | `hooks/useWebPushSubscription.ts` dùng `/api/vapid-public-key`, `/api/push-subscribe`, `/api/push-unsubscribe` (TDD v4/10 ghi `/push/...` là **lỗi thời**); nguồn `service-worker.js` được tạo ở `frontend/src/renderer/public/service-worker.js` (D5; trước đó không có); các `fetch` của hook không đặt `credentials:'include'`; không có kênh `notifications.subscribe` | Deep link `/?section=mcp&tab=approvals&approval=<id>`: SW `notificationclick` ⇒ `postMessage({type:'orca:navigate'})`/`openWindow`, SPA parse query lúc boot rồi `openSettingsTarget`; hộp thoại khi app mở là đường chính; push thật phụ thuộc CR-NOTIF-002 (`DeliverPush` chưa có) — FE-009 |
| TDD cũ | v5/02 (≈36 slice) và v5/05 (AdminApp) **lỗi thời** so với mã | Solution tham chiếu mã thật; mục "Sửa TDD kèm theo" liệt kê |
| Bất biến | Không sửa `App.tsx`, `main.tsx`, `web-preload-api.ts` ngoài **thêm kênh/namespace** (nguyên tắc 11, 19 của TDD v5 00-index); mọi `on*()` phải có cleanup (nguyên tắc 12); web-only gate `ORCA_PLATFORM === 'web'` | Ghi trong từng solution |

## 3. Mẫu nội dung một FE solution (khuôn `crs/v4/team-rbac/solutions`)

`# FE-MCP-SOL-NNN: …` → banner trạng thái → **CR Reference** (CR link, mức ưu tiên, phạm vi *chỉ phần FE*) → **Backend dependency** (bảng kênh CONTRACT + BE solution; nói rõ khi BE chưa có thì FE làm gì) → **Impact analysis (gitnexus)** (bảng Symbol/Direction/Risk — chỉ ghi lệnh cần chạy + rủi ro dự kiến, **không bịa số**) → **Bối cảnh (đã xác nhận lại)** kèm đường dẫn file thật → **Giải pháp** (`### Bước N` + `**File:** … (NEW|MODIFY)` + code TS/TSX) → **Trạng thái UI** (loading/empty/error/forbidden/disabled-by-flag) → **A11y & i18n & style** → **Files cần sửa** (bảng) → **Verification** (`cd frontend && npx vitest run …`, `npx tsc --noEmit -p tsconfig.json`, Playwright nếu có) → **Sửa TDD kèm theo** → **Không làm ở solution này**.
