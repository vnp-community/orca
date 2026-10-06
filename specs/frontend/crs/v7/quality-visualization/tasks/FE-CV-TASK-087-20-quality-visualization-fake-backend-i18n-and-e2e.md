# FE-CV-TASK-087-20: Fake backend trend/coverage/hotspot/structure, i18n, e2e

**From Solution:** [FE-CV-SOL-087-quality-trend-coverage-hotspot](../solutions/FE-CV-SOL-087-quality-trend-coverage-hotspot.md) mục 4, 5
**Priority:** P1
**Area:** frontend / test-support + i18n
**File:** `test-support/code-intel-fake-backend.ts` (sửa), `i18n/code-intel-quality-locale-coverage.test.ts` + `i18n/locales/{en,es,ja,ko,zh}.json` (khoá `auto.components.reviewQuality.{coverage,trend,hotspot,dependency}.*`), `tests/e2e/quality-charts-blocks.spec.ts` (mới, kế hoạch)
**Depends on:** 087-15..087-19, 087-08
**Status:** [ ] TODO

## Context

- Mẫu phủ khoá: `task-jira-link-locale-coverage.test.ts`; fake backend G4 thuộc CR-050.
- Fixture cỡ trần từ `quality-chart-fixtures.ts` (088-09).

## Việc cần làm

1. Kịch bản fake: coverage `measured`/`estimated`/`partial`/`report:null`+`reason`; trend 1/2/50/200 điểm, `metrics` thưa, `source:'ci'`; findings hotspot với khoá `metrics` khác nhau; `structure` có vòng và truncated; lỗi `TIMEOUT {inProgress}` và `OUTPUT_TOO_LARGE`.
2. Khoá i18n đủ 5 locale vào test phủ.
3. e2e (kế hoạch): mở lens, mở từng khối, thấy trạng thái rỗng có lý do, bấm ô mở diff.

## Kiểm thử

Chạy `pnpm --filter orca-frontend test -- src/renderer/src/i18n/code-intel-quality-locale-coverage`; e2e theo hạ tầng web (chưa xác nhận).

## Tiêu chí hoàn thành

- [ ] Mọi trạng thái `loading|empty|error|stale|ready` có kịch bản.
- [ ] 5 locale đủ khoá.

## Rủi ro

- Fake backend dùng chung với CR-050; phối hợp merge.
