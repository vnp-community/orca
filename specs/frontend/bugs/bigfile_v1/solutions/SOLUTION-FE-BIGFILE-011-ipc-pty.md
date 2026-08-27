# SOLUTION-FE-BIGFILE-011 — Tách `ipc/pty.ts` (5,185 dòng)

**Bug:** `../BUG-FE-BIGFILE-011-ipc-pty.md`
**Trạng thái:** 📝 Proposed
**Thứ tự thực hiện:** #1 (rủi ro thấp nhất, làm trước — xem `SOLUTION-FE-BIGFILE-001` mục 3)
**Chiến lược:** Barrel/facade (xem `SOLUTION-FE-BIGFILE-001` mục 1)

---

## Cấu trúc file mới

| File mới | Nội dung chuyển sang | Dòng gốc | Ước tính dòng |
|---|---|---|---|
| `pty-pane-key-registry.ts` | `getPtyIdForPaneKey`, `registerPaneKeyTeardownListener`, `hasPendingRendererSerializerForPaneKey` | 219–460 | ~240 |
| `pty-startup-color-query.ts` | `answerStartupTerminalColorQueriesForPty` | 461–544 | ~85 |
| `pty-host-env.ts` | `type BuildPtyHostEnvOptions`, `buildPtyHostEnv` | 545–833 | ~290 |
| `pty-ownership-registry.ts` | `register/unregisterRemotePtyProvider`, `getRemotePtyProvider`, `get/setLocalPtyProvider`, `getPtyIdsForConnection`, `clearPtyOwnershipForConnection`, `clearProviderPtyState`, `delete/setPtyOwnership`, `rebindLocalProviderListeners` | 1050–1251 | ~200 |
| `pty-renderer-delivery-debug.ts` | `type PtyRendererDeliveryDebugSnapshot`, `getPtyRendererDeliveryDebugSnapshot`, `resetPtyRendererDeliveryDebug` | 1252–1357 | ~110 |
| `pty-provider-listener-binding.ts` | `unbindLocalProviderListeners` | 1448–1458 | ~10 |
| `ipc/pty.ts` (giữ nguyên tên) | `registerPtyHandlers`, `registerHeadlessPtyRuntime`, `killAllPty` — phần lõi IPC registration, KHÔNG tách trong solution này (xem mục "Không làm trong bước này") | 1459–5185 | ~3,730 (giảm từ 5,185) |

**Sau bước này**: `ipc/pty.ts` giảm từ 5,185 → ~3,730 dòng (giảm 28%). Vẫn còn
trên ngưỡng Critical — cần bước 2 (xem mục cuối) để xử lý `registerPtyHandlers`.

## Các bước thực hiện

1. **Đọc lại comment dòng 1 đầy đủ** trước khi bắt đầu:
   > "PTY IPC is intentionally centralized in one main-process module so
   > spawn-time environment scoping, lifecycle cleanup, foreground-process
   > inspection, and renderer IPC stay behind a single audited boundary.
   > Splitting it by line count would scatter tightly coupled terminal..."
   (câu bị cắt — đọc tiếp phần còn lại trong file trước khi quyết định ranh
   giới cuối cùng, có thể có ràng buộc kỹ thuật cụ thể về thứ tự khởi tạo).
2. `gitnexus impact({target: "getPtyIdForPaneKey", direction: "upstream"})`
   (và tương tự cho từng export sẽ di chuyển) — xác nhận danh sách caller,
   đặc biệt lưu ý caller trong `desktop/`/`frontend/` split-repo copies (nếu
   file này có bản sao ở nơi khác, áp dụng đồng thời).
3. Tạo `pty-pane-key-registry.ts`: copy nguyên văn 3 hàm (219–460) + import cần
   thiết. `ipc/pty.ts` đổi thành `export { ... } from './pty-pane-key-registry'`.
4. Lặp lại bước 3 cho từng file còn lại theo thứ tự bảng trên (mỗi file 1
   commit riêng, xác nhận xanh trước khi sang file tiếp theo).
5. Sau khi 6 file đã tách, `ipc/pty.ts` chỉ còn: import + `export { ... }` cho
   6 file trên + phần `registerPtyHandlers`/`registerHeadlessPtyRuntime`/
   `killAllPty` giữ nguyên tại chỗ.

## Không làm trong bước này

`registerPtyHandlers` (dòng 1459–5150, ~3,691 dòng, 71% file gốc) **không**
tách trong solution này — đây là 1 hàm orchestration lớn đăng ký toàn bộ IPC
channel cho PTY, cần thiết kế riêng (nhóm theo domain channel: create/attach,
write/resize, serialize/scrollback, signal/kill) và nên làm SAU khi đã có
kinh nghiệm từ việc tách 6 file trên. Đề xuất theo dõi như 1 solution riêng
sau khi bước này hoàn tất và đo lại kích thước.

## Xác minh

- `pnpm run typecheck` (3 target)
- `pnpm run lint`
- `gitnexus detect_changes({scope: "all"})`
- `node scripts/find-frontend-bigfiles.mjs` — xác nhận `ipc/pty.ts` giảm
  xuống ~3,730 dòng

## Rủi ro

Thấp — toàn bộ export di chuyển đều là pure function/type, không phụ thuộc
`this`, không có state module-level chia sẻ ẩn giữa các nhóm (cần xác nhận
lại khi đọc file thật, đặc biệt `pty-ownership-registry.ts` có khả năng cao
giữ 1 `Map`/`Set` module-level dùng chung giữa các hàm trong nhóm đó — giữ cả
nhóm trong CÙNG 1 file mới, không tách nhỏ hơn nữa).
