# FE-CV-TASK-062-05: Host method desktop `codeIntel.reviewSummary` (ngoài `frontend/`, cần chủ sở hữu desktop duyệt)

**From Solution:** [FE-CV-SOL-062](../solutions/FE-CV-SOL-062-mobile-review-summary.md) mục 2.2
**Priority:** P2
**Area:** desktop / main runtime (**ngoài `frontend/`, cần chủ sở hữu desktop duyệt**)
**File:** `desktop/src/main/runtime/rpc/methods/code-intel.ts` (mới) + `code-intel.test.ts`; `desktop/src/main/runtime/rpc/methods/index.ts` (sửa: đăng ký `CODE_INTEL_METHODS`)
**Depends on:** FE-CV-TASK-062-01 (shape); O-4 cho port thật; hợp đồng UI-API §8
**Status:** [x] DONE (verified 2026-10-07: desktop vitest rpc/methods/code-intel.test.ts 12/12 PASS; oxlint clean on new files)

## Context

- Đã xác minh: `defineMethod({name, params, handler})` (mẫu `host-capabilities.ts`); `methods/index.ts`, `mobile.test.ts` tồn tại; chưa có `codeIntel.*` ở host.
- **Chạy `gitnexus_impact` trên `methods/index.ts` registry trước khi sửa; `detect_changes` trước commit.**

## Việc cần làm

1. Định nghĩa `CodeIntelSummaryPort` (giao diện): `getOverlay`, `getFindings`, `getStatus` (theo shape hợp đồng) và `NoGatewayCodeIntelPort` mặc định (trả `available:false`).
2. `defineMethod` `codeIntel.reviewSummary` với params `{worktree: string, scope?: 'branch'}` (schema chặt, chuỗi ≤ 512): ánh xạ sang `MobileReviewSummary` (≤ 50 mục, title/summary từ `titleKey`+`params` bằng bảng mẫu tiếng Anh, không `evidence`, che chuỗi tự do); ánh xạ `CODEINTEL_*` → `reason` (`DISABLED`→`flag_off`, nhóm no-binding→`no_binding`, `INDEX_MISSING`→`index_missing`, `TOOL_UNAVAILABLE`→`tool_unavailable`).
3. Đăng ký ở `methods/index.ts`. Không gọi kênh ghi nào.

## Kiểm thử

- Test với port giả (mẫu `mobile.test.ts`): ánh xạ đầy đủ, cắt 50, không `evidence`, từng mã lỗi → `reason`, port mặc định, params sai bị từ chối.
- Chạy test desktop theo cấu hình `desktop/` (lệnh chưa kiểm chứng).

## Tiêu chí hoàn thành

- [ ] Handler không phụ thuộc gateway thật.
- [ ] Không trả mã nguồn/`evidence`.

## Rủi ro

- Phụ thuộc O-4; bảng mẫu `titleKey` có thể lệch backend.

## Ghi chú triển khai (2026-10-07)

- File: `desktop/src/main/runtime/rpc/methods/{code-intel.ts, code-intel-summary-port.ts, code-intel-review-summary-mapping.ts, code-intel.test.ts}`; đăng ký ở `methods/index.ts`. Cổng mặc định ném `CODEINTEL_UNAVAILABLE` ⇒ `{available:false}` (không `reason`). Bảng mẫu `titleKey` là đề xuất, cần đồng bộ với backend. Bộ che chuỗi cục bộ (`maskReviewSummaryText`) thay cho bộ che chung (O-16). **Ngoài `frontend/`, cần chủ sở hữu desktop duyệt.** `gitnexus impact` trả `ambiguous` (2 symbol trùng tên, 0 ảnh hưởng, LOW).
