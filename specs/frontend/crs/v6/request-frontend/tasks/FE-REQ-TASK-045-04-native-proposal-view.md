# FE-REQ-TASK-045-04: Chế độ "Proposal": đề xuất Plan native dạng cấu trúc, chỉ đọc

**From Solution:** [FE-REQ-SOL-045](../solutions/FE-REQ-SOL-045-plan-documents-and-revision-ui.md)
**Priority:** P1
**Area:** frontend / request / plan
**File:** `renderer/src/components/request/plan/documents/PlanProposalView.tsx` (+ test, mới), `renderer/src/components/request/plan/documents/plan-proposal-model.ts` (+ test, mới), `renderer/src/components/request/plan/{PlanSummaryHeader,RequestPlanTab}.tsx` (sửa)
**Depends on:** FE-REQ-TASK-045-02, FE-REQ-TASK-045-03 (cùng chỗ sửa `PlanSummaryHeader`)
**Status:** [x] DONE (2026-10-20)

---

## Context

- Đề xuất native là JSON `PlanProposal` (`plan.proto`): `title`, `summary`, `notes`, `phases[]` (`title`, `description`, `tasks[]`, `depends_on_phase_indices`), `tasks[]` khi không có Phase; `TaskProposal`: `title`, `description`, `task_type`, `estimated_hours`, `prompt_template`, `depends_on_indices`, `labels`, `irreversible`. Đã được làm sạch bí mật bởi `EncodePlanProposal`.
- Cây Tree hiện có hiển thị tiêu đề, trạng thái, ước lượng; `summary`, `notes` và mô tả đầy đủ chỉ thấy khi mở từng task.
- Dữ liệu từ `usePlanProposalOfRun(requestId, {runId})` (task 02); `runId` của bản hiện hành lấy từ `usePlanRevisions().current.runId`; với gateway cũ (không có `planRevisions`) dùng `request.planProposal` không `runId` (lượt mới nhất).
- Chỉ dùng khi `request.engine !== 'openspec'`.

## Việc cần làm

1. `plan-proposal-model.ts`: `toProposalOutline(proposal)` biến JSON dây thành cây hiển thị (Phase → task hoặc task phẳng) với chỉ số hiển thị 1-based, phụ thuộc "depends on #2, #3" (trong cùng Phase hoặc Plan), cờ `irreversible`, nhãn; chống chỉ số lệch (bỏ phụ thuộc không hợp lệ thay vì lỗi); giờ ước lượng định dạng.
2. `PlanProposalView`: tiêu đề, `summary` và `notes` bằng `RequestMarkdownContent`; mỗi Phase là một mục thu gọn mở sẵn; mỗi task: tiêu đề, loại, ước lượng giờ, huy hiệu "Irreversible" (chữ + màu token), nhãn, "Depends on …", mô tả Markdown; `prompt_template` trong `<details>` "Agent prompt" (văn bản thuần, `<pre>`). Không có nút sửa/chạy.
3. Trạng thái: tải (skeleton), lỗi (Try again), `running` ("A new version is being written…" khi lượt chỉnh chạy và bản đang xem là bản cũ hơn), lượt `failed` (hiện đề xuất của bản `current` nếu có), rỗng.
4. `PlanSummaryHeader`/`RequestPlanTab`: mục "Proposal" (thay cho "Documents" khi native) hiện khi Request đã có Plan; dùng cùng `PlanDisplayView = 'tree'|'graph'|'documents'` với nhãn khác theo engine (một khoá i18n cho mỗi nhãn).
5. Trang chủ đề `Plan` (Tree) không đổi.

## Kiểm thử

- Mô hình: có/không Phase, phụ thuộc hợp lệ/không hợp lệ, `irreversible`, ước lượng, đề xuất rỗng.
- Component: render đủ trường, `notes` Markdown, `<details>` prompt, trạng thái tải/lỗi/đang viết bản mới.
- `RequestPlanTab`: native có "Proposal", không có "Documents"; OpenSpec ngược lại.
- `pnpm vitest run`, kiểu, lint.

## Tiêu chí hoàn thành

- [x] Người dùng đọc được `summary`, `notes`, từng Phase và task (mô tả, loại, nhãn, phụ thuộc, ước lượng, cờ không đảo ngược) của Plan native. Bằng chứng: `PlanProposalView.test.tsx` ("shows the summary, the notes, every phase and every task field…", "marks an irreversible task…"); `plan-proposal-model.test.ts`.
- [x] Chỉ đọc, không HTML thô; chỉ số phụ thuộc lệch không làm hỏng trang. Bằng chứng: `PlanProposalView.test.tsx` ("has no button that edits or runs anything", "drops raw HTML…"); `plan-proposal-model.test.ts` (chỉ số ngoài khoảng/chính nó/trùng bị bỏ).
- [x] Hoạt động với gateway cũ (không `planRevisions`). Bằng chứng: `PlanProposalView.test.tsx` ("on a gateway without versions it reads the latest run…"); `RequestPlanTab.revision.test.tsx` (tab không gọi `request.planRevisions` khi thiếu cờ).

## Rủi ro và lưu ý

- Chạy `gitnexus_impact` cho `PlanSummaryHeader` (dùng ở `RequestPlanTab` và đồ thị).
- Đề xuất của lượt mới nhất có thể khác Plan đã commit (khoảng giữa lượt thành công và commit): ghi chú "Not committed yet" nếu `request.status === 'planning'` và `planTaskId` khác bản đề xuất; kiểm khi triển khai.

## Ghi chú triển khai (2026-10-20)

- **File mới**: `plan/documents/plan-proposal-model.ts` (`toProposalOutline`, `formatEstimatedHours`, `formatDependencies`), `plan/documents/PlanProposalView.tsx` (+ test cả hai). `PlanSummaryHeader` và `RequestPlanTab` đã sửa ở task 03 (mục "Proposal" cùng `PlanDisplayView`).
- **Lệnh**: `vitest run` hai file test: 18 test xanh (cùng nhóm task 03: 127). `tsc` mốc 113; `oxlint` sạch.
- **Quyết định**: chỉ số hiển thị 1-based theo từng danh sách (Phase hoặc Plan); phụ thuộc ngoài khoảng/chính nó/trùng bị bỏ thay vì lỗi; phase mở sẵn bằng `Collapsible`; `prompt_template` trong `<details>` (đóng) với `<pre>` văn bản thuần; summary/notes/mô tả qua `RequestMarkdownContent gfm`. Bản đọc theo `runId` + `revision` của bản đang xem (không dùng "lượt mới nhất" khi có danh sách phiên bản, vì nó rỗng lúc đang chỉnh). Ghi chú "Not committed yet": chỉ hiện khi không có danh sách phiên bản và Request ở `planning` ("This is the latest proposal. It is not saved as the plan yet."), vì dây không có trường nào để so với `planTaskId`.
- **Blast radius**: file mới; chỉ `RequestPlanTab` (qua `PlanDocumentsPane`) dùng.
- **Chưa kiểm**: đề xuất thật trên stack và ảnh chụp (task 08, spec `plan-documents` ca (a)).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
