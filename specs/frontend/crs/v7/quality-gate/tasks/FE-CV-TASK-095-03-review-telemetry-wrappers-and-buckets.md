# FE-CV-TASK-095-03: Hàm bọc và hàm chia khoảng

**From Solution:** [FE-CV-SOL-095-review-telemetry](../solutions/FE-CV-SOL-095-review-telemetry.md) mục 2.3
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/lib/review-telemetry.ts` (mới) + test
**Depends on:** FE-CV-TASK-095-01
**Status:** [ ] TODO

## Context

- Mẫu `lib/mcp-telemetry.ts`; `track()` fire-and-forget, không ném.
- Web: no-op.

## Việc cần làm

1. Tám hàm `track*`; `bucketCount`, `bucketLarge`, `bucketLatencyMs`, `bucketDwellMs`, `toToolBucket`.
2. Không import kiểu có trường định danh.

## Kiểm thử

- Biên (0,1,3,10,11,30,31; 59 999/60 000 ms); `toToolBucket` lạ → `other`; mock `track`.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không hàm nào trả giá trị ngoài enum.

## Rủi ro

- Ánh xạ `tool` cần cập nhật khi thêm công cụ.
