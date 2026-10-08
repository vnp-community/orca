# FE-CV-TASK-087-08: Fake backend `quality.*`, phủ khoá i18n, e2e

**From Solution:** [FE-CV-SOL-087-quality-scorecard-and-state](../solutions/FE-CV-SOL-087-quality-scorecard-and-state.md) mục 4, 5
**Priority:** P1
**Area:** frontend / test-support + i18n
**File:** `frontend/src/renderer/src/test-support/code-intel-fake-backend.ts` (CR-050; sửa: kịch bản quality), `i18n/code-intel-quality-locale-coverage.test.ts` (từ 088-09; thêm khoá), `i18n/locales/{en,es,ja,ko,zh}.json` (khoá `auto.components.reviewQuality.*`, `auto.hooks.codeIntelQuality.*`), `tests/e2e/quality-gate-scorecard.spec.ts` (mới, kế hoạch)
**Depends on:** 087-01..087-07, 088-09
**Status:** [x] DONE (verified 2026-10-08)

## Context

- Fake backend G4 là cổng để frontend không đợi backend thật (hợp đồng §7.1).
- Khoá "read-by-name" phải có tay trong 5 locale (mẫu `task-jira-link-locale-coverage.test.ts`).
- `tests/playwright.web.config.ts` tồn tại; chưa xác nhận chạy được trong CI.

## Việc cần làm

1. Kịch bản fake: gate pass/warn/fail/unknown; run `percent:null`; mất push (chỉ polling); `interrupted`; `ENV_NOT_READY` (`missing[]`), `PROFILE_UNKNOWN`, `RUN_IN_PROGRESS`, `RATE_LIMITED`, `QUALITY_GATE_DISABLED`, `NOT_AUTHORIZED`; `comparison` đủ 9 relation; `dirty`; run `failed` step.
2. Thêm khoá i18n của 087-03..07 vào test phủ và 5 locale.
3. e2e (khi chạy được): chạy profile fake, thấy cổng, huỷ giữa chừng, tắt cờ ẩn lens.

## Kiểm thử

Chạy `pnpm --filter orca-frontend test -- src/renderer/src/i18n/code-intel-quality-locale-coverage`; e2e theo hạ tầng web (kế hoạch).

## Tiêu chí hoàn thành

- [ ] 5 locale đủ khoá; fake backend phủ mọi mã lỗi ở 087-06.
- [ ] Không hex/dependency mới.

## Rủi ro

- Bản dịch cần người duyệt; fake backend thuộc CR-050 nên cần phối hợp merge.

## Ghi chú triển khai (2026-10-07)

- Không sửa lõi `code-intel-fake-backend.ts` (đã có `setChannelData/setHandler/failNext/pushQuality`); kịch bản nằm ở `test-support/code-intel-quality-scenarios.ts` (+ `createFakeQualityCall` đưa fake vào slice thật).
- i18n: nhóm `auto.components.reviewQuality.scorecard.*` (130 khoá) + `auto.components.reviewMap.lens.quality.label`; test riêng `i18n/quality-scorecard-locale-coverage.test.ts` (dùng `test-support/quality-copy-locale-assertions.ts`) thay vì thêm vào `code-intel-quality-locale-coverage.test.ts`.
- Bản dịch es/ja/ko/zh do máy soạn, cần người duyệt.
