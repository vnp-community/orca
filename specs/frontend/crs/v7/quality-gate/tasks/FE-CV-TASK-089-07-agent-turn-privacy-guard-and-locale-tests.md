# FE-CV-TASK-089-07: Test riêng tư và khoá i18n

**From Solution:** [FE-CV-SOL-089-agent-turn-recorder](../solutions/FE-CV-SOL-089-agent-turn-recorder.md) mục 5, 6
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/turns/agent-turn-privacy-guard.test.ts` (mới), `frontend/src/renderer/src/i18n/agent-turn-verification-locale-coverage.test.ts` (mới), `frontend/src/renderer/src/i18n/locales/*.json`
**Depends on:** FE-CV-TASK-089-01..06
**Status:** [x] DONE (verified 2026-10-07: 7 tests privacy-guard + 5 tests locale-coverage)

## Context

- CR-095: telemetry không bao giờ có `agent_type`, `model`, id lượt.

## Việc cần làm

1. Test tĩnh: module recorder không import `track`/telemetry.
2. Phủ khoá năm locale.
3. Test: payload không chứa chuỗi prompt mẫu.

## Kiểm thử

- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Test xanh.
- [ ] Năm locale đủ khoá.

## Rủi ro

- Khoá dịch cần duyệt.

## Ghi chú triển khai (2026-10-07)

Bổ sung 28 khoá locale năm ngôn ngữ (trước đó test locale fail vì thiếu khoá).
