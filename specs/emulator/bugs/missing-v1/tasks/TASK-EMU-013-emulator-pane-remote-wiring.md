# TASK-EMU-013: Nối `emulator.*` calls vào target động + `projectId`

**Solution:** [SOL-EMU-007](../solutions/SOL-EMU-007-frontend-agent-selection-ui.md)
**Priority:** P2
**Status:** `[x]` DONE

## Điều kiện an toàn quan trọng nhất (đã xác nhận)

`getActiveRuntimeTarget(settings)` (`frontend/src/renderer/src/runtime/runtime-rpc-client.ts`) trả về `{kind:'local'}` bất cứ khi nào không có runtime environment nào active — đúng mặc định của Electron desktop. Đổi các lời gọi `emulator.*` từ hard-code `{kind:'local'}` sang hàm này là **additive**: hành vi desktop hiện tại không đổi, chỉ khi có `{kind:'environment', environmentId}` active mới bắt đầu gửi `projectId` kèm theo.

Đã xác nhận thêm: schema Zod của các handler `emulator.*` phía local (Electron, `desktop/src/main/runtime/rpc/methods/emulator.ts`) đều `z.object({...})` **không** `.strict()` — Zod mặc định strip field lạ không khai báo trong schema. Nghĩa là thêm `projectId` vào params không làm handler local nhận lỗi hay đổi hành vi — field đó chỉ có ý nghĩa ở nhánh remote (backend-go, TASK-EMU-009's `resolveEmulatorConnectionID`).

## Việc đã làm

File mới `frontend/src/renderer/src/components/emulator-pane/emulator-pane-runtime-target.ts`:
```ts
export function resolveEmulatorPaneRuntimeTarget(worktreeId: string): {
  target: RuntimeClientTarget
  projectId: string | undefined
}
```
`projectId` lấy từ `worktree.projectId` (qua store's `getKnownWorktreeById`) — vắng mặt cho workspace kiểu repo-only cũ (trước khi có Project entity); các caller phải chấp nhận `projectId` là `undefined` (backend-go's `resolveEmulatorConnectionID` đã coi `projectId` rỗng là nhánh honest-stub sẵn có, không phải lỗi — TASK-EMU-009).

Đổi mọi lời gọi `callRuntimeRpc({kind:'local'}, 'emulator.xxx', params)` thành `callRuntimeRpc(target, 'emulator.xxx', {...params, projectId})`, giữ nguyên mọi field cũ (`worktree: worktreeId`, `x`/`y`, `points`, `name`, `orientation`, …) — thuần additive:

- `use-emulator-pane-controls.ts`: `sendTap`/`sendButton`/`sendGesture`/`sendRotate`
- `use-emulator-pane-shutdown.ts`
- `use-emulator-pane-session.ts`
- `MobileEmulatorSettingsPane.tsx`'s `emulator.availability` (không kèm `projectId` — đây là kiểm tra khả dụng ở mức host/SDK, không gắn với 1 project cụ thể; `emulator.availability` phía backend-go luôn relay kể cả không có projectId, trả `available:false` trung thực — TASK-EMU-009)

## Verify (chạy thật, có bằng chứng — chung với TASK-EMU-012, xem file đó)

`tsc --noEmit`: 113 lỗi, khớp baseline, zero lỗi mới trong các file đã sửa. `vitest run`: 25 file fail / 131 test fail, khớp baseline. `vite build`: thành công.

## Chưa làm (ngoài phạm vi frontend này)

Xác nhận end-to-end thật với `infra-fleet-service` chạy local (docker-compose) — cần TASK-EMU-006 (transport thật của `emulator/`) hoàn thiện phần đăng ký `AgentKind` qua kết nối WS thật, và cần backend-go's `devServer.list`/`add` wscompat nối field `kind` (xem TASK-EMU-012's ghi chú "no-op an toàn"). Chưa có môi trường có DB/backend-go chạy thật trong sandbox này để verify.
