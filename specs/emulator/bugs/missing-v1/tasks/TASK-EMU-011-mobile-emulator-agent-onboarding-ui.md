# TASK-EMU-011: Onboarding UI cho Mobile Emulator Agent

**Solution:** [SOL-EMU-007](../solutions/SOL-EMU-007-frontend-agent-selection-ui.md)
**Priority:** P2
**Status:** `[x]` DONE — tái dùng luồng "Add Dev Server" có sẵn thay vì luồng riêng

## Quyết định thiết kế

Không tạo dialog/luồng "Add Mobile Emulator Agent" hoàn toàn tách biệt. `AddDevServerDialog.tsx` (dùng chung bởi cả `DevServerList.tsx` và `MobileEmulatorSettingsPane.tsx` mới) mở rộng thêm 1 `Select` chọn `kind` — vì cả 2 loại agent đăng ký qua **cùng một registry** (`DevServer`, phân biệt bởi `kind` — TASK-EMU-007), toàn bộ luồng test-connection/add/connect là **y hệt nhau**, chỉ khác nhãn hiển thị và lệnh khởi động binary. Tách dialog riêng sẽ nhân đôi logic mà không có lợi ích gì.

Entry point mới: nút "+ Add Mobile Emulator Agent" trong `MobileEmulatorSettingsPane.tsx` (section "Mobile Emulator Agents") mở `AddDevServerDialog` với `initialKind="mobile-emulator"` — người dùng vẫn có thể chọn kind thủ công từ `DevServerList.tsx`'s "+ Add Dev Server" nếu muốn.

## Về "installer script"

Không tạo script `curl | bash`/`.exe` cài đặt — repo hiện **không có hạ tầng phân phối binary đã build sẵn** cho `emulator/` (không npm publish, không GitHub Release). Một installer thật sẽ không có gì để tải. `AddDevServerDialog`'s khối hướng dẫn khi chọn `direct-websocket` hiển thị lệnh build-from-source thật:
```
cd emulator && pnpm install && node build.mjs
ORCA_BACKEND_URL=<url> ORCA_AGENT_TOKEN=<token> node out/emulator.js
```
Khi nào repo có hạ tầng phân phối (release binary), khối này đổi thành lệnh tải/chạy binary — không cần đổi kiến trúc, chỉ đổi text hiển thị.

## Chi tiết triển khai + verify

Xem [TASK-EMU-012](./TASK-EMU-012-settings-agent-picker.md) — cùng 1 lượt sửa, chung bằng chứng verify (`tsc`, `vitest`, `vite build`).
