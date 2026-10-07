# FE-CV-TASK-050-14: `useCodeIntelIndexStatus` và `useCodeIntelReindex`

**From Solution:** [FE-CV-SOL-050-store-and-query-hooks](../solutions/FE-CV-SOL-050-store-and-query-hooks.md) mục 4.5
**Priority:** P0
**Area:** frontend / hooks
**File:** `hooks/useCodeIntelIndexStatus.ts`, `hooks/useCodeIntelReindex.ts` (mới), tests
**Depends on:** FE-CV-TASK-050-10, FE-CV-TASK-050-11, FE-CV-TASK-050-13
**Status:** [x] DONE

## Context

- `status` trả `IndexStatus` đơn (không phong bì), `refresh?`; `reindex {mode}` ⇒ `{jobId,status,mode,trigger}`; `reindexStatus {jobId}`; lỗi `REINDEX_IN_PROGRESS {jobId,stage?}`, `REINDEX_COOLDOWN {retryAfterSeconds}` (PQ-16, §2.3).

## Việc cần làm

1. `useCodeIntelIndexStatus`: tải khi mount (cache `loadedAt`), `refresh({force})` gọi `status {refresh:true}`; polling 30 s chỉ khi `codeIntelEventsState==='polling'`; trả `overall`, `tools`, `indexBasis`, `activeJob`, `scopeMismatch`, `stale` (suy từ `overall` ∈ STALE/OVERLAY và `tools[].freshness`).
2. `useCodeIntelReindex.start(mode='incremental')`: `isStarting=true` đồng bộ; `IN_PROGRESS` ⇒ gắn job; `COOLDOWN` ⇒ `cooldownUntil`; tiến độ từ push, dự phòng `reindexStatus` mỗi 2 s khi `running`; `succeeded` ⇒ làm mới status và tăng counter; `percent:null` giữ null.
3. Không toast; lỗi trả `error` snapshot.

## Kiểm thử

- Đồng hồ giả: khoá nút ngay; hai lần bấm = một lời gọi; cooldown; in-progress; hoàn tất làm mới status.

## Tiêu chí hoàn thành

- [ ] Test xanh; không đọc `tools[]` như mảng `IndexStatus[]` cũ.

## Rủi ro

- `overall=OVERLAY` nghĩa là index checkout chính + diff (O-6): UI giải thích ở SOL-051.
