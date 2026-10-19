# FE-REQ-TASK-037-02: Kiểu, parser, hằng kênh, hook `request.flow`, `request.executionSummary`, `request.activity` và sửa bảng dự phòng

**From Solution:** [FE-REQ-SOL-037](../solutions/FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) mục 2.2, CR-037 mục 2.2
**Priority:** P1
**Area:** frontend / shared + hooks
**File:** `frontend/src/shared/{request-types,request-wire-parsers,request-rpc-methods,request-flow-registry}.ts` (sửa), `frontend/src/shared/request-flow-registry.fixture.json` (mới), `frontend/src/renderer/src/hooks/{useRequestFlow,useExecutionSummary,useRequestActivity}.ts` (mới) và test cùng tên, `request-flow-registry.test.ts` (sửa/mới)
**Depends on:** FE-REQ-TASK-018-01, 018-02, 018-03; backend BE-REQ-TASK-038-04, 05, 06, 07 cho chạy thật (kiểu và hook làm trước được với dữ liệu giả)
**Status:** [x] DONE

---

## Context

- `REQUEST_RPC_METHODS` không có hằng cho `request.flow` dù gateway đã có kênh; hai kênh còn lại là mới (BE-SOL-038).
- `REQUEST_FLOW_REGISTRY` lệch `domain.FlowFor` (SOL-037 mục 1). Bảng này chỉ là dự phòng khi runtime chưa có `request.flow`.
- Parser của dự án chịu enum lạ (không ném lỗi, rơi về `unknown`), theo `request-wire-parsers.ts`.
- Kênh `unsupported` không phải lỗi (quy ước CR-REQ-018).

## Việc cần làm

1. Hằng: `FLOW: 'request.flow'`, `ACTIVITY: 'request.activity'`, `EXECUTION_SUMMARY: 'request.executionSummary'`.
2. Kiểu: `RequestFlowView`; `OrcaRequest` thêm `engine?: 'native'|'openspec'|'unknown'`, `executionMode?: 'step_by_step'|'continuous'|''`; `ExecutionSummary` (`progress`, `currentPhaseId`, `running[]`, `failed[]`, `needsYou[]`, `handover`, `nextPhaseId`); `NeedsYouItem` (`kind: approval|clarification|failed_task|pre_deploy|drift`); `RequestActivityEntry` (`kind`, `severity`, `actorKind`, `occurredAt`, `summary`, `subjectType`, `subjectId`, `attempt?`, `details?`). Parser tương ứng, chịu `kind` lạ (giữ chuỗi, hiển thị chung).
3. Hook:
   - `useRequestFlow(type, size)`: cache theo khoá, dự phòng sang bảng tĩnh khi `unsupported`.
   - `useExecutionSummary(requestId, {enabled})`: poll 5 giây khi `enabled`, tạm dừng khi `document.visibilityState !== 'visible'`, làm mới theo sự kiện qua `useRequestSubscription`.
   - `useRequestActivity(requestId, filter)`: `entries`, `loadMore`, `isLoading`, `error`, `unsupported`.
4. Sửa `REQUEST_FLOW_REGISTRY` cho khớp backend (bảng ở SOL-037 mục 1). Sinh `request-flow-registry.fixture.json` từ `FlowFor` (script nhỏ ở backend `request-service`, ghi lệnh trong PR) và test: với từng loại, bảng dự phòng khớp fixture (`analysisKind`, `analysisGate`, `planKind`, `hasPhases` theo cỡ, `startGate`, `executionGates`, `completesAfterAnalysis`).
5. Không rẽ nhánh `type === '...'` trong hook; mọi quyết định đọc từ `RequestFlowView`.

## Kiểm thử

- Parser: dữ liệu đủ, thiếu trường, `kind` lạ, `engine` lạ.
- `useRequestFlow`: gọi một lần cho cùng `(type,size)`; `unsupported` trả dự phòng; hợp nhất với `openSpecProfile`.
- `useExecutionSummary`: poll bật/tắt theo `enabled` và độ hiển thị; refetch khi sự kiện.
- `useRequestActivity`: phân trang, đổi bộ lọc xoá danh sách, `unsupported`.
- Fixture khớp bảng dự phòng cho 11 loại.
- `pnpm --filter orca-frontend test -- src/shared/request src/renderer/src/hooks/useRequestFlow src/renderer/src/hooks/useExecutionSummary src/renderer/src/hooks/useRequestActivity` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Ba hằng kênh và kiểu mới có mặt, parser chịu giá trị lạ.
- [x] Ba hook chạy với dữ liệu giả và trả `unsupported` không lỗi.
- [x] Bảng dự phòng khớp fixture backend cho 11 loại.
- [x] Không còn dùng bảng tĩnh khi `request.flow` có sẵn.

## Rủi ro và lưu ý

- Tên trường camelCase của hai kênh mới chốt ở `CONTRACT-request-ui-api.md` (BE task 07); đối chiếu khi backend merge.
- Poll nhiều Request cùng lúc chưa đo: hook chỉ chạy cho Request đang chọn.
- Fixture phải được tái sinh khi `FlowFor` đổi; ghi cách tái sinh vào đầu file test.

## Ghi chú triển khai

Hoàn thành 2026-10-16.

- Kênh mới `request.activity`, `request.executionSummary` trong `request-rpc-methods.ts` (tham số định danh là `id`, không phải `requestId`); kiểu `RequestFlowView`, `ExecutionSummary`, `RequestActivityEntry` ở `shared/request-execution-types.ts`; parser chịu giá trị lạ (`kind` lạ giữ nguyên chuỗi, tập đóng lạ thành `unknown`) ở `request-execution-parsers.ts` (+test).
- `OrcaRequest.engine`/`executionMode` và parser ở `request-types.ts`/`request-wire-parsers.ts`.
- `request-flow-registry.ts` viết lại theo `FlowFor` của backend (không theo bảng cũ: ví dụ chỉ `change_request` có cổng Phase, `hotfix`/`security` có cổng `pre_deploy`, `docs`/`task` có cổng `task_list`). Bảng được khoá bằng `request-flow-registry.fixture.json`, do `flow_fixture_test.go` của backend sinh (`UPDATE_FLOW_FIXTURE=1`); test FE đọc cùng file nên hai bên không lệch được. Bắt được một lỗi của chính tôi: `performance` có `openSpecProfile = light`.
- Hook `useRequestFlow` (cache theo `type|size`, bảng dự phòng khi chưa có hoặc runtime cũ), `useExecutionSummary` (làm mới theo sự kiện và poll 5 giây khi `executing`, dừng khi trang ẩn, `unsupported` không lỗi), `useRequestActivity` (phân trang con trỏ, đổi bộ lọc thì bắt đầu lại). Test: 21 hook + 187 test `shared/`.
- `RequestPlanTab` dùng `request.flow` của backend để biết cổng duyệt khi có; bảng tĩnh chỉ còn là dự phòng.
