# SOL-EMU-007: Frontend — chọn Mobile Emulator Agent & nối `emulator.*` remote

**Resolves:** CR-DS-009 §2.3/§4, Phase 5
**Service:** `frontend/`
**Status:** ✅ DONE
**Task(s):** [TASK-EMU-011](../tasks/TASK-EMU-011-mobile-emulator-agent-onboarding-ui.md), [TASK-EMU-012](../tasks/TASK-EMU-012-settings-agent-picker.md), [TASK-EMU-013](../tasks/TASK-EMU-013-emulator-pane-remote-wiring.md)

## Vấn đề

Toàn bộ backend (`emulator/`, `packages/dev-agent-transport/`, backend-go's `AgentKind`/project binding/routing — Phase 1-4) đã xong, nhưng phía `frontend/`:
1. Không có UI nào để user chọn "Mobile Emulator Agent nào cho project này" — field `mobileEmulatorAgentId` (TASK-EMU-008) không ai gọi.
2. Mọi lời gọi `emulator.*` từ `EmulatorPane`/`MobileEmulatorSettingsPane` **hard-code `{kind:'local'}`** — chỉ chạy được ở Electron desktop, không bao giờ đi qua backend-go dù ở web/server mode.
3. Không có cách đăng ký (pair) một Mobile Emulator Agent từ UI.

## Giải pháp đã triển khai

- **Type + selector**: `AgentKind`/`kind` trên `DevServer`, `useDevServersOnly()`/`useMobileEmulatorAgents()` selector (TASK-EMU-012a).
- **UI chọn agent cho project**: `ProjectMobileEmulatorAgentSection.tsx` mirror `ProjectDevServerSection.tsx`, lưu qua `project.update` (TASK-EMU-012b).
- **Đăng ký agent**: tái dùng `AddDevServerDialog.tsx` với `kind` selector thay vì dialog riêng; hướng dẫn build-from-source thật, không bịa installer (TASK-EMU-011, TASK-EMU-012c).
- **Nối `emulator.*` remote**: `resolveEmulatorPaneRuntimeTarget()` — target động (`getActiveRuntimeTarget`) + `projectId`, additive, xác nhận `{kind:'local'}` desktop mặc định không đổi (TASK-EMU-013).

## Giới hạn đã biết (ghi trung thực, không phải "chưa xong nhưng giấu")

1. Backend-go's `devServer.list`/`listForUser`/`add` wscompat channel (`channels.go`) **chưa đọc/trả field `kind`** — filter `kind` ở web/server mode hiện là no-op an toàn (không lọc được, không lỗi). Cần 1 task backend-go riêng nối field này qua view struct.
2. `AgentTokenPanel.tsx` (component hiển thị lệnh khởi động sau khi có token) **không được render ở đâu trong app** — gap có sẵn từ trước CR-DS-009, áp dụng như nhau cho Dev Server Agent lẫn Mobile Emulator Agent, không mở rộng phạm vi để vá.
3. Chưa verify end-to-end với `infra-fleet-service`/`emulator/` thật chạy qua docker-compose — cần TASK-EMU-006 (transport thật) hoàn thiện phần đăng ký `AgentKind` trước.

## Verify

`tsc --noEmit`: 113 lỗi, khớp baseline đo bằng `git stash`/tsc/stash-pop trước khi sửa — zero lỗi mới. `vitest run`: 25 file fail / 131 test fail, khớp baseline (đã tìm và sửa 1 regression thật trong lúc verify — xem TASK-EMU-012). `vite build`: thành công (`✓ built in 1m 4s`).

## Sự cố hạ tầng trong pass triển khai

Lượt đầu triển khai phần này bị chặn giữa chừng bởi `ENOSPC` (hết dung lượng đĩa) của sandbox — không phải lỗi trong code. Đã dọn ~17GB cache/rác hệ thống (go build cache, apt, trivy, playwright, pnpm store prune, journal logs — không đụng source code/toolchain) ở một phiên riêng trước khi hoàn tất pass này.
