# FE-CV-TASK-059-04: Lens Hợp đồng: bảng trước/sau, huy hiệu tương thích, chi tiết, nhóm migration

**From Solution:** [FE-CV-SOL-059](../solutions/FE-CV-SOL-059-contract-lens-and-findings.md) mục 2.2
**Priority:** P1
**Area:** frontend / renderer components
**File:** `frontend/src/renderer/src/components/review-map/contract/{ContractDiffLens,ContractChangeTable,ContractSignatureCell,ContractCompatibilityBadge,ContractChangeDetail,ContractMigrationGroup}.tsx` (mới) + test; `review-lens-registry.ts` (SOL-051, thêm `id:'contract'`)
**Depends on:** FE-CV-TASK-059-01, 059-03; FE-CV-SOL-051-review-workspace-shell; FE-CV-SOL-053-impact-lens-and-symbol-detail; FE-CV-SOL-057-erd-lens (action ERD)
**Status:** [ ] TODO

## Context

- `ui/table`, `ui/collapsible`, `ui/badge`, `ui/select`, `ui/toggle-group`; ảo hoá bằng `@tanstack/react-virtual` khi > 100 dòng. Màu không phải kênh duy nhất. `unknown` không dùng biểu tượng `pass`/`Check`.

## Việc cần làm

1. `ContractDiffLens`: bộ lọc (`kinds`, service, "Chỉ phá vỡ", "Chỉ do thay đổi này", tìm kiếm), chip tóm tắt là bộ lọc, nhóm `service → kind`, trạng thái (rỗng có ghi chú loại nguồn đã quét; `truncated` "Đang hiển thị X / Y"; lỗi inline + "Thử lại").
2. `ContractChangeTable` 4 cột (Hợp đồng/Trước/Sau/Tương thích); `ContractSignatureCell` mono + tô token khác nhau bằng token CSS (không Monaco, không hex); nếu không có `before|after` thì hiện `change` + `details`.
3. `ContractCompatibilityBadge`: `breaking` (`destructive` + `ShieldAlert`), `risky` (`AlertTriangle`), `compatible` (`Check`), `unknown` (`HelpCircle` + "Chưa xác định"); `ruleId` lạ nguyên văn.
4. `ContractChangeDetail` (cột phải): `ruleId` dịch theo bảng, `details`, `files[]` ("Xem diff" đúng dòng nếu SOL-053 hỗ trợ, nếu không mở tệp và nêu giới hạn), `evidence[]`, `consumers[]` (rỗng + `breaking` ⇒ "Chưa tìm thấy nơi dùng…").
5. `ContractMigrationGroup`: `statements[]`, `tables[]` + "Mở trong ERD" (`setReviewLens`→`erd` + `setErdService` + `selectErdTable`), `findings[]` bằng `FindingRow`.

## Kiểm thử

- `ContractChangeTable` (nhãn chữ cho `breaking|risky|unknown`), `ContractCompatibilityBadge`, `ContractChangeDetail` (consumers rỗng; nút ERD chỉ khi có bảng), `ContractDiffLens` (rỗng/`truncated`/lỗi + thử lại), chuỗi chứa HTML hiển thị như văn bản.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/contract`.

## Tiêu chí hoàn thành

- [ ] Các tiêu chí 1–4 của SOL-059 mục 5.
- [ ] Không hex; không thêm thư viện; không `max-lines` disable.

## Rủi ro

- Nhảy dòng diff phụ thuộc SOL-053; chưa có thì mở tệp.
