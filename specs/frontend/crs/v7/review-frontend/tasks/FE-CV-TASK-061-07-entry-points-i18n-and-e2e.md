# FE-CV-TASK-061-07: i18n 5 locale và e2e web cho điểm vào Review

**From Solution:** [FE-CV-SOL-061](../solutions/FE-CV-SOL-061-review-entry-points.md) mục 2.7, 6
**Priority:** P1
**Area:** frontend / i18n + tests
**File:** `i18n/locales/{en,es,ja,ko,zh}.json`; `i18n/code-intel-locale-coverage.test.ts` (thêm `KEYS`); `tests/e2e/code-intel-web/review-summary.web.e2e.ts`
**Depends on:** FE-CV-TASK-061-03..061-06; 073-02, 073-03
**Status:** [ ] TODO

## Context

- Khoá `auto.components.reviewMap.Entry*|ReviewSummaryPanel*|QuickActions*`.

## Việc cần làm

1. Dịch khoá sang 4 locale; thêm `KEYS`.
2. e2e (fake backend): cờ tắt ⇒ không lối vào; cờ bật + agent giả `done` ⇒ nút Review ⇒ tab `review` mở lens Ảnh hưởng; tab sidebar hiện tóm tắt; Cmd+K liệt kê hành động; `backend.streamCount()==0` khi cờ tắt.

## Kiểm thử

- `pnpm --filter orca-frontend test -- src/renderer/src/i18n`; e2e **chưa chạy**.

## Tiêu chí hoàn thành

- [ ] 5 locale đủ khoá.
- [ ] e2e xanh trên fake backend.

## Rủi ro

- Agent giả cần phát trạng thái hook; phụ thuộc cách web nạp `agent-status` (chưa kiểm chứng).
