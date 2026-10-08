# FE-CV-TASK-087-20: Fake backend trend/coverage/hotspot/structure, i18n, e2e

**From Solution:** [FE-CV-SOL-087-quality-trend-coverage-hotspot](../solutions/FE-CV-SOL-087-quality-trend-coverage-hotspot.md) mục 4, 5
**Priority:** P1
**Area:** frontend / test-support + i18n
**File:** `test-support/code-intel-fake-backend.ts` (sửa), `i18n/code-intel-quality-locale-coverage.test.ts` + `i18n/locales/{en,es,ja,ko,zh}.json` (khoá `auto.components.reviewQuality.{coverage,trend,hotspot,dependency}.*`), `tests/e2e/quality-charts-blocks.spec.ts` (mới, kế hoạch)
**Depends on:** 087-15..087-19, 087-08
**Status:** [x] DONE (verified 2026-10-07: 16 test pass — i18n/quality-visualization-locale-coverage.test.ts 11, test-support/code-intel-quality-visualization-fake-data.test.ts 5; e2e chỉ có kế hoạch, chưa viết spec)

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

## Ghi chú triển khai (2026-10-07)

- **Fake backend không bị sửa:** `test-support/code-intel-fake-backend.ts` đã có `setHandler`, nên kịch bản nằm trọn trong file mới `test-support/code-intel-quality-visualization-fake-data.ts` (`registerQualityVisualizationScenario(backend, {trend, coverage, hotspot, structure})`; mỗi nguồn có state `loading|empty|error|stale|ready`, lỗi `timeout-in-progress` (CODEINTEL_TIMEOUT `{inProgress}`), `output-too-large`, generic; `stale` = lần đầu thành công, sau đó lỗi; trend 1/2/50/200 qua `points`; coverage `measured|estimated|partial`; structure `dag|cycle|large` (large = envelope truncated, totalCount 400); `report:null` + reason ở state `empty`). Số liệu coverage theo D5 (tỉ lệ 0..1). Findings handler chỉ trả hotspot khi `rules` chứa `hotspot.file`, còn lại trả rỗng (thay thế handler findings khi đăng ký).
- **i18n:** một nhóm duy nhất `auto.components.reviewQuality.visualization.*` (73 khoá, lồng theo dấu chấm) trong `quality-visualization-copy.ts` + export `qv`, đủ 5 locale; test phủ ở `i18n/quality-visualization-locale-coverage.test.ts` (thay cho đường dẫn `code-intel-quality-locale-coverage.test.ts` trong task, file đó thuộc chart). Khác kế hoạch ban đầu: không tách `{coverage,trend,hotspot,dependency}` thành nhóm riêng.
- **e2e (kế hoạch, chưa viết `tests/e2e/quality-charts-blocks.spec.ts`):** (1) bật `qualityGateEnabled`, mở lens Quality; (2) mở lần lượt bốn khối, mỗi khối với kịch bản `ready`, xác nhận chỉ khi mở mới có request `quality.trend/quality.coverage/findings/structure`; (3) kịch bản coverage `empty` -> thấy "Reason: ..." và không có 0%; (4) kịch bản `error` timeout/too-large -> thông báo inline + "Retry" gọi lại; (5) bấm ô treemap / khoảng dòng chưa phủ -> diff mở đúng tệp/dòng; (6) trend 50 điểm "Showing 50/200", chuyển Turn/Commit; (7) DSM `cycle` có khối viền + ▲, bấm ô liệt kê cạnh. Hạ tầng e2e web chưa xác nhận.
