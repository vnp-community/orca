# FE-REQ-TASK-041-01: Kiểu, parser và hook `useAnalysisRevision`

**From Solution:** [FE-REQ-SOL-041](../solutions/FE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Area:** frontend / request / solution
**File:** `frontend/src/shared/{request-types,request-wire-parsers}.ts` + parser Solution, `renderer/src/components/request/solution/{useAnalysisRevision,revision-suggestions}.ts`, test
**Depends on:** Backend TASK-REQ-041-02
**Status:** [ ] TODO

---

## Context

- `Solution` chưa có `revisionFeedback`, `seq`.
- `useSolutionDecision` có `actions.generate({feedback, idempotencyKey})`, `clampFeedback`; mã lỗi đã phân loại ở `solution-error-classification.ts`.

## Việc cần làm

1. `Solution.revisionFeedback?`, `Solution.seq?`; parser chịu thiếu/sai kiểu, không ném lỗi.
2. `useAnalysisRevision` theo SOL-041 mục 2.2: kiểm rỗng/độ dài, gọi `generate` trực tiếp, `busy` theo trạng thái Request, ánh xạ lỗi, giữ nội dung khi lỗi.
3. `revision-suggestions.ts`: chip theo `SolutionKind` (solution, diagnosis → solution kind với `diagnosisFocus` không cần ở UI, findings, answer), nhãn qua `translate`.
4. Không gọi `reject`.

## Kiểm thử

- Hook: thành công; rỗng; quá dài; `FORBIDDEN` giữ nội dung; `WRONG_STATE` làm mới; `NO_CONNECTION`; `busy` kéo dài đến khi trạng thái Request đổi (dùng sự kiện giả); bấm đúp chỉ một lời gọi.
- Parser: bảng ca.

## Tiêu chí hoàn thành

- [ ] `submit` chỉ gọi `generate`, không `reject`.
- [ ] Lỗi không làm mất nội dung đã gõ.
- [ ] `busy` theo trạng thái Request.

## Rủi ro và lưu ý

- Tham gia chung `useSolutionDecision`: không đổi hành vi hiện có của nó.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
