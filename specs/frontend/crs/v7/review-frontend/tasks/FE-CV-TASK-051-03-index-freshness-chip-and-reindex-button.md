# FE-CV-TASK-051-03: `IndexFreshnessChip` (9 `overall`) và `ReindexButton`

**From Solution:** [FE-CV-SOL-051-review-workspace-shell](../solutions/FE-CV-SOL-051-review-workspace-shell.md) mục 4.3
**Priority:** P0
**Area:** frontend / review-map
**File:** `components/review-map/IndexFreshnessChip.tsx`, `ReindexButton.tsx` (mới), tests
**Depends on:** FE-CV-TASK-050-14, FE-CV-TASK-051-02
**Status:** [x] DONE

## Context

- `IndexStatus.overall` (PQ-08), `tools[]`, `indexBasis[]`, `activeJob {id,stage,percent|null}`; reindex `incremental|full`, cooldown `CODEINTEL_REINDEX_COOLDOWN`.
- `useNow` (`components/dashboard/useNow.ts`) cho nhãn tương đối; `ui/popover`, `ui/progress`.

## Việc cần làm

1. Chip theo bảng SOL 4.3 (icon `lucide-react` + chữ, không chỉ màu); `Intl.RelativeTimeFormat` cập nhật mỗi phút.
2. Popover chi tiết công cụ, `indexBasis`, nút "Làm mới index" và "Lập lại toàn bộ" (xác nhận bằng `useConfirmationDialog`).
3. `ReindexButton`: khoá ngay; theo thang thời lượng; `Progress` không xác định khi `percent===null`; cooldown đếm ngược; giai đoạn lạ ⇒ hiển thị nguyên `message`; lỗi inline.
4. `OVERLAY`: câu giải thích "Dựa trên index của checkout chính và diff; số dòng có thể lệch".

## Kiểm thử

- Mỗi `overall`; `percent:null`; cooldown; `IN_PROGRESS`; reduced-motion tĩnh; hai lần bấm = một lời gọi.

## Tiêu chí hoàn thành

- [ ] 9 trạng thái đúng; không "0%" giả; không toast.

## Rủi ro

- Tập `stage` chưa biết.
