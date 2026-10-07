# AG-CV-TASK-081-03: Diệt cả cây tiến trình, cả Windows (`quality-process-tree-kill.ts`)

**From Solution:** [AG-CV-SOL-081-quality-runner-core](../solutions/AG-CV-SOL-081-quality-runner-core.md) mục 5.3
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-process-tree-kill.ts` (mới), `.test.ts` (mới)
**Depends on:** không
**Status:** [x] DONE

## Context

Hợp đồng §5.4: sau ≤ 15 s cả nhóm biến mất. Đã đọc `agent-exec-handler.ts:72-88`: `killProcessTree` không xuất khẩu, POSIX chỉ kill một tiến trình, Windows dùng `exec` chuỗi. Viết mới (CR-081 2.7).

## Việc cần làm

1. `export async function killProcessTree(pid: number, opts?: { termGraceMs?: number; verifyTimeoutMs?: number; platform?: NodeJS.Platform }): Promise<{ gone: boolean; orphanSuspected: boolean }>` (mặc định 5000 / 10000).
2. POSIX: `process.kill(-pid, "SIGTERM")`; chờ `termGraceMs` (thăm dò mỗi 100 ms bằng `process.kill(-pid, 0)`); còn sống → `process.kill(-pid, "SIGKILL")`; kiểm đến `verifyTimeoutMs`: `ESRCH` = đã biến mất; không thì `orphanSuspected:true` + log (không nuốt im lặng). `ESRCH` ngay từ đầu là thành công.
3. Windows: `execFile("taskkill", ["/pid", String(pid), "/T", "/F"], { windowsHide:true })` (không shell); `platform` tiêm để test mock.
4. Điều kiện: chỉ gọi với `pid` của tiến trình đã spawn `detached:true`; từ chối `pid <= 1`.

## Kiểm thử

POSIX thật: script Node cha spawn 2 con (`sleep`-like Node), bỏ qua SIGTERM ở một con → sau kill `process.kill(-pid,0)` ném ESRCH, `gone:true`; pid không tồn tại → `gone:true`; `pid=1`/âm bị từ chối; Windows: mock `execFile` được gọi với đúng argv. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-process-tree-kill.test.ts` (ca POSIX bỏ qua trên win32 bằng `it.skipIf`).

## Tiêu chí hoàn thành

- [x] Cây con cháu biến mất ≤ 15 s; hàm idempotent.
- [x] Không có `exec` chuỗi/shell.

## Rủi ro

Tiến trình tự `setsid`/double-fork thoát khỏi nhóm (CR-081 2.7, hạn chế đã biết). macOS chưa kiểm chứng.
