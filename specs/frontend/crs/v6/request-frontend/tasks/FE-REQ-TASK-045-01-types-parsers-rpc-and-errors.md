# FE-REQ-TASK-045-01: Kiểu, parser dây, hằng RPC, cờ tính năng và mã lỗi của Plan revision

**From Solution:** [FE-REQ-SOL-045](../solutions/FE-REQ-SOL-045-plan-documents-and-revision-ui.md)
**Priority:** P1
**Area:** frontend / request / plan
**File:** `shared/request-plan-revision.ts` (mới), `shared/request-plan-revision-parsers.ts` (+ test, mới), `shared/request-rpc-methods.ts` (sửa), `shared/request-errors.ts` (+ test, sửa nếu cần), `renderer/src/store/slices/request.ts` (đọc cờ `requestFlowFeatures`), `renderer/src/hooks/request-plan-generation.ts` (sửa: `generatePlanPropose`), `shared/request-rpc-contract.test.ts` (cập nhật)
**Depends on:** Không (chạy thật cần BE-REQ-TASK-045-05 và 045-08)
**Status:** [x] DONE (2026-10-20)

---

## Context

- `request-rpc-methods.ts` có `GENERATE_PLAN` và `PLAN_PROPOSAL`; không có `request.planBundleFile` hay `request.planRevisions` (`grep planBundleFile|PLAN_BUNDLE_FILE` trong `frontend/src` rỗng).
- `request-plan-generation.ts`: `generatePlanProposeCommit(id, options)` chạy "propose" rồi "commit" từ tab; `ProposeValue` không có `bundle`, `checks`, `knowledge`.
- FE-REQ-SOL-044 (task 01) thêm `REQUEST_PLAN_AUTO_FEATURE = 'request.planAuto'`, `supportsPlanAuto`, `RETRY_PLAN` vào đúng các file `request-rpc-methods.ts` và `store/slices/request.ts`: hợp nhất, không ghi đè.
- Cờ tính năng của gateway: `requestFlowFeatures` trong `store/slices/request.ts`, mẫu `REQUEST_BASE_REF_FEATURE` ở `shared/request-base-ref.ts:16`.
- Backend (BE-REQ-SOL-045) thêm: `request.planRevisions {id, pageSize?, pageToken?}`, `revision` ở `request.planProposal`/`request.planBundleFile`, `idempotencyKey` ở `request.generatePlan`, `bundle.files[].sha256/change`, các mã lỗi `REQUEST_PLAN_REVISION_IN_PROGRESS|NOT_APPLICABLE|NO_CHANGE|NOT_FOUND|TEXT_PRUNED`.

## Việc cần làm

1. `request-rpc-methods.ts`: thêm `PLAN_BUNDLE_FILE: 'request.planBundleFile'`, `PLAN_REVISIONS: 'request.planRevisions'`; cập nhật `request-rpc-contract.test.ts` nếu nó đối chiếu danh sách kênh.
2. `shared/request-plan-revision.ts`: kiểu `PlanRevision` (`revision`, `runId`, `status: 'current'|'superseded'`, `engine`, `revisionFeedback`, `actorId`, `generatedAt`, `commit?`, `filesChanged`, `fileCount`, `textKept`, `stats?`), `PlanBundleFileEntry` (`path`, `size`, `omitted`, `sha256?`, `change: 'added'|'changed'|'removed'|'unchanged'`), `PlanBundleView` (`changeId`, `branch`, `commit`, `status`, `syncState`, `files`), `PlanCheck`, `PlanKnowledge`, `PlanProposalView` (kiểu dây của `request.planProposal`: `runId`, `status`, `engine`, `proposal`, `bundle?`, `checks?`, `knowledge?`, `errorCode?`, `errorMessage?`), hằng `REQUEST_PLAN_REVISIONS_FEATURE = 'request.planRevisions'`.
3. `request-plan-revision-parsers.ts`: `parsePlanRevisions(value)`, `parsePlanBundleFile(value)`, `parsePlanProposalView(value)`; **không bao giờ ném**: trường thiếu → mặc định an toàn (`files: []`, `textKept: true`, `change: 'unchanged'`, `status: 'superseded'` nếu lạ), phần tử hỏng bị bỏ, `revisionFeedback` cắt về chuỗi.
4. `request-plan-generation.ts`: thêm `generatePlanPropose(id, {feedback?, idempotencyKey?})` chỉ gọi `mode: 'propose'` (kèm `feedback`, `idempotencyKey` khi có) và trả `{runId}`; **giữ** `generatePlanProposeCommit` cho "Generate plan" lần đầu cho tới khi CR-REQ-044 thay đổi; `PlanGenerationOptions` thêm `idempotencyKey`. Không đổi hành vi thăm dò.
5. Hàm `isPlanRevisionsSupported(features: ReadonlySet<string>): boolean` (đọc cờ `request.planRevisions`) và selector `supportsPlanRevisions` trong `store/slices/request.ts` theo mẫu `baseRef`. Cờ `request.planAuto` (server commit) và selector `supportsPlanAuto` do FE-REQ-TASK-044-01 thêm; nếu chưa vào thì task này **không** thêm (tránh hai bản), chỉ import khi có.
6. `request-errors.ts`/`plan-decision-error-message.ts`: nếu chưa ánh xạ, thêm nhận dạng cho năm mã mới (phân loại theo mã backend như `backendErrorCode`); thông điệp người dùng do task 05 viết trong `plan-revision-errors.ts`.

## Kiểm thử

- Parser: payload đầy đủ, thiếu trường, sai kiểu, `change` lạ, `files` null, danh sách rỗng, nhiều trang.
- `generatePlanPropose`: gửi đúng tham số, không commit, `feedback` rỗng bị bỏ khỏi payload.
- `isPlanRevisionsSupported` với và không có cờ.
- `pnpm vitest run` trên các file đã đổi; kiểm kiểu.

## Tiêu chí hoàn thành

- [x] Parser không bao giờ ném với gateway cũ hoặc payload lạ. Bằng chứng: `request-plan-revision-parsers.test.ts` (`it.each([null, undefined, 'text', 42, [], true …])` cho ba parser, trường thiếu, sai kiểu, `change` lạ, `files` null, hai trang).
- [x] `generatePlanPropose` chỉ propose; `generatePlanProposeCommit` không đổi hành vi. Bằng chứng: `request-plan-generation.test.ts` (`generatePlanPropose`: một lệnh gọi, không commit, `feedback` rỗng bị bỏ; test cũ của `generatePlanProposeCommit` giữ nguyên và xanh).
- [x] Hằng RPC và cờ tính năng có mặt; test hợp đồng RPC xanh. Bằng chứng: `request-rpc-contract.test.ts` xanh (7 test) sau khi gateway đăng ký `request.planRevisions` trong `channel_inventory.json`; `store/slices/request.test.ts` cho `supportsPlanRevisions`.

## Rủi ro và lưu ý

- Chạy `gitnexus_impact` cho `generatePlanProposeCommit` (dùng ở `usePlanDecision`, `RequestPlanTab` qua `regeneratePlan`).
- Tên trường dây (`revisionFeedback`, `filesChanged`, …) theo task BE 08; đối chiếu `CONTRACT-request-ui-api.md` khi nó đã cập nhật.

## Ghi chú triển khai (2026-10-20)

- **File mới**: `shared/request-plan-revision.ts` (kiểu `PlanRevision`, `PlanBundleFileEntry`, `PlanBundleView`, `PlanCheck`, `PlanKnowledge`, `PlanProposalDocument`/`PlanPhaseProposal`/`PlanTaskProposal`, `PlanProposalView`, `REQUEST_PLAN_REVISIONS_FEATURE`, `isPlanRevisionsSupported`), `shared/request-plan-revision-parsers.ts` (+ test: `parsePlanRevisions`, `parsePlanProposalView`, `parsePlanBundleFile`). **Sửa**: `shared/request-rpc-methods.ts` (`PLAN_BUNDLE_FILE`, `PLAN_REVISIONS`), `shared/request-errors.ts` (+ test: bảy mã `REQUEST_PLAN_REVISION_{IN_PROGRESS,NOT_APPLICABLE,NO_CHANGE,TEXT_PRUNED,NOT_FOUND}`, `REQUEST_PLAN_BUNDLE_{NOT_FOUND,FILE_NOT_FOUND}` ánh xạ kind), `shared/request-plan-auto.ts` (+ test: `origin` nhận `revision`, theo ghi chú của agent backend CR-044), `store/slices/request.ts` (+ test: selector `supportsPlanRevisions`), `hooks/request-plan-generation.ts` (+ test: `generatePlanPropose`, `idempotencyKey` trong `PlanGenerationOptions`).
- **Lệnh**: `vitest run` trên `src/shared/request-plan-revision-parsers.test.ts request-errors.test.ts request-plan-auto.test.ts request-rpc-contract.test.ts store/slices/request.test.ts hooks/request-plan-generation.test.ts`: 6 file, 177 test xanh. `tsc` giữ mốc 113 lỗi, không lỗi mới. `oxlint` sạch trên file đã đổi.
- **Quyết định theo dây thật** (đọc `channels_request_plan_revisions.go` và proto): gateway trả JSON `protojson` nên số `int64` (`size`, `durationMs`) có thể là chuỗi `"123"` và số 0 / `false` có thể bị lược hoặc được phát (`EmitUnpopulated`): parser nhận cả số lẫn chuỗi số, `hasCost:false` nghĩa là không báo chi phí (không phải miễn phí). `textKept` mặc định `true` khi thiếu theo spec; nếu gateway lược `false` thì lần đọc file sẽ trả `REQUEST_PLAN_REVISION_TEXT_PRUNED` và hook hiển thị đúng "pruned" (không hỏng). Phiên bản trùng số bị gộp, sắp mới nhất trước.
- **Sai lệch**: (1) `request-errors.ts` KHÔNG thêm `REQUEST_PLAN_FORBIDDEN`/`REQUEST_PLAN_FEEDBACK_TOO_LONG`/`REQUEST_BASE_REF_NOT_FOUND`: các mã này được nhận dạng theo `backendCode` trong `plan-revision-errors.ts` (như mẫu Analysis), tránh đổi kind của luồng cũ. (2) `isPlanRevisionsSupported(features)` nằm ở `shared/request-plan-revision.ts`; selector của store bọc nó. (3) `PENDING_GATEWAY_CHANNELS` của test hợp đồng không phải thêm gì: gateway đã đăng ký kênh trong lúc làm.
- **Blast radius** (grep, không có GitNexus): `generatePlanProposeCommit` được `usePlanDecision.regeneratePlan` và `useRequestActions.ts` gọi (chữ ký tương thích: `idempotencyKey` tuỳ chọn, không đổi payload khi vắng), và `usePlanRevision` mới; `request-rpc-methods.ts` thêm hằng nên `RequestRpcMethod` rộng ra, không đổi chỗ dùng cũ.
- **Chưa kiểm**: dây thật của `request.planRevisions` / `bundle.files[].change` trên stack (task 08).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
