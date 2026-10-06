# FE-CV-TASK-085-02: View-model thuần của khối cảnh báo cổng

**From Solution:** [FE-CV-SOL-085-source-control-quality-notice](../solutions/FE-CV-SOL-085-source-control-quality-notice.md) mục 2.2, 2.4
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/right-sidebar/source-control-quality-gate-view-model.ts` (mới) + `.test.ts`
**Depends on:** FE-CV-TASK-085-01; kiểu `QualityGate` từ FE-CV-SOL-050-types-and-runtime-bridge
**Status:** [ ] TODO

## Context

- Hợp đồng 4.7 / PQ-34: `reasons[] = {check, observed, threshold, result, code, params, ...}`; `unknown` ưu tiên hơn `warn`.
- CR-085 quyết định 8: `pass` không hiện khối.

## Việc cần làm

1. Cài `buildQualityNoticeViewModel` theo bảng 2.4: pass/hidden, warn, fail, unknown, timeout/offline → unknown với `unavailable`, `stale`.
2. Ánh xạ `code` → khoá i18n `auto.components.right.sidebar.qualityGateNotice.reason.<code>`; mã lạ → `.reason.unknown` kèm `check` nguyên văn (văn bản thuần).
3. Cắt còn 3 lý do đầu, giữ `reasonCount` đầy đủ.
4. Không có nhánh nào cho ra ngôn ngữ "đạt/an toàn".

## Kiểm thử

- Bảng ca đủ tổ hợp; `unknown` không bao giờ thành hidden; `mode:"block"` không đổi kết quả.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Hàm thuần, không import React/store.
- [ ] Không chuỗi hiển thị cứng (chỉ khoá).

## Rủi ro

- Mã lý do chưa đủ (backend có thể thêm).
