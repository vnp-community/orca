# FE-CV-TASK-085-01: Hook gộp cờ code-intel/quality/AI

**From Solution:** [FE-CV-SOL-085-source-control-quality-notice](../solutions/FE-CV-SOL-085-source-control-quality-notice.md) mục 2.2, 2.3 (1)
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/hooks/useQualityFeatureFlags.ts` (mới), `useQualityFeatureFlags.test.ts` (mới)
**Depends on:** FE-CV-SOL-050-store-and-query-hooks (slice settings, `useCodeIntelSupport`); fake backend G4
**Status:** [x] DONE (verified 2026-10-07: 8 tests useQualityFeatureFlags.test.ts)

## Context

- Hợp đồng 6: nguồn chuẩn là `codeIntel.settings.get` → `Settings.effective`; không dùng `typeof window.api.codeIntel` (web bọc Proxy).
- Hook này dùng lại cho 089, 090, 092, 093 (task phụ thuộc vào đây).

## Việc cần làm

1. Trả `{state, codeIntel, quality, ai}`: `codeIntel = support.state==="enabled" && effective.codeIntelEnabled`; `quality = codeIntel && effective.qualityGateEnabled`; `ai = quality && effective.aiReviewEnabled`.
2. `state:"unknown"` khi chưa có `settings.get`; khi unknown mọi cờ `false` (fail closed).
3. Không tự gọi RPC: chỉ đọc slice của 050 (nếu 050 chưa có selector, ghi giả định trong PR).

## Kiểm thử

- Bảng ca: ba cờ x unknown/ready; cờ tắt giữa chừng; đổi môi trường.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Mọi cờ `false` khi unknown.
- [ ] Không gọi `codeIntelClient` trực tiếp.
- [ ] Test xanh.

## Rủi ro

- Tên selector của 050 chưa chốt (chưa kiểm chứng).

## Ghi chú triển khai (2026-10-07)

Selector tách thành 4 selector nguyên thuỷ (zustand v5 lặp vô hạn nếu trả object mới). Hình dạng `state` giữ `unknown|enabled|disabled|unsupported` (lệch spec `unknown|ready`).
