# AG-CV-TASK-080-04: Khối `host` (`platform`, `cores`, `loadavg1`, `freeMemBytes`)

**From Solution:** [AG-CV-SOL-080-index-basis-and-reindex-triggers](../solutions/AG-CV-SOL-080-index-basis-and-reindex-triggers.md) mục 5.5
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-host-snapshot.ts` (mới), `agent/src/relay/codeintel-host-snapshot.test.ts` (mới)
**Depends on:** không
**Status:** [x] DONE

## Context

Hợp đồng §4.1 `data.host` và §5.1 `quality.listProfiles.host` dùng cùng bốn trường; backend (CR-080 2.2.2) hoãn tier 2 khi `loadavg1 > 0,7 × cores`. Task nhỏ nhất của solution; `quality.listProfiles` (AG-CV-TASK-081-17) dùng lại hàm này.

## Việc cần làm

1. `export type HostSnapshot = { platform: NodeJS.Platform; cores: number; loadavg1: number; freeMemBytes: number }`.
2. `export function readHostSnapshot(deps = { os }): HostSnapshot`: `cores = os.availableParallelism()` (Node ≥ 18.14, agent target node22), `loadavg1 = os.loadavg()[0]` làm tròn 2 chữ số, `freeMemBytes = os.freemem()`.
3. Không cache (rẻ). Không đọc `/proc` trực tiếp (đa nền tảng).

## Kiểm thử

`codeintel-host-snapshot.test.ts`: giá trị hữu hạn, `cores >= 1`, `loadavg1 >= 0`; `deps` giả trả số cố định → kết quả đúng; làm tròn. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-host-snapshot.test.ts`.

## Tiêu chí hoàn thành

- [x] Hàm thuần theo `deps`; test xanh; file < 60 dòng.

## Rủi ro

- Windows: `loadavg` luôn 0 (MVP không hỗ trợ Windows, hợp đồng §1.1). Trong container (cgroup) `freemem` là của host, không phải giới hạn container: chấp nhận, ghi chú.
