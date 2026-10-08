# FE-REQ-TASK-020-02: `SolutionPanel`, banner, trạng thái sinh và các view Chẩn đoán, Findings, Answer

**From Solution:** [FE-REQ-SOL-020](../solutions/FE-REQ-SOL-020-solution-review-ui.md) mục 2.1, 2.2, 2.5
**Priority:** P0
**Area:** frontend / request / solution
**File:** `frontend/src/renderer/src/components/request/solution/{RequestAnalysisTab,SolutionPanel,SolutionVersionSwitcher,SolutionStatusBanner,SolutionBody,DiagnosisView,FindingsView,AnswerView,SolutionGenerationState,RequestMarkdownContent}.tsx` (mới); `components/request/RequestDetailPane.tsx` (sửa: cắm `RequestAnalysisTab`); test cùng tên
**Depends on:** FE-REQ-TASK-020-01, 019-03, 018-03
**Status:** [x] DONE (verified 2026-10-07: vitest components/request/solution 90 pass, tsc/oxlint sạch ở file solution/)

## Context

- Kênh: `solution.list {requestId, kind?, status?, pageSize, pageToken}`; `solution.generate {requestId, idempotencyKey?, feedback?}` (25 s). Sự kiện làm mới: `solution.proposed|approved`, `request.status_changed`.
- `react-markdown ^10.1.0` có trong `frontend/package.json`; không dùng `MarkdownPreview.tsx` của editor.
- `ui/sheet.tsx`, `ui/skeleton.tsx`, `ui/tabs.tsx` sẵn có.

## Việc cần làm

1. `RequestAnalysisTab({request})`: `useSolutions(request.id)` + `useApprovals({requestId, subjectType})`; không render nếu `REQUEST_FLOW_REGISTRY[type].analysisKind===null`.
2. `SolutionPanel`: chọn Solution hiện tại (mới nhất không `superseded`); `SolutionVersionSwitcher` liệt kê các bản, bản `superseded` thu gọn nhãn "Bản cũ".
3. `SolutionStatusBanner` theo `getSolutionPresentation`: icon lucide + chữ + token màu (`status-success` cho `approved`, `destructive` cho `rejected`); hiện `dueAt`, lý do bị từ chối, người, thời điểm.
4. `SolutionBody` rẽ theo `kind` (`solution` → `SolutionOptionCompare` do 020-03; `diagnosis` → `DiagnosisView` (nguyên nhân gốc, phạm vi ảnh hưởng, baseline đo); `findings` → `FindingsView` (câu hỏi, phát hiện, nguồn, kết luận); `answer` → `AnswerView` (câu trả lời, trích dẫn)). Trường thiếu thì "Không có dữ liệu".
5. `RequestMarkdownContent`: `react-markdown` không `rehype-raw`, `components.a` chỉ link `http:`/`https:` (còn lại render chữ thường), không `dangerouslySetInnerHTML`; nội dung > 4.000 ký tự thì thu gọn với "Xem đầy đủ" mở `Sheet`.
6. `SolutionGenerationState`: `draft` (đang soạn), lỗi sinh (nút "Sinh lại" gọi `generate` với `idempotencyKey` mới), `REQUEST_RATE_LIMITED` (khoá ngắn).
7. Trạng thái: Request `analyzing` chưa có Solution → Skeleton + "AI đang phân tích"; `network` → banner "Thử lại"; `forbidden` → "Bạn không có quyền xem phân tích"; `unsupported` không tới đây.
8. `request-event-bus`: refetch khi `requestId` khớp và `eventType` thuộc tập `solution.*`/`approval.*`.

## Kiểm thử

- Component: mỗi `kind` render đúng khung; mỗi `status` đúng banner; `superseded` thu gọn; Markdown không chạy script (`<script>` hiển thị như chữ hoặc bị bỏ) và `javascript:` không thành link; Skeleton; `network`; `forbidden`; `draft`.
- Phím tắt: không có phím cho hành động ở task này.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/solution`.

## Tiêu chí hoàn thành

- [ ] Bốn `kind` hiển thị được với dữ liệu thiếu trường.
- [ ] Không HTML thô từ nội dung AI.
- [ ] Đủ trạng thái rỗng, tải, lỗi.
- [ ] Khoá i18n `SolutionPanel.*`, `SolutionStatusBanner.*`, `DiagnosisView.*`, `FindingsView.*`, `AnswerView.*`, `SolutionGenerationState.*` đủ 5 locale.

## Rủi ro và lưu ý

- Cách lấy nội dung đầy đủ (`content_ref`) chưa có kênh: nút "Xem đầy đủ" chỉ hiện với nội dung đã có.
- Markdown rất dài làm chậm: cắt hiển thị đầu, tải đủ trong `Sheet`.

## Ghi chú triển khai (2026-10-07)

- `RequestAnalysisTab` giữ props `{request,onChanged}`; việc cắm vào `RequestDetailPane` thuộc agent 019 (không sửa ở đây).
- Trạng thái Solution theo parser hiện tại (`generating|ready|chosen|rejected|superseded`); `parseSolution` được mở rộng khoan dung để nhận `draft|proposed|approved`, `options` dạng object và `chosenOption` số (test `shared/request-solution-wire-parser.test.ts`).
- Nội dung diagnosis/findings/answer đọc từ `content` (JSON snake/camel) hoặc fallback Markdown; schema AI chưa chốt.
- `useApprovals({requestId})` ghi đè `pendingApprovalCount` toàn cục bằng số của riêng request: rủi ro cần agent hook xử lý.
- Bản dài > 4000 ký tự cắt đầu + `Sheet` xem đầy đủ.
