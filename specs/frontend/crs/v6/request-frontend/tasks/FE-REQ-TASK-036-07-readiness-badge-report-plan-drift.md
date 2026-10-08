# FE-REQ-TASK-036-07: `ReadinessBadge`, `ReadinessReportSheet`, `PlanDriftBanner` và `PlanDriftReviewSheet`

**From Solution:** [FE-REQ-SOL-036](../solutions/FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md) mục 2.7
**Priority:** P1
**Area:** frontend / request / readiness
**File:** `frontend/src/renderer/src/components/request/readiness/{ReadinessBadge,ReadinessReportSheet,PlanDriftBanner,PlanDriftReviewSheet,PhaseReadinessSummary}.tsx`, `readiness-action-rules.ts` (mới); `components/request/plan/PlanTaskRow.tsx`, `PhaseNode.tsx`, `RequestPlanTab.tsx` (sửa, FE-REQ-TASK-021-03/021-05); `components/task/TaskDetail.tsx` (sửa); test cùng tên
**Depends on:** FE-REQ-TASK-036-01, 036-02, 036-03 (mở `ClarificationPanel`), FE-REQ-TASK-032-06 (`GraphPanel` lens `execution`), FE-REQ-TASK-021-03, 021-05, 021-04
**Status:** [x] DONE (verified 2026-10-08: vitest readiness-components (9), readiness-action-rules (5), TaskDetail-request-artifacts (6 mới), TaskDetail (25), PlanTree (8, +1 chip Lệch), usePhaseDrift (2) pass)

## Context

- CR-029 2.4: `TaskReadinessReport.outcome` ∈ `ready|needs_info|spec_defect|env_defect`; hành động phía backend: `ready` → render packet và `Execute`; `needs_info` → `RequestClarification(source=task_blocked, source_ref=task_id)`, Request sang `awaiting_information`, không tính lần thử; `spec_defect` → `ReturnToBacklog(stage=plan, category=other)` ở bản đầu (chuyển `plan_revision` khi cho phép); `env_defect` → giữ task `open`, Request `executing`, cảnh báo, thử lại vòng đối soát, quá `REQUEST_DISPATCH_RETRY_WINDOW` thì `ReturnToBacklog(stage=task, category=blocked_dependency)`, không tính lần thử. Báo cáo chứa `findings [{code, tier, path?, message}]` (mã ổn định, **không** chứa giá trị biến môi trường).
- Ba tầng (CR-036 2.6): Cấu trúc, Ngữ nghĩa, Môi trường (`tier`); chỉ **tên** biến môi trường được hiện.
- Kênh: `readiness.check {taskId}` (chạy khô, ghi báo cáo), `readiness.get {taskId}`, `readiness.list {phaseId}`; sự kiện `readiness.reported` (CR-029: `{request_id, task_id, attempt, outcome, tier}`). Runtime không có kênh (`unsupported`) thì giữ hành vi SOL-021 (không khoá nút Chạy).
- Lệch kế hoạch (CR-030 2.7): `impact.drift_detected` ở `enforce` mở Approval `subject_type=phase`, `stage=drift_review`; `OnApproved` tiếp tục, `OnRejected` là `ReturnToBacklog(stage=phase, category=rejected)`; **không có RPC huỷ Phase** và Task đang chạy vẫn chạy tới cùng. Ở `shadow` chỉ ghi và hiển thị. `impact.drift {phaseId}` trả dự kiến so với thực tế. **Lệch với CR-036 2.7**: "Trả về Plan" đưa về `planning` và "Huỷ Phase" không có ở backend (SOL-036 Correction 5): dùng `approval.approve`/`approval.reject` và bỏ nút "Huỷ Phase".
- `PlanTaskRow` (021-03) có chỗ cho badge; `TaskDetail` không nhận props (đọc `activeTaskId` từ store); nút Chạy bị khoá bởi `executionGateByTaskId` trong store (SOL-021): thêm điều kiện sẵn sàng vào cổng này.
- Hành động "Kết nối dev server": tái dùng điều hướng có sẵn của app tới màn kết nối dev server (đọc code trước khi viết; không tạo luồng mới).

## Việc cần làm

1. `readiness-action-rules.ts` (thuần): `getReadinessPresentation(outcome): { labelKey, Icon, tone }` (`ready` `CircleCheck`, `needs_info` `MessageCircleQuestion`, `spec_defect` `FileWarning`, `env_defect` `ServerCrash`; `unknown` không badge); `getReadinessAction(report, ctx: { canWrite: boolean; hasDevServer: boolean }): { kind: 'run' | 'answer' | 'regenerate' | 'connect' | 'notify' | 'none'; labelKey; noteKey?: string }` theo bảng CR-036 2.6 (`env_defect`: `kind: hasDevServer ? 'notify' : 'connect'`, `noteKey = 'NotCountedAsAttempt'`); `isRunBlockedByReadiness(report | null, supported): boolean` (null hoặc `unsupported` → false; `ready` → false; còn lại true); `groupFindingsByTier(findings)`; `summarizePhase(list): { ready, needsInfo, specDefect, envDefect, unchecked }`.
2. `ReadinessBadge.tsx`: props `{ report: TaskReadinessReport | null; compact?: boolean }`; chữ + icon (`Badge`); `null` hoặc `unknown` không render; `Tooltip` nêu `tier` và thời điểm; click mở `ReadinessReportSheet` (khi không `compact`). Chỉ cho task làm việc (không `plan`, `phase`; kiểm `isWorkTask` của `task-hierarchy.ts` ở 021-01).
3. `ReadinessReportSheet.tsx`: `Sheet`; đầu: `ReadinessBadge` + `checkedAt` + `specDigest` rút gọn; thân: ba nhóm `Cấu trúc / Ngữ nghĩa / Môi trường` (từ `tier`), mỗi mục `code` (monospace), `message`, `path`, đạt/không (icon + chữ), **chỉ tên** biến môi trường; nút chính theo `getReadinessAction` (`ready` "Chạy" là nút hiện có của `TaskDetail`, không thêm đường chạy mới; `needs_info` "Trả lời câu hỏi" gọi cuộn/mở `ClarificationPanel` (đặt `useAppStore.openRequestPage({section:'requests', requestId})`, mở `Sheet` tại đó; Request đã ở `awaiting_information` do backend); `spec_defect` "Sinh lại task" (nhãn nói rõ "Trả về bước Plan", điều hướng tới tab Plan, **không** gọi RPC mới), `env_defect` "Kết nối dev server" hoặc "Báo người vận hành" kèm dòng "Không tính vào số lần thử"). Nút "Kiểm tra sẵn sàng" gọi `check(taskId)` (vô hiệu khi đang chạy, hiện skeleton); nói rõ "Việc chạy thật vẫn tự kiểm lại".
4. `PhaseReadinessSummary.tsx`: hàng tóm tắt "7 sẵn sàng, 1 thiếu thông tin, 1 lỗi môi trường" (từ `readiness.list`, `summarizePhase`), hiển thị ở `PhaseNode`; không có dữ liệu (`unsupported` hoặc rỗng) thì không render.
5. `PlanTaskRow` (sửa): thêm `ReadinessBadge compact`; chip "Lệch" khi task thuộc Phase có drift thấp hơn ngưỡng (chỉ chip, không dải; dữ liệu từ `impact.drift` nếu `level` thấp; nếu hình dạng không đủ thì bỏ chip và ghi câu hỏi mở).
6. `TaskDetail` (sửa): đầu chi tiết hiện `ReadinessBadge` và nút "Kiểm tra sẵn sàng"; nối `isRunBlockedByReadiness` vào cổng Chạy sẵn có: task chưa `ready` có nút Chạy khoá kèm tooltip nêu lý do (tiếp nối quy tắc "Phase chưa duyệt" của SOL-021); `unsupported` giữ hành vi cũ.
7. `PlanDriftBanner.tsx`: hiện ở đầu `RequestPlanTab` khi nhận sự kiện `impact.drift_detected` hoặc có Approval `phase` với `stage==='drift_review'` và `status==='pending'`: "Thay đổi thực tế vượt kế hoạch. Phase <tên> đã tạm dừng." (chỉ ở `enforce`; `shadow` hiển thị "Tham khảo" không dải đỏ); nút chính "Xem và duyệt lại" mở `PlanDriftReviewSheet`. Không phím tắt.
8. `PlanDriftReviewSheet.tsx`: bảng dự kiến so với thực tế (file và service đã đổi ngoài `scope`, điểm thực tế so với dự kiến, ngưỡng đã vượt, bằng chứng tóm tắt dạng văn bản) từ `impact.drift`; `GraphPanel` lens `execution` kèm nhãn "lệch" (`meta.drifted`); hành động: "Chấp nhận lệch và tiếp tục" (lý do bắt buộc ≥ 10 ký tự bằng `RejectReasonDialog`-kiểu `DecisionRationaleField`; gọi `approval.approve {id, expectedVersion, expectedDigest, comment}`; quyền theo backend) và "Trả về" (`approval.reject` với lý do; ghi rõ "Request sẽ về backlog theo quy định", không hứa về `planning`); bỏ nút "Huỷ Phase". Task đang chạy tới cùng: dòng ghi chú "Các task đang chạy vẫn chạy tới khi xong".

## Bảng tham chiếu nhanh

| Kết quả | Icon | Nút chính | Ghi chú |
|---|---|---|---|
| `ready` | `CircleCheck` | "Chạy" (nút hiện có) | không thêm đường chạy |
| `needs_info` | `MessageCircleQuestion` | "Trả lời câu hỏi" | mở `ClarificationPanel` |
| `spec_defect` | `FileWarning` | "Sinh lại task" | backend `ReturnToBacklog(stage=plan)` |
| `env_defect` | `ServerCrash` | "Kết nối dev server" hoặc "Báo người vận hành" | "Không tính vào số lần thử" |

- Kênh WS và payload: `readiness.check {taskId}`, `readiness.get {taskId}`, `readiness.list {phaseId}`, `impact.drift {phaseId}`, `approval.approve|reject {id, expectedVersion, expectedDigest, comment}` (Approval `phase`, `stage=drift_review`).
- Sự kiện: `readiness.reported`, `impact.drift_detected`.
- Phím tắt: không có cho hành động lệch kế hoạch; `Esc` đóng sheet.
- Khoá i18n: `auto.components.request.readiness.{badge.ready,badge.needs_info,badge.spec_defect,badge.env_defect,check,checking,notCounted,tier.structure,tier.semantic,tier.environment,envVarNames,action.run,action.answer,action.regenerate,action.connect,action.notify,phase.summary,drift.banner,drift.review,drift.accept,drift.return,drift.runningNote,drift.shadow}`.
- Trạng thái: không có dữ liệu `readiness.*` (`unsupported`) thì không badge và không khoá nút Chạy.

## Kiểm thử

- `readiness-action-rules.test.ts`: 4 kết quả → đúng hành động và nhãn; `env_defect` có `NotCountedAsAttempt`; `isRunBlockedByReadiness` (null, `unsupported`, ready, các lỗi); `groupFindingsByTier`; `summarizePhase`.
- `ReadinessBadge.test.tsx`: 4 giá trị có chữ và icon; `null` không render; không hiện giá trị biến môi trường (truyền `message` có tên biến, kiểm không có giá trị).
- `ReadinessReportSheet.test.tsx`: ba nhóm; nút chính đúng theo kết quả; "Kiểm tra sẵn sàng" gọi `check`; `needs_info` điều hướng tới Clarification; `spec_defect` điều hướng tab Plan và không gọi RPC.
- `PlanDriftBanner.test.tsx`: hiện khi Approval `drift_review` `pending`; `shadow` không dải; "Xem và duyệt lại" mở sheet.
- `PlanDriftReviewSheet.test.tsx`: "Chấp nhận lệch" khoá tới khi có lý do; gọi `approval.approve` đúng tham số; "Trả về" gọi `reject`; không có nút "Huỷ Phase".
- `TaskDetail` (mở rộng): Chạy khoá khi chưa `ready` (tooltip lý do); `unsupported` không khoá.
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/components/request/readiness src/renderer/src/components/request/plan src/renderer/src/components/task/__tests__/TaskDetail`.

## Tiêu chí hoàn thành

- [ ] `ReadinessBadge` hiện đủ 4 giá trị; mỗi giá trị có đúng nút chính; `env_defect` ghi "không tính lần thử".
- [ ] Task chưa `ready` có nút Chạy khoá kèm lý do; runtime không có `readiness.*` giữ hành vi cũ.
- [ ] Chỉ tên biến môi trường được hiện, không giá trị.
- [ ] `impact.drift_detected` (hoặc Approval `drift_review`) hiện `PlanDriftBanner`; "Chấp nhận lệch" đòi lý do.
- [ ] Không có nút "Huỷ Phase"; "Trả về" ghi rõ kết quả backlog.
- [ ] Không phím tắt cho hành động lệch; không màu hex.

## Rủi ro và lưu ý

- `readiness.check` có thể chạy lâu hoặc bất đồng bộ (câu hỏi mở 7 SOL-036); `check` hiển thị trạng thái "Đang kiểm tra" và chờ `readiness.reported`.
- `spec_defect` ở bản đầu đưa Request về backlog (`ReturnToBacklog(stage=plan)`), không phải sinh lại tại chỗ: nhãn phải trung thực.
- Hình dạng `impact.drift` chưa chốt; parser chịu thiếu; bảng dự kiến so với thực tế hiển thị trường có; thiếu thì "Chưa có dữ liệu".
- Khoá nút Chạy cứng hay chỉ cảnh báo là câu hỏi mở 7 của CR-036; bản này khoá khi backend hỗ trợ, không khoá khi `unsupported`.
- `TaskDetail`, `PlanTaskRow`, `PhaseNode`, `RequestPlanTab` thuộc SOL-021: chỉ thêm điểm cắm cần thiết.

## Ghi chú triển khai (2026-10-07)

`ReadinessBadge`/`ReadinessReportSheet`/`PhaseReadinessSummary` xong và gắn vào `PlanTaskRow`, `PhaseNode`, `TaskDetail` (badge, nút "Kiểm tra sẵn sàng", khoá Chạy qua `isRunBlockedByReadiness`; chỉ cho task có `requestId`). `PlanDriftSection` (banner + sheet) gắn vào `RequestPlanTab`; hành động chỉ "chấp nhận" (approve) và "trả về" (reject), không "huỷ Phase". Chưa: chip "Lệch" trên `PlanTaskRow`, test render `TaskDetail` với báo cáo readiness, đường dẫn tới màn dev server (nút chỉ hiện khi host truyền `onConnectDevServer`).

## Ghi chú triển khai (2026-10-08)

- Chip "Lệch" trên `PlanTaskRow` (`drifted`, chữ + icon, token `risk-medium`): `hooks/usePhaseDrift.ts` gọi `impact.drift {phaseId}` (tạm) một lần mỗi Phase đã chạy (`in_progress|review|done`), nạp lại khi có `impact.drift_detected`; `unsupported` thì không chip.
- `TaskDetail` nối `ReadinessReportSheet.onConnectDevServer` → `useOpenDevServerSettings` (Settings > Servers).
- Test tích hợp `components/task/__tests__/TaskDetail-request-artifacts.test.tsx`: `needs_info` hiện badge và khoá Chạy kèm lý do; `env_defect` mở sheet, chỉ tên biến, "Kết nối dev server" mở Settings > Servers; `ready` không khoá; task ngoài Request không có UI readiness; runtime `unsupported` không khoá Chạy.
