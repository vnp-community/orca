# FE-CV-TASK-062-06: Allowlist mobile cho `codeIntel.reviewSummary` và test bảo vệ (ngoài `frontend/`, cần chủ sở hữu desktop duyệt)

**From Solution:** [FE-CV-SOL-062](../solutions/FE-CV-SOL-062-mobile-review-summary.md) mục 2.2
**Priority:** P2
**Area:** desktop / main runtime (**ngoài `frontend/`, cần chủ sở hữu desktop duyệt**)
**File:** `desktop/src/main/runtime/runtime-rpc.ts` (sửa: thêm vào `MOBILE_RPC_METHOD_ALLOWLIST`, dòng ~155); test allowlist (cạnh test hiện có của runtime-rpc; vị trí xác nhận khi làm); `specs/frontend/api/mobile-rpc-catalog.md` (cập nhật tài liệu)
**Depends on:** FE-CV-TASK-062-05
**Status:** [ ] TODO

## Context

- Đã xác minh: `const MOBILE_RPC_METHOD_ALLOWLIST = new Set([...])` (không export) dòng 155; kiểm tra `device.scope==='mobile' && !has(method)` ⇒ `forbidden` dòng 1086.
- U8: phiên thiết bị bị từ chối ở gateway; mobile chỉ được method host này.

## Việc cần làm

1. **`gitnexus_impact` trên `runtime-rpc.ts`** (báo blast radius).
2. Thêm `'codeIntel.reviewSummary'` (đúng một chuỗi) vào tập, giữ thứ tự chữ cái.
3. Test: method có trong allowlist; **mọi** `codeIntel.*` khác (`codeIntel.reindex`, `reviewState.save`, …) không có; thiết bị `mobile` gọi method ngoài tập ⇒ `forbidden`. Nếu tập không export, test qua đường dispatch công khai thay vì export mới (hoặc export hằng tối thiểu có chú thích "chỉ cho test").
4. Cập nhật `mobile-rpc-catalog.md` (hàng mới, ghi "unavailable tới khi O-4 chốt").

## Kiểm thử

- Test allowlist như trên; hồi quy test runtime-rpc hiện có.
- Lệnh test desktop chưa kiểm chứng.

## Tiêu chí hoàn thành

- [ ] Chỉ một method mới; test chứng minh.
- [ ] Không đổi `MOBILE_PROTOCOL_VERSION`.

## Rủi ro

- Mở allowlist sai làm lộ kênh ghi cho mobile; test bảo vệ là chốt chặn.
