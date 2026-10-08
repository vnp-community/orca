# FE-REQ-TASK-036-02: Hook `useClarifications`, `useDecisions`, `useImpactAssessment`, `useTaskReadiness`, `useExecutionResult`

**From Solution:** [FE-REQ-SOL-036](../solutions/FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md) mục 2.3, 2.4, 2.5, 2.6, 2.7, 2.8
**Priority:** P0
**Area:** frontend / hooks
**File:** `frontend/src/renderer/src/hooks/{useClarifications,useDecisions,useImpactAssessment,useTaskReadiness,useExecutionResult}.ts` (mới); test cùng tên
**Depends on:** FE-REQ-TASK-036-01 (kiểu, hằng kênh), 018-02 (`callRequestRpc`, `subscribeRequestBus`), 018-03 (mẫu `useRequest`, `useApprovals`)
**Status:** [x] DONE (verified 2026-10-07: vitest hooks useClarifications (3), useDecisions (3), useImpactAssessment (8), useTaskReadiness (4), useExecutionResult (3) pass)

## Context

- Mẫu hook của dự án: `useRequest`, `useApprovals`, `useSolutions` (FE-REQ-TASK-018-03, 020-01); `useTaskSource.ts` cho cách ẩn êm khi runtime không có kênh. Mỗi hook: tải một lần theo tham số, `cancelled` khi unmount, hàm ghi trả `Result<T, RequestRpcError>` không ném, làm mới bằng `subscribeRequestBus`.
- CONTRACT mục 3: `request.event` chỉ kích hoạt tải lại; không bảo đảm giao đủ, nên mỗi hook phải tải lại khi mở màn hình và khi kết nối lại; sự kiện không mang nội dung câu hỏi hay trả lời.
- CR-028: `AnswerClarification` có `complete` (`false` lưu nháp, `true` hoàn tất), `expectedVersion`; phản hồi `{clarification, requestStatus, requestRevision, stillMissing}`.
- CR-028 2.8: `solution.choose` ghi Decision; `decision.confirm {decisionId, confirmationText, expectedVersion}`; không có `decision.record`.
- CR-030: `impact.get {subjectType, subjectId}`, `impact.request`, `impact.compare {solutionId}`, `impact.findings {assessmentId, dimension?, minLevel?}`, `impact.evidence {findingId}`, `impact.accept`, `risk.override`; lỗi `REQUEST_RISK_ASSESSMENT_PENDING`.
- CR-029: `readiness.check {taskId}` (chạy khô và ghi báo cáo), `readiness.get {taskId}`, `readiness.list {phaseId}`; `execution.get {taskId}` là kênh đề xuất.
- Polling dự phòng: Request đang mở 15 giây, chỉ khi `document.visibilityState === 'visible'` (CONTRACT mục 3, `useRequestEvents` của 018-03 đã làm; hook của task này chỉ đăng ký bus).

## Việc cần làm

1. `useClarifications(requestId: string | null)`: `clarification.list {requestId, status?: 'open'}` rồi `clarification.get` cho bản mở (một Request tối đa một Clarification `open`); trả `{ open: Clarification | null, history: Clarification[], loading, error, refetch, answer(payload: AnswerPayload), cancel(id, expectedVersion), draft, setDraftValue(questionId, value), clearDraft, hasDraft }`. `draft` là state trong hook (`Map<questionId, unknown>`), **không** `localStorage`/`sessionStorage`; mất khi unmount hoặc đổi `clarificationId`. `answer` gọi `callRequestRpc(CLARIFICATION_ANSWER, payload)`, parse phản hồi, `refetch` khi thành công; trả `{ ok: true, value: { stillMissing, requestStatus } }` hoặc `{ ok: false, error }`; chống bấm đôi (cờ `submitting` và bỏ qua lời gọi thứ hai). Không gọi `complete=false` tự động (câu hỏi mở 3 SOL-036); nếu `payload.complete===false` thì cho gọi (người dùng chủ động, tuỳ chọn UI sau).
2. Sự kiện làm mới: `clarification.requested|answered|expired|cancelled`, `request.status_changed` với `requestId` khớp (`trigger` bất kỳ). Khi `request.status_changed` kèm `status!=='awaiting_information'` thì `open=null` ngay trước khi tải lại.
3. `useDecisions(requestId, solutionId?)`: `decision.list {requestId}`; trả `{ decisions, current: Decision | null (theo solutionId hoặc subjectKind='solution_option' mới nhất), loading, error, refetch, confirm(decisionId, confirmationText, expectedVersion) }`. `confirm` gọi `decision.confirm`; lỗi `REQUEST_DECISION_CONFIRMATION_MISMATCH` → `validation` hiển thị cạnh ô gõ. Làm mới theo `decision.recorded|confirmed`, `solution.approved|proposed`, `approval.decided`.
4. `useImpactAssessment({ subjectType, subjectId, solutionId? })`: `impact.get` (tóm tắt), `impact.findings {assessmentId, minLevel: 'medium'}` sau khi có `assessmentId`; `compare` khi có `solutionId` (`impact.compare`). Trả `{ summary: ImpactSummary | null, findings: ImpactFinding[], comparison: ImpactComparison[] | null, status: 'idle'|'loading'|'collecting'|'ready'|'unsupported'|'noDevServer'|'forbidden'|'error', request(): Promise<Result>, acceptRisk({findingId, rationale}): Promise<Result>, override({gate, reason}): Promise<Result>, refetch }`. `acceptRisk` gửi `impact.accept {assessmentId: summary.assessmentId, findingId, rationale, assessmentDigest: summary.digest}`; `rationale.trim().length < 10` bị chặn ngay ở client với `{ ok:false, error: validation }` (không gọi mạng); `override` yêu cầu `reason.trim().length >= 20`. `REQUEST_RISK_ASSESSMENT_STALE` thì `refetch` và đặt `acceptedFindingIds` cục bộ về rỗng (nơi gọi dùng cờ `acceptancesInvalidated`). `status='collecting'` khi `summary.status==='collecting'` hoặc lỗi `REQUEST_RISK_ASSESSMENT_PENDING`: lặp tải 3 giây tối đa 5 lần.
5. `useTaskReadiness({ taskId?: string; phaseId?: string })`: `readiness.get` cho task, `readiness.list` cho Phase (trả `byTaskId: Record<string, TaskReadinessReport>` và `phaseSummary {ready, needsInfo, specDefect, envDefect, unchecked}`); `check(taskId)` gọi `readiness.check` rồi cập nhật báo cáo; `unsupported` → `status='unsupported'` và không khoá nút Chạy (hành vi SOL-021). Làm mới theo `readiness.reported`. Câu hỏi mở 7: nếu `readiness.check` trả `runId` thay vì báo cáo, hook chờ sự kiện (cờ cấu hình nhỏ, mặc định đồng bộ).
6. `useExecutionResult(taskId)`: `execution.get {taskId, latestOnly: true}` (tạm); trả `{ result, status: 'idle'|'loading'|'ready'|'legacy'|'unsupported'|'error', refetch }`; `legacy` khi không có bản ghi (task trước CR-029); `unsupported` khi `method_not_found`. Làm mới theo `execution.verified` và `task.statuschanged` đã có trong bus (nếu bus phát; kiểm 018-02).
7. Mọi hook dùng một hàm nội bộ `useRefetchOnRequestEvent(requestId, eventTypes, refetch)` đặt trong `hooks/useRefetchOnRequestEvent.ts` (mới, tách vì dùng chung; tên cụ thể, không `helpers`).
8. Mọi lời gọi qua `callRequestRpc` (đúng `getActiveRuntimeTarget`, hợp SSH/remote); không đọc `localStorage` cho dữ liệu nhạy cảm.

## Bảng tham chiếu nhanh

| Hook | Kênh gọi | Sự kiện làm mới | Trạng thái |
|---|---|---|---|
| `useClarifications` | `clarification.list`, `.get`, `.answer`, `.cancel` | `clarification.*`, `request.status_changed` | `loading`, `submitting`, `error` |
| `useDecisions` | `decision.list`, `decision.confirm` | `decision.*`, `solution.approved`, `approval.decided` | `loading`, `error` |
| `useImpactAssessment` | `impact.get`, `.findings`, `.compare`, `.request`, `.accept`, `risk.override` | `impact.assessed`, `risk.accepted` | `idle`, `loading`, `collecting`, `ready`, `unsupported`, `noDevServer`, `forbidden`, `error` |
| `useTaskReadiness` | `readiness.get`, `.list`, `.check` | `readiness.reported` | `idle`, `loading`, `ready`, `unsupported` |
| `useExecutionResult` | `execution.get` (tạm) | `execution.verified` | `idle`, `loading`, `ready`, `legacy`, `unsupported`, `error` |

- Khoá i18n: không (hook không có chuỗi giao diện; lỗi trả mã để component dịch).
- Phím tắt: không.
- Chặn client: `rationale` accept ≥ 10, `reason` override ≥ 20 (sau trim); lỗi trả `validation` mà không gọi mạng.

## Trình tự làm gợi ý

1. Viết `useRefetchOnRequestEvent` và test.
2. Viết `useClarifications` (test `draft` không dùng `Storage`), rồi `useDecisions`.
3. Viết `useImpactAssessment` (test chặn `rationale` và `reason`, `collecting` lặp), `useTaskReadiness`, `useExecutionResult`.
4. Chạy toàn bộ test hook; kiểm huỷ khi unmount.
5. Ghi trong PR các kênh "(tạm)" cần đối chiếu CONTRACT.

## Kiểm thử

- `useClarifications.test.tsx` (mock `callRequestRpc`, `subscribeRequestBus`): tải `open`; `answer` gửi một lần với cả danh sách; hai lần bấm liên tiếp chỉ một lời gọi; `draft` không dùng `localStorage` (spy `Storage.prototype`); `request.status_changed` ra khỏi `awaiting_information` xoá `open`; `NOT_ASSIGNEE` → `forbidden`.
- `useDecisions.test.tsx`: `current` đúng; `confirm` gọi đúng tham số; `CONFIRMATION_MISMATCH` trả `validation`.
- `useImpactAssessment.test.tsx`: `acceptRisk` rationale 9 ký tự không gọi mạng, 10 ký tự (sau trim) gọi với `assessmentDigest` đúng; `ASSESSMENT_STALE` làm mới và đặt `acceptancesInvalidated`; `collecting` lặp tối đa 5 lần (fake timers); `unsupported`, `noDevServer`, `forbidden`; `override` 19 ký tự bị chặn, 20 gọi.
- `useTaskReadiness.test.tsx`: `check` cập nhật báo cáo; `list` tạo `phaseSummary` đúng; `unsupported` không khoá.
- `useExecutionResult.test.tsx`: `legacy` khi không có bản ghi; `unsupported`.
- Huỷ khi unmount: không `setState` sau huỷ (mỗi hook).
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/hooks/useClarifications src/renderer/src/hooks/useDecisions src/renderer/src/hooks/useImpactAssessment src/renderer/src/hooks/useTaskReadiness src/renderer/src/hooks/useExecutionResult src/renderer/src/hooks/useRefetchOnRequestEvent`.

## Tiêu chí hoàn thành

- [ ] Năm hook trả `Result` không ném, huỷ đúng khi unmount, làm mới theo sự kiện.
- [ ] Nháp trả lời không rời bộ nhớ của hook (không `localStorage`).
- [ ] Chặn client cho `rationale < 10` (accept) và `reason < 20` (override); gửi `assessmentDigest`.
- [ ] `unsupported` ẩn êm cả năm hook.
- [ ] Không gửi `tenantId`/`userId`; mọi tham số camelCase.

## Rủi ro và lưu ý

- Kênh `execution.get` và hình dạng `readiness.list` chưa chốt; giữ thay đổi trong hook và `request-rpc-methods.ts`.
- `answer` có thể mất kết nối giữa chừng: sau lỗi `network`, `refetch` để biết Clarification đã `answered` chưa (CR-028 2.4 bước 6: gửi lặp cùng nội dung trả kết quả hiện có), không tự gửi lại.
- Không gộp `useImpactAssessment` với `useGraphLens` (SOL-032): khác vòng đời (một bên tóm tắt, một bên đồ thị), tránh gọi trùng nhưng chia sẻ `assessmentId` qua tham số.
- Mỗi hook nhỏ gọn; nếu vượt ngưỡng `max-lines`, tách `*-result-merging.ts`, không thêm disable.

## Ghi chú triển khai (2026-10-07)

Thêm `useRefetchOnRequestEvent.ts` và 5 hook. Nháp trả lời chỉ trong state; hook bắt mọi lỗi để không có unhandled rejection. `useImpactAssessment` trả thêm `canOverride` (từ `viewerCan.override` của `impact.get`) và export `ImpactAssessmentApi`.
