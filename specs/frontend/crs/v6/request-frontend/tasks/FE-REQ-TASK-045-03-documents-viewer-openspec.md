# FE-REQ-TASK-045-03: Chế độ "Documents": xem proposal, design, specs delta, tasks của OpenSpec change

**From Solution:** [FE-REQ-SOL-045](../solutions/FE-REQ-SOL-045-plan-documents-and-revision-ui.md)
**Priority:** P1
**Area:** frontend / request / plan
**File:** `renderer/src/components/request/plan/documents/{PlanDocumentsView,PlanDocumentTabs,PlanDocumentBody,PlanChecksPanel}.tsx` (+ test, mới), `renderer/src/components/request/solution/RequestMarkdownContent.tsx` (sửa: tuỳ chọn `gfm`), `renderer/src/components/request/plan/{PlanSummaryHeader,RequestPlanTab}.tsx` (sửa), `renderer/src/components/request/plan/plan-view-state.ts` (kiểu `PlanDisplayView`)
**Depends on:** FE-REQ-TASK-045-02
**Status:** [x] DONE (2026-10-20)

---

## Context

- `RequestPlanTab.tsx` (249 dòng) giữ `planView: 'tree' | 'graph'`; `PlanSummaryHeader.tsx` có `ToggleGroup` Tree/Graph (`view`, `onViewChange` kiểu `'tree' | 'graph'`).
- `RequestMarkdownContent` (89 dòng): react-markdown, không HTML thô, chỉ `http(s)`, thu gọn sau 4000 ký tự bằng `Sheet`; không dùng `remark-gfm` (có trong `frontend/package.json`).
- `tasks.md` chứa task list `- [ ] 1.1 …` và HTML comment `<!-- orca:begin plan … -->`; `design.md` có thể có bảng.
- `request.engine` (`OrcaRequest.engine?: RequestEngine`) cho biết engine; chỉ openspec có bộ tài liệu.
- Dữ liệu: hook của task 02; bản đang xem do `PlanVersionSwitcher` (task 06) quyết định, ở task này mặc định bản hiện hành.

## Việc cần làm

1. `RequestMarkdownContent`: thêm prop tuỳ chọn `gfm?: boolean` (mặc định `false`, không đổi chỗ dùng cũ); khi bật, truyền `remarkPlugins={[remarkGfm]}`; checkbox của task list hiển thị `disabled` (không tương tác). Test: `gfm` bật hiện bảng và checkbox, tắt giữ như cũ; HTML thô vẫn bị bỏ; liên kết không phải `http(s)` vẫn bị vô hiệu.
2. `PlanSummaryHeader`: `view` và `onViewChange` nhận `'tree' | 'graph' | 'documents'`; thêm `ToggleGroupItem value="documents"` nhãn "Documents" chỉ khi prop mới `documentsAvailable` (true khi `request.engine === 'openspec'`); giữ tương thích với chỗ dùng khác (đồ thị `CR-REQ-032` truyền `view` không có `documents`).
3. `PlanChecksPanel`: nhận `checks`, `knowledge`, `isCurrent`; khối thu gọn ("Checks", mở sẵn khi có cảnh báo): mỗi `check` có biểu tượng và chữ theo `severity` (`info`/`warning`, luôn có chữ không chỉ màu), mã + `where` + thông điệp; khối "Reported by the authoring agent": có sẵn tri thức hay không, snapshot, số phần tử đã đọc (danh sách thu gọn), `gaps`, `conflicts`; dòng nhỏ "This is the agent's own report; Orca did not verify it."; `SECRETS_REDACTED` hiển thị như một check bình thường. Bản cũ: "Checks are shown for the current version only".
4. `PlanDocumentTabs`: nhận danh sách `DocumentTab` (task 02), tab đang chọn, `onSelect`; dùng `ToggleGroup`/`Tabs` shadcn; huy hiệu `New`/`Changed`/`Removed` (chữ, màu token trạng thái) và biểu tượng + `title` "Too large to display" cho `omitted`; mũi tên trái/phải đổi tab; nhãn truy cập đầy đủ; cuộn ngang trong chính thanh tab khi nhiều spec (không cuộn trang).
5. `PlanDocumentBody`: dùng `usePlanBundleFile`; skeleton khi tải; lỗi có nút "Try again"; `omitted`: khối "This file is too large to display ({{size}}). It is on the proposal branch `{{branch}}`." (kích thước lấy từ danh sách file, **không** từ nội dung dấu); `pruned`; `notFound`; hai chế độ "Rendered"/"Source" (`ToggleGroup`, nhớ lựa chọn trong state của view, không `localStorage`): Rendered = `RequestMarkdownContent gfm`, Source = `<pre className="font-mono text-xs …">` cuộn trong khung có chiều cao giới hạn, không `dangerouslySetInnerHTML`; nút "Copy" cho Source (nếu đã có tiện ích sao chép trong repo, dùng lại).
6. `PlanDocumentsView`: ghép dải kiểm tra, thanh tab, thân; chọn mặc định tab đầu (proposal) và nhớ tab khi chuyển phiên bản nếu đường dẫn còn tồn tại; trạng thái rỗng "This change has no documents yet." / lỗi tải danh sách.
7. `RequestPlanTab`: `planView` nhận `'documents'`; render `PlanDocumentsView` thay cho `PlanTree`/`PlanOutline`/`GraphPanel` khi chọn; tách phần mới ra file riêng để `RequestPlanTab.tsx` không vượt `max-lines`.
8. Mọi chuỗi mới qua `translate('auto.components.request.plan.documents.…', 'English text')`; khoá 5 locale ở task 07.

## Kiểm thử

- Component: tab theo thứ tự chuẩn; huy hiệu; bị lược (kích thước từ danh sách file); `pruned`; lỗi + Try again; Rendered/Source (văn bản có `<!-- orca:begin plan` hiện ở Source, không hiện ở Rendered); `gfm`; checks (mức, tri thức, cảnh báo bí mật); bản cũ.
- `PlanSummaryHeader`: mục Documents chỉ khi `documentsAvailable`; đồ thị cũ không đổi.
- `RequestPlanTab`: chuyển sang Documents; native không có mục Documents; test hiện có xanh.
- Truy cập: bàn phím trên thanh tab; `aria-selected`; tương phản huy hiệu (token).
- `pnpm vitest run` + kiểm kiểu + `pnpm lint`.

## Tiêu chí hoàn thành

- [x] Người dùng đọc được proposal, design, từng spec delta và tasks của change. Bằng chứng: `PlanDocumentsView.test.tsx` (tab đúng thứ tự, đọc khi mở, Rendered/Source, checkbox chỉ đọc); `RequestPlanTab.revision.test.tsx` ("offers Documents…") với payload dạng gateway.
- [x] File bị lược hiện placeholder với kích thước thật; không tải cả bộ tài liệu. Bằng chứng: `PlanDocumentsView.test.tsx` ("too large to display (8.6 MB) … request/7-proposal", không gọi `planBundleFile` cho file đó; chỉ một lần đọc lúc đầu). Kích thước đúng nếu backend sửa G10 (task BE-045-03): UI lấy từ danh sách file.
- [x] Kiểm tra và khoảng trống tri thức hiển thị, ghi rõ là lời của agent. Bằng chứng: `PlanChecksPanel.test.tsx` (mức bằng chữ, mở sẵn khi có cảnh báo, `SECRETS_REDACTED`, "Reported by the authoring agent", dòng "Orca did not verify it", bản cũ).
- [x] Không HTML thô; Solution hiển thị như cũ. Bằng chứng: `RequestMarkdownContent.test.tsx` (`gfm` bật: bảng, checkbox chỉ đọc, `skipHtml`, liên kết chỉ http(s); tắt: không bảng/checkbox như trước; 40 test cũ giữ xanh).
- [x] Không file nào vượt `max-lines`. Bằng chứng: `oxlint` sạch; `RequestPlanTab.tsx` 263 dòng mã (cap 300).

## Rủi ro và lưu ý

- Văn bản do agent viết: không bao giờ chèn HTML; `Source` chỉ là văn bản.
- `RequestPlanTab` đang dài: tách trước khi thêm.
- Chạy `gitnexus_impact` cho `RequestMarkdownContent` (dùng ở Solution, Answer, Findings), `PlanSummaryHeader`.

## Ghi chú triển khai (2026-10-20)

- **File mới** (`components/request/plan/documents/`): `PlanDocumentsView.tsx`, `PlanDocumentTabs.tsx`, `PlanDocumentBody.tsx`, `PlanChecksPanel.tsx`, `PlanDocumentsPane.tsx` (chọn Documents/Proposal theo engine) và test của từng phần. **Sửa**: `solution/RequestMarkdownContent.tsx` (+ test: prop `gfm`), `plan/PlanSummaryHeader.tsx` (+ `PlanSummaryHeader.test.tsx` mới), `plan/plan-view-state.ts` (`PlanDisplayView = 'tree'|'graph'|'documents'`), `plan/RequestPlanTab.tsx`.
- **Lệnh**: `vitest run` trên `plan/documents`, `RequestMarkdownContent.test.tsx`, `PlanSummaryHeader.test.tsx`: 8 file, 127 test xanh; `RequestPlanTab.revision.test.tsx` kiểm mục Documents trong tab thật. `tsc` mốc 113; `oxlint` sạch.
- **Quyết định**: (1) `react-markdown` v10 không có `rehype-raw` nên HTML thô (kể cả `<!-- orca:begin plan -->`) bị hiện thành văn bản chứ không bị bỏ như CR giả định; vì vậy `gfm` bật luôn `skipHtml` (mọi HTML thô bị bỏ, comment máy ẩn khỏi Rendered, vẫn thấy ở Source). Solution không đổi (mặc định `gfm=false`). (2) Thanh tab dùng `Tabs` shadcn (Radix: mũi tên trái/phải, `aria-selected`) trong khung cuộn ngang riêng; huy hiệu New/Changed/Removed bằng chữ + token (`status-success`, `quality-warning`, `destructive`), không hiện ở phiên bản 1 (mọi file sẽ là "New"). (3) File `omitted` hoặc `removed` hoặc bản bị lược chữ không bao giờ bị yêu cầu đọc (chỉ mục đã nói rõ). (4) Copy dùng lại `McpCopyButton` (không toast). (5) Prop của `PlanSummaryHeader` là `documentsView?: 'documents'|'proposal'` (ẩn khi vắng) thay cho `documentsAvailable` vì task 04 cần nhãn thứ hai; mục hiện khi `flowPlanKind !== 'single_task'` (hotfix không có).
- **Blast radius** (grep): `RequestMarkdownContent` dùng ở `SolutionSection`, `AnswerView`, `DiagnosisView`, `FindingsView` và hai view mới (thêm prop tuỳ chọn, không đổi chỗ cũ); `PlanSummaryHeader` chỉ `RequestPlanTab` dùng (+ test).
- **Chưa kiểm**: Rendered/Source với tài liệu OpenSpec thật và ảnh chụp (task 08, spec `plan-documents`).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
