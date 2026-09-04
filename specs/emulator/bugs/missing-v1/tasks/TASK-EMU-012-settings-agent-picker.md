# TASK-EMU-012: `kind` trên `DevServer` + UI chọn Mobile Emulator Agent

**Solution:** [SOL-EMU-007](../solutions/SOL-EMU-007-frontend-agent-selection-ui.md)
**Priority:** P2
**Status:** `[x]` DONE

## Việc đã làm

### 012a — `kind` trong type + API list

- `frontend/src/shared/dev-server-types.ts`: thêm `type AgentKind = 'dev-server' | 'mobile-emulator'`, field `kind?: AgentKind` trên `DevServer`/`DevServerInput` (map 1:1 `AGENT_KIND_DEV_SERVER`/`AGENT_KIND_MOBILE_EMULATOR` — TASK-EMU-007), `type DevServerListFilter = { kind?: AgentKind }`.
- `frontend/src/preload/api-types.ts`, `frontend/src/renderer/src/web/web-preload-api.ts`: `devServer.list`/`listForUser` nhận `filter?: DevServerListFilter` optional — forward qua `callRuntimeResult`.
- `frontend/src/renderer/src/store/slices/dev-servers-selectors.ts`: thêm `useDevServersOnly()` (loại `mobile-emulator`) và `useMobileEmulatorAgents()` (chỉ `mobile-emulator`) — filter trên slice sẵn có, không tạo slice mới.

**Giới hạn đã ghi rõ trong code (comment ở `web-preload-api.ts`)**: backend-go's `channels.go`'s `devServer.list`/`listForUser`/`add` wscompat channel **hiện chưa đọc `kind` từ args hay trả field `kind`** (`devServerView` struct không có `Kind`) — filter ở web/server mode hiện là **no-op an toàn** (không lọc được gì nhưng cũng không lỗi), cho tới khi có task backend-go riêng nối field này vào wscompat layer (`ListDevServers`/`RegisterDevServer`'s RPC đã có field `kind` từ TASK-EMU-007, chỉ thiếu bước wscompat → gRPC → wscompat response chưa nối field này qua view struct).

### 012b — UI chọn Mobile Emulator Agent cho project

- `frontend/src/renderer/src/types/workspace-types.ts`: `OrcaProject += mobileEmulatorAgentId?: string`.
- File mới `frontend/src/renderer/src/components/project/ProjectMobileEmulatorAgentSection.tsx` — mirror `ProjectDevServerSection.tsx`: load `devServer.list({kind:'mobile-emulator'})`, sync từ `project.mobileEmulatorAgentId`, lưu qua `project.update({id, mobileEmulatorAgentId})` (KHÔNG qua `project.rebindDevServer` — field đó và guard riêng của nó chỉ cho `devServerId`, TASK-EMU-008 cố tình không mở rộng).
- `ProjectSettings.tsx`: gắn component trên vào tab General, cạnh `ProjectDevServerSection`.

### 012c — Luồng "Add Mobile Emulator Agent"

- `AddDevServerDialog.tsx`: thêm `Select` chọn `kind` ('dev-server' | 'mobile-emulator'), forward vào `testConnection`/`addAndConnect`'s input (đã có sẵn field `kind?` trên `DevServerInput`). Title/description/placeholder/nút "Add Agent" đổi theo kind. Thêm prop `initialKind` để caller mở thẳng vào chế độ Mobile Emulator Agent.
- Khi `connectionType === 'direct-websocket'`: hiển thị khối hướng dẫn build-from-source thật (`cd emulator && pnpm install && node build.mjs` rồi lệnh chạy với `ORCA_BACKEND_URL`/`ORCA_AGENT_TOKEN`) — **không** tạo installer "curl\|bash" giả vờ, vì repo chưa có hạ tầng phân phối binary đã build sẵn (không npm publish, không GitHub release cho `emulator/`/`agent/`).
- `AgentTokenPanel.tsx`: thêm prop `agentKind?: AgentKind` (mặc định `'dev-server'` — mọi call site trước task này không truyền gì, nên mặc định giữ nguyên hành vi cũ) — đổi lệnh hiển thị (`node out/emulator.js` thay vì `node agent.js`) và nhãn "Run on..." khi `mobile-emulator`. **Ghi chú quan trọng**: component này hiện **không được render ở bất kỳ đâu trong app** (grep xác nhận, chỉ tự tham chiếu chính nó) — đây là gap có sẵn từ trước (`specs/frontend/crs/v2/agent/tasks/TASK-FE-007`/`SOL-FE-AG-002` xây component này nhưng chưa từng nối vào sự kiện `agentTokenGenerated` thật), áp dụng cho **cả 2 loại agent như nhau**, không phải gap do CR-DS-009 tạo ra. Không mở rộng phạm vi task này để wire nốt gap đó.
- `MobileEmulatorSettingsPane.tsx`: thêm section "Mobile Emulator Agents" — liệt kê agent đã pair (`useMobileEmulatorAgents()`) + nút "+ Add Mobile Emulator Agent" mở `AddDevServerDialog` với `initialKind="mobile-emulator"`.

## Verify (chạy thật, có bằng chứng)

```
$ node <ts>/bin/tsc --noEmit -p frontend/tsconfig.json
113 lỗi — KHỚP CHÍNH XÁC baseline đo trước khi sửa (113, không phải 110 như ước tính ban đầu của pass trước — số baseline chính xác được xác nhận bằng git stash/tsc/stash-pop). Zero lỗi mới.

$ node node_modules/vitest/vitest.mjs run --config config/vitest.config.ts
Test Files  25 failed | 1871 passed | 3 skipped (1899)
     Tests  131 failed | 16404 passed | 8 skipped (16543)
```
Khớp chính xác baseline (25 file fail / 131 test fail).

```
$ node node_modules/vite/bin/vite.js build
✓ built in 1m 4s
```

## Regression tìm thấy và đã sửa trong pass này

`ProjectMobileEmulatorAgentSection.tsx`'s `.then((list) => setAgents(list ?? []))` chỉ guard `null`/`undefined`, không guard giá trị resolve không phải mảng — trong môi trường test (RPC mock trả về giá trị khác mảng cho `devServer.list`), `agents.map` throw, làm sập toàn bộ `<ProjectSettings>` (test `ProjectSettings.test.tsx` không tìm thấy `repo-picker-item-r1` vì cả cây React crash). Sửa thành `Array.isArray(list) ? list : []`. Test suite quay lại khớp baseline sau fix.
