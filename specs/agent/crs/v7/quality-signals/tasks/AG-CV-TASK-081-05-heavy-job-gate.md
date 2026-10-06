# AG-CV-TASK-081-05: Cổng việc nặng dùng chung `quality.run` và `codeintel.reindex` (`agent-heavy-job-gate.ts`)

**From Solution:** [AG-CV-SOL-081-quality-runner-core](../solutions/AG-CV-SOL-081-quality-runner-core.md) mục 5.1
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-heavy-job-gate.ts` (mới), `.test.ts` (mới); sửa 1 chỗ `agent/src/relay/codeintel-reindex-job.ts` (AG-CV-SOL-004)
**Depends on:** AG-CV-TASK-081-04 (kiểu `acquire`); phần nối reindex chặn bởi AG-CV-SOL-004
**Status:** [ ] TODO

## Context

Hợp đồng §5.2: `ORCA_HEAVY_JOBS`=1, chờ ≤ `ORCA_HEAVY_QUEUE_WAIT_MS`=10 phút rồi bước `skipped`. CR-081 2.9: semaphore trong tiến trình, chia sẻ với reindex. Khoá tệp liên tiến trình (nhiều agent cùng máy) là P1 chưa thiết kế (CR Q4), ngoài task.

## Việc cần làm

1. `createHeavyJobGate(opts?: { max?: number; now?: () => number })` đọc `ORCA_HEAVY_JOBS` (số nguyên ≥ 1, ≤ `availableParallelism()`, sai → 1 + log), trả `{ acquire(signal, waitMs): Promise<() => void>; stats(): { running: number; waiting: number } }`; hàng đợi FIFO; hết `waitMs` → reject `HeavyGateTimeoutError`; `signal.abort` → rời hàng đợi.
2. Singleton `getHeavyJobGate()` cho cả tiến trình; `release` idempotent.
3. Sửa `codeintel-reindex-job.ts`: bọc phần spawn `analyze`/`index` bằng `acquire`; `sync` nhẹ của CodeGraph không qua cổng (theo CR-081 2.9: chỉ việc nặng).
4. `quality-limits.ts` chứa `ORCA_HEAVY_QUEUE_WAIT_MS` (mặc định 600000).

## Kiểm thử

Test: max=1 hai bên tranh nhau, thứ tự FIFO, `waitMs` hết hạn, abort khi đang chờ, release gấp đôi không tăng slot, lỗi trong tác vụ vẫn release (dùng `try/finally` ở người gọi, test bằng helper); env sai bị bỏ. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/agent-heavy-job-gate.test.ts`.

## Tiêu chí hoàn thành

- [ ] Không hai bước nặng chạy đồng thời khi max=1, kể cả reindex và quality.
- [ ] Không có timer treo sau khi test kết thúc.

## Rủi ro

Hai tiến trình agent trên cùng máy không chia sẻ cổng. Nếu SOL-004 chưa có `codeintel-reindex-job.ts`, hoàn thành phần cổng trước, phần nối để `BLOCKED`.
