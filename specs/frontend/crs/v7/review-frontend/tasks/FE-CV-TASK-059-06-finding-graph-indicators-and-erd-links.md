# FE-CV-TASK-059-06: Chỉ báo phát hiện trên đồ thị và liên kết "Xem trong đồ thị"/ERD

**From Solution:** [FE-CV-SOL-059](../solutions/FE-CV-SOL-059-contract-lens-and-findings.md) mục 2.3
**Priority:** P2
**Area:** frontend / renderer (selector + điều hướng lens)
**File:** `frontend/src/renderer/src/components/review-map/findings/finding-graph-target.ts` (mới) + test; `frontend/src/renderer/src/store/slices/code-intel.ts` (selector đã có từ 059-03; chỉ nối)
**Depends on:** FE-CV-TASK-059-03, 059-05; FE-CV-SOL-053-impact-lens-and-symbol-detail; FE-CV-SOL-054-structure-lens; FE-CV-SOL-057-erd-lens
**Status:** [x] DONE

## Context

- Chỉ báo trên đồ thị (10 §6.7): SOL-059 chỉ cung cấp selector; lens khác tô icon (không sửa lens khác trong task này). Liên kết "Xem trong đồ thị" là hàm điều hướng thuần.

## Việc cần làm

1. `resolveFindingGraphTarget(finding, availableLensIds, overlay)` → `{lens, nodeKey} | null`: `layer_violation`/`dependency_cycle` → `structure`/`impact` nếu `evidence[0].symbol.key` có trong tập nút hiện hành (nhận qua tham số, không import store); `hotspot`/`dead_code` → `structure`; `missing_tenant_id`/`rls_removed` → `erd` chỉ khi `params.table` có (không đoán); khác ⇒ `null`.
2. Hàm nối `openFindingInGraph(worktreeId, target)` gọi `setReviewLens` + `setReviewSelectedSymbol` (SOL-051) hoặc `setErdService`/`selectErdTable` (SOL-057).
3. Tài liệu hoá selector `selectOpenFindingsBySymbolKey` (đã tạo ở 059-03) cho SOL-053/054/055 dùng; không sửa các lens đó ở task này.

## Kiểm thử

- `resolveFindingGraphTarget` cho từng `kind`; thiếu `params.table` ⇒ `null`; lens không khả dụng ⇒ `null`.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/findings/finding-graph-target`.

## Tiêu chí hoàn thành

- [ ] Hàm thuần có test; "Xem trong đồ thị" ẩn khi `null`.
- [ ] Không import chéo ngược từ lens khác vào `findings/`.

## Rủi ro

- Phụ thuộc `setReviewSelectedSymbol`/`setErdService` đã tồn tại (SOL-051/057); thiếu thì nút ẩn.
