# FE-REQ-TASK-045-02: Hook dữ liệu: danh sách phiên bản, tải từng file tài liệu, đề xuất theo lượt

**From Solution:** [FE-REQ-SOL-045](../solutions/FE-REQ-SOL-045-plan-documents-and-revision-ui.md)
**Priority:** P1
**Area:** frontend / request / plan
**File:** `renderer/src/hooks/{usePlanRevisions,usePlanBundleFile,usePlanProposalOfRun}.ts` (+ test, mới), `renderer/src/components/request/plan/documents/plan-document-model.ts` (+ test, mới)
**Depends on:** FE-REQ-TASK-045-01
**Status:** [x] DONE (2026-10-20)

---

## Context

- Mẫu làm mới theo sự kiện: `useAnalysisRevision.ts` (`useRequestSubscription`, `PROGRESS_EVENT_RE`, `REQUEST_POLL_EVENT_TYPE`); mẫu tải danh sách: `hooks/useSolutions.ts`; gọi RPC: `callRequestRpc` (`runtime/request-rpc-client.ts`).
- Backend: `request.planRevisions` phân trang mới nhất trước; `request.planBundleFile {id, path, revision?}` trả `{path, content, omitted}` hoặc lỗi `REQUEST_PLAN_REVISION_TEXT_PRUNED`, `REQUEST_PLAN_BUNDLE_FILE_NOT_FOUND`, `REQUEST_PLAN_BUNDLE_NOT_FOUND`; `request.planProposal {id, runId?, revision?}`.
- Bộ tài liệu tới 1 MiB: không tải cả bộ.

## Việc cần làm

1. `usePlanRevisions(request)`: tải trang đầu (20), `loadMore()` tải trang kế bằng `nextPageToken`; làm mới khi `request.version`, `request.status` đổi hoặc nhận sự kiện khớp `/(?:^|\.)(?:plan\.[a-z_]+|approval\.[a-z_]+|request\.status_changed)$/` (và khung thăm dò `REQUEST_POLL_EVENT_TYPE` khi đang chỉnh); trả `{revisions, current, hasMore, loadMore, isLoading, error, supported}`; `supported=false` khi cờ `request.planRevisions` thiếu hoặc RPC trả `Unimplemented`/không tồn tại (đừng coi là lỗi người dùng). Không gọi khi Request không có Plan (`planTaskId` rỗng và chưa từng có lượt).
2. `usePlanBundleFile(requestId, path, revision)`: gọi `PLAN_BUNDLE_FILE`; nhớ kết quả theo khoá `requestId|revision|path` trong `useRef<Map>` (bỏ khi `requestId` đổi; không nhớ lỗi), huỷ bằng cờ khi rời; trạng thái `idle|loading|ready|error|omitted|pruned|notFound`; `retry()`. `revision` bỏ trống/0 = hiện hành; **không** nhớ bản hiện hành qua một lần chỉnh: khoá của bản hiện hành gồm `tasksMdDigest` (hoặc `commit`) lấy từ `PlanBundleView` để làm mới khi bản đổi.
3. `usePlanProposalOfRun(requestId, {runId, revision})`: gọi `PLAN_PROPOSAL` một lần mỗi cặp; trả `{view, isLoading, error}` với `view` là `PlanProposalView` đã parse; khi `status === 'running'` báo `running` (không thăm dò ở đây; việc thăm dò lượt chỉnh thuộc task 05).
4. `plan-document-model.ts`: `orderDocuments(files: PlanBundleFileEntry[]): DocumentTab[]` — thứ tự `proposal.md`, `design.md`, các `specs/<capability>/spec.md` (sắp theo tên), `tasks.md`, phần còn lại; `DocumentTab{path, label, group: 'proposal'|'design'|'spec'|'tasks'|'other', omitted, size, change}`; nhãn spec = `<capability>` (đoạn giữa của đường dẫn); đường dẫn lạ (có `..`, tuyệt đối) bị bỏ; `isOmittedPlaceholder(content)` để dự phòng khi `omitted` thiếu; `formatBytes`.
5. Không gọi RPC trong render; không `setState` sau khi huỷ; không rò rỉ listener.

## Kiểm thử

- `usePlanRevisions`: trang đầu, `loadMore`, làm mới theo sự kiện, `supported=false`, lỗi mạng giữ danh sách cũ.
- `usePlanBundleFile`: tải, nhớ, bản khác nhau nhớ riêng, lỗi không nhớ, `pruned`, `omitted`, `notFound`, đổi Request thì bỏ nhớ, huỷ.
- `usePlanProposalOfRun`: `running`, `failed`, thành công, lỗi.
- `orderDocuments`: thứ tự chuẩn, nhiều spec, thiếu `design.md`, file lạ, đường dẫn độc hại.
- `pnpm vitest run` các file đã đổi.

## Tiêu chí hoàn thành

- [x] Mỗi tài liệu chỉ tải khi mở, được nhớ trong phiên. Bằng chứng: `usePlanBundleFile.test.ts` (tải một lần, tab mở lại không gọi lại, bản khác nhớ riêng, lỗi không nhớ, đổi Request bỏ nhớ, huỷ); `PlanDocumentsView.test.tsx` ("reads the open document only, once…").
- [x] Danh sách phiên bản phân trang và làm mới theo sự kiện. Bằng chứng: `usePlanRevisions.test.ts` (trang đầu 20, `loadMore` theo token, giữ trang cũ khi làm mới, sự kiện `plan.*` và khung thăm dò khi `planning`, gộp loạt kích hoạt thành một lần gọi đang bay).
- [x] Gateway cũ (`supported=false`) không gây lỗi. Bằng chứng: `usePlanRevisions.test.ts` (không cờ: không gọi kênh; kênh `unsupported`: `supported=false`, `error=null`); `PlanDocumentsView.test.tsx` ("works on a gateway without versions").
- [x] Thứ tự và nhãn tài liệu đúng cho cấu trúc OpenSpec chuẩn. Bằng chứng: `plan-document-model.test.ts` (thứ tự chuẩn, nhiều spec, thiếu design, file lạ, đường dẫn độc hại, trùng).

## Rủi ro và lưu ý

- Làm mới khi `request.version` đổi có thể kích nhiều lần liên tiếp: gộp bằng một lần gọi đang bay (`inFlight` ref).
- `commit` có thể rỗng ở gateway cũ: khoá dự phòng `tasksMdDigest`, rồi `generatedAt` của bản hiện hành.

## Ghi chú triển khai (2026-10-20)

- **File mới**: `hooks/usePlanRevisions.ts`, `hooks/usePlanBundleFile.ts`, `hooks/usePlanProposalOfRun.ts`, `components/request/plan/documents/plan-document-model.ts` (+ test cho từng file).
- **Lệnh**: `vitest run` trên bốn file test: 4 file, 52 test xanh; `tsc` mốc 113; `oxlint` sạch.
- **Thiết kế**: `usePlanRevisions` đọc trang đầu qua một hàm có cờ `inFlight`/`again` (một loạt kích hoạt cùng lúc gây đúng một lần gọi đang bay và tối đa một lần nữa), gộp trang đã tải thêm khi làm mới (`pagedBack`), và chỉ gọi khi có `planTaskId` và cờ `request.planRevisions`; lỗi mạng giữ danh sách cũ. `usePlanBundleFile(requestId, path, revision, currentKey)`: thêm tham số thứ tư `currentKey` (`tasksMdDigest` hoặc `commit` của bản hiện hành) để bản hiện hành không bị nhớ qua một lần chỉnh; `Map` trong `useRef`, đặt lại khi đổi Request, trạng thái `idle|loading|ready|error|omitted|pruned|notFound`, `retry()`. `usePlanProposalOfRun(requestId, {runId, revision, refreshKey}, enabled)`: một lần gọi mỗi cặp; làm mới cùng bản thì giữ câu trả lời cũ, bản khác thì không bao giờ hiện câu trả lời của bản này (tránh hiện nhầm bản hiện hành dưới banner "older version"). `running` được báo, không thăm dò.
- **Sai lệch**: hook `usePlanProposalOfRun` nhận thêm `refreshKey` và `enabled`; khoá cache của bản hiện hành nằm ở hook thứ hai (task nói cùng ý). `orderDocuments` thêm lọc `isSafeDocumentPath` (tuyệt đối, `..`, `\`, NUL, `//`) và bỏ trùng. Nhãn tab fixed ("Proposal/Design/Tasks") qua `translate` với khoá chữ nghĩa đầy đủ.
- **Blast radius**: chỉ file mới, chưa có chỗ dùng cũ.
- **Chưa kiểm**: hành vi với gateway thật (task 08).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
