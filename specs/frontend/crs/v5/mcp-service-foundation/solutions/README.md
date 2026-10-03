# frontend Solutions — MCP Service Foundation (v5)

**CRs:** [docs/crs/v5/mcp-service-foundation](../../../../../../docs/crs/v5/mcp-service-foundation/README.md) (CR-MCP-001/002)
**Hợp đồng bắt buộc:** [CONTRACT-mcp-ui-api.md](../../../../../backend-go/crs/v5/CONTRACT-mcp-ui-api.md) · **Quy ước FE + sự thật đã xác minh:** [crs/v5/README.md](../../README.md)
**Phía backend:** [BE-MCP-SOL-001](../../../../../backend-go/crs/v5/mcp-service-foundation/solutions/BE-MCP-SOL-001-scaffold-mcp-service.md), [BE-MCP-SOL-002](../../../../../backend-go/crs/v5/mcp-service-foundation/solutions/BE-MCP-SOL-002-gateway-mcp-endpoint-wiring.md)

## Feature này có UI không?

**Không có UI riêng.** CR-001/002 là hạ tầng backend (service + route `/mcp`). Phần frontend duy nhất là *nền tảng dùng chung* mà các feature MCP khác cần: kiểu dữ liệu, đường gọi kênh `mcp.*`, store, mục Settings, và quy tắc ẩn UI khi `MCP_ENABLED=false`. Vì vậy chỉ có một solution (FE-MCP-SOL-001) và nó **không hiển thị gì** cho tới khi một FE solution khác đăng ký tab.

## Re-verify (khảo sát mã frontend, 2026-10-01) — đối chiếu với giả định của README FE v5

| Khẳng định | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| "Settings > MCP" không có section toàn cục; chỉ `McpConfigSection` theo repo | Đúng; `useSettingsNavigationMetadata.ts` không có id `mcp`; không có danh sách section cứng khác | Không |
| Thêm section Settings = 3 chỗ | Đúng (metadata, `Settings.tsx` `<SettingsSection>` + `isSectionMounted`, `*-search.ts`). Thêm: nên bổ sung `'mcp'` vào union `SettingsNavTarget` để `openSettingsTarget({pane:'mcp'})` đúng kiểu — các section admin hiện không có trong union và được cast | Bổ sung nhỏ |
| `callRuntimeResult` + `getClientForEnvironment().subscribe` là đường gọi/stream | Đúng, **nhưng cả hai là hàm private trong `web-preload-api.ts` (4478 dòng)** ⇒ solution tách `web-mcp-api.ts` và inject; chỉ sửa `web-preload-api.ts` bằng 1 import + 1 property | Chi tiết triển khai |
| Stream đăng ký bằng `subscribeRuntimeStreamChannel` | **Không dùng được cho web**: hàm này từ chối `target.kind==='local'`, mà web khi chưa chọn môi trường luôn là `local` | **Lệch** ⇒ cần `window.api.mcp.subscribeEvents` |
| `unsubscribe()` dừng luồng | Với web session client, `unsubscribe` chỉ xoá callback cục bộ (`web-session-client.ts:69`); server giữ goroutine tới khi đóng socket | **Lệch** ⇒ chỉ đăng ký **một lần/app**, không theo mount; đề nghị BE-013 dọn theo `ctx` kết nối |
| Lỗi RPC là `Error(message)` dạng `"<CODE>: msg"` (CONTRACT C4) | Web nhận `{code:"internal", message: err.Error()}` (`session_dialect.go`); chuỗi gRPC thô sẽ là `rpc error: code=… desc=MCP_X: …` nếu gateway không cắt ⇒ BE-002 thêm `mcpChannelError`; FE vẫn parse chịu lỗi cả hai dạng | Phòng thủ hai đầu |
| Nhiều tham số vị trí (`consent.decide`: `requestId`, `{decision,scopes}`) | Web session dialect chỉ chuyển **một** object (`normalizeInboundMessage`) ⇒ mọi `mcp.*` nhận 1 object | **Lệch với CONTRACT §2 (không nói rõ)** ⇒ đề nghị đổi CONTRACT (thêm C10) |
| `ORCA_PLATFORM === 'web'` là cổng | `vite.config.ts` đặt cứng `'web'` cho mọi build trong `frontend/`; điều kiện "hoặc Electron kết nối runtime từ xa" của CONTRACT §6 thành vô nghĩa trong cây này | Đơn giản hoá: chỉ cần `window.api.mcp` tồn tại + `enabled` |
| Global sync hook nằm trong `useIpcEvents` | Đúng (`useDevServersSync()` ở dòng 866) | Không; `useIpcEvents` gọi bởi `App` ⇒ HIGH impact, chỉ thêm 1 dòng |
| i18n: có script `verify:localization-*` | `package.json` gọi nhưng `config/scripts/` **không có** file tương ứng | **Chưa xác minh** nơi script thật |

## Solutions

| Solution | CR | Area | Effort | Status |
|---|---|---|---|---|
| [FE-MCP-SOL-001](./FE-MCP-SOL-001-mcp-frontend-foundation.md) | CR-MCP-001/002 (phần FE) | `shared/`, `preload/api-types.ts`, `web/`, `runtime/`, `store/`, `hooks/`, `components/settings/` | Medium | ✅ Implemented — see Gaps |

## Thứ tự thực thi & phụ thuộc

```
FE-MCP-SOL-001  ──▶  FE-MCP-SOL-002 … 011 (mỗi cái: 1 dòng trong mcp-tab-registry + component riêng)
        └─ không cần BE sẵn để merge: khi mcp.server.info "not implemented" ⇒ UI ẩn, không lỗi
```

- Làm **đầu tiên** và merge độc lập; có thể merge trước cả BE-002 (kênh chưa có ⇒ coi như tắt).
- Thứ tự gợi ý bên trong: `shared/mcp-types.ts` → `runtime-mcp-*` → `web-mcp-api.ts` + preload typing → slice → `useMcpSync` → Settings 3 chỗ → `McpPane`.
- Phụ thuộc BE để *kiểm thủ công*: BE-MCP-SOL-002 (khung kênh, `MCP_DISABLED`) và BE-MCP-SOL-003 (`mcp.server.info`).
