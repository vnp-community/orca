# FE-CV-TASK-050-06: Preload Electron `codeIntel` (ngoài `frontend/`, cần chủ sở hữu desktop duyệt)

**From Solution:** [FE-CV-SOL-050-types-and-runtime-bridge](../solutions/FE-CV-SOL-050-types-and-runtime-bridge.md) mục 1 (Electron), 9
**Priority:** P1
**Area:** desktop (ngoài `frontend/`, cần chủ sở hữu desktop duyệt)
**File:** `desktop/src/preload/index.ts` (sửa), `desktop/src/preload/api-types.ts` (sửa)
**Depends on:** FE-CV-TASK-050-04
**Status:** [~] PARTIAL — desktop/src/preload/index.ts uses createCodeIntelBridge (local=method_not_found, env=runtimeEnvironments:call) but has no test and the subscribeRuntimeEnvironmentFromPreload frame shape was not verified against the new raw-frame callbacks; needs desktop owner review

## Context

- `desktop/src/preload/index.ts` (:4012-4044) có `runtimeEnvironments` (list/call/subscribe qua `ipcRenderer.invoke('runtimeEnvironments:…')`); không có `mcp`. `desktop/src/shared` và `desktop/src/renderer/src` là bản riêng; `electron.vite.config.ts` đặt alias `@` vào `desktop/src/renderer/src`.
- Chưa kiểm chứng desktop import được `frontend/src/shared`.

## Việc cần làm

1. Điều tra (đọc, không sửa): bundle Electron thật dùng renderer nào; ghi kết luận vào PR.
2. Nếu preload dùng được: gắn `codeIntel` bằng cùng logic `createCodeIntelBridge` (sao chép file nếu không import chéo gói được; đặt trong `desktop/src/shared/` với tên cụ thể).
3. Nếu không: **không thêm** gì; ghi chặn trong PR; `useCodeIntelSupport` trả `unsupported` cho đích local (đã là hành vi mặc định, UI-API §7).

## Kiểm thử

- Test preload của desktop theo mẫu có sẵn (chưa đọc); nếu không có thì kiểm tay: Electron local ⇒ tab không hiện.

## Tiêu chí hoàn thành

- [x] Hoặc có `window.api.codeIntel` ở Electron, hoặc có ghi chú chặn rõ ràng; không phá build desktop.

## Rủi ro

- Hai bản mã nguồn renderer lệch nhau: Review ở Electron có thể trễ cả giai đoạn.
