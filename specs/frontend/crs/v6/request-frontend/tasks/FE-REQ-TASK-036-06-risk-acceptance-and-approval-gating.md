# FE-REQ-TASK-036-06: `RiskAcceptanceChecklist`, cổng duyệt theo mức rủi ro và ghi đè cổng

**From Solution:** [FE-REQ-SOL-036](../solutions/FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md) mục 2.6
**Priority:** P1
**Area:** frontend / request / impact (duyệt)
**File:** `frontend/src/renderer/src/components/request/impact/{RiskAcceptanceChecklist,RiskOverrideMenu}.tsx`, `risk-approval-rules.ts` (mới); `components/request/solution/SolutionDecisionBar.tsx`, `components/request/plan/PlanApprovalBar.tsx`, `PhaseApprovalBar.tsx` (sửa); `hooks/useApprovals.ts` (sửa, FE-REQ-TASK-018-03: thêm tham số duyệt); test cùng tên
**Depends on:** FE-REQ-TASK-036-04 (`SolutionDecisionBar` đã tích hợp Decision), 036-05 (`RiskSummaryCard`, `ImpactFindingList`), 036-02 (`useImpactAssessment`)
**Status:** [~] PARTIAL — vitest impact/risk-approval-rules (5), RiskOverrideMenu (3), SolutionDecisionBar.decision (11) pass — thiếu: nối cổng vào PlanApprovalBar/PhaseApprovalBar

## Context

- Bảng hệ quả theo mức (CR-REQ-036 2.5, đối chiếu CR-030 2.6): Thấp luồng thường; Trung bình: phần tác động nằm trên nút Duyệt và phải mở trước khi nút bật (backend: cờ `viewed_impact_digest` trong `DecideApprovalRequest`; `ListPendingForUser` đánh dấu không cho duyệt hàng loạt); Cao: mỗi phát hiện từ Cao trở lên cần `RiskAcceptance` kèm lý do ≥ 10 ký tự (`REQUEST_RISK_ACCEPTANCE_REQUIRED`), thêm cổng `pre_deploy` và cờ tính năng/đường quay lui nếu thiếu; Nghiêm trọng: như Cao cộng "Chờ người duyệt thứ hai" (`required_approvals=2`, đề xuất CR-009, chưa có trong CONTRACT), nhắc chia Phase nhỏ (`REQUEST_PLAN_RISK_TOO_HIGH`).
- Chỉ `mode==='enforce'` mới chặn (CR-030 2.9); `shadow` chỉ hiển thị, không đòi chấp nhận.
- `impact.accept {assessmentId, findingId, rationale, assessmentDigest}` → `RiskAcceptance`; `digest` đổi (`REQUEST_RISK_ASSESSMENT_STALE`) thì chấp nhận cũ mất hiệu lực. `approval.approve` nhận thêm `viewedImpactDigest` và `acceptedFindingIds` (CR-030 mục 8, tên camelCase tạm).
- `risk.override {requestId, gate, reason}`: `reason` ≥ **20** ký tự (CR-030 2.6), quyền `team:<risk_override_team>` hoặc admin, có kiểm toán; menu thừa, **không** là nút chính (CR-036 2.5). `REQUEST_RISK_APPROVER_NOT_ALLOWED`: người không đủ quyền thấy nội dung và dòng "Cần người duyệt thuộc nhóm được chỉ định"; vai trò thật do backend, UI chỉ hiện tên team khi backend trả.
- "Duyệt nhanh" ở hộp duyệt (SOL-022) bị bỏ cho mức từ Trung bình (CR-036 mục 8): phía SOL-022 chưa sửa.

## Việc cần làm

1. `risk-approval-rules.ts` (thuần): `getApprovalRequirements(summary: ImpactSummary | null, findings: ImpactFinding[], state: { viewedImpact: boolean; acceptedFindingIds: ReadonlySet<string>; digestAtAcceptance: string | null }): { level; mode; requiresView: boolean; mustAccept: ImpactFinding[]; unaccepted: ImpactFinding[]; needsSecondApprover: boolean; canApprove: boolean; blockedReasonKey: string | null }`. Quy tắc: `summary==null` hoặc `level==='unknown'` → không chặn (hành vi SOL-020, `blockedReasonKey=null`, nhưng hiển thị "Chưa đánh giá"); `mode==='shadow'` → không chặn; `low` → không chặn; `medium` → `requiresView`, `canApprove` khi `viewedImpact`; `high` → `mustAccept = findings.filter(level >= 'high')`, `canApprove` khi `unaccepted` rỗng **và** `digestAtAcceptance===summary.digest`; `critical` → như `high` và `needsSecondApprover=true` (UI chỉ báo "Chờ người duyệt thứ hai"; không chặn nút duyệt của người thứ nhất, backend quyết). Thứ tự mức dùng `GRAPH_RISK_ORDER`.
2. `RiskAcceptanceChecklist.tsx`: props `{ requirements; onAccept(findingId, rationale): Promise<Result>; disabled?: boolean }`. Mỗi phát hiện từ Cao trở lên: `Checkbox` + `Textarea` "Lý do chấp nhận" (≥ 10 ký tự sau trim; `aria-invalid`), nút "Ghi nhận chấp nhận" chỉ bật khi đủ lý do; khi ghi nhận thành công hiển thị trạng thái "Đã chấp nhận" với tên người và thời gian (`RiskAcceptance.acceptedBy`, `createdAt`). `digest` đổi (hook báo `acceptancesInvalidated`) thì xoá mọi trạng thái đã chấp nhận và hiện banner "Đánh giá vừa thay đổi, hãy xác nhận lại"; không giữ lý do nháp nếu hợp lệ (giữ văn bản nháp trong state để người dùng chỉnh nhanh). Mức Nghiêm trọng hiện dòng "Chờ người duyệt thứ hai" và gợi ý "Chia Phase nhỏ hơn và thử nghiệm quay lui".
3. Mức Trung bình: bố trí phần tác động (`RiskSummaryCard` + `ImpactFindingList`) **trên** nút Duyệt; `viewedImpact` đặt true khi người dùng mở phần tác động (nút "Xem tác động" bật `Collapsible` hoặc cuộn qua theo `IntersectionObserver`, chọn mở `Collapsible` vì dễ test); gửi `viewedImpactDigest = summary.digest` khi duyệt.
4. `useApprovals.approve` (sửa): nhận thêm `{ viewedImpactDigest?: string; acceptedFindingIds?: string[] }` và đưa vào `approval.approve` (tên camelCase tạm); không đổi hành vi khi không truyền.
5. Ghép vào `SolutionDecisionBar`, `PlanApprovalBar`, `PhaseApprovalBar`: nút Duyệt khoá theo `getApprovalRequirements(...).canApprove` kèm tooltip `blockedReasonKey`; sau khi `approve` lỗi `REQUEST_RISK_ACCEPTANCE_REQUIRED` hoặc `REQUEST_RISK_ASSESSMENT_PENDING`/`STALE`: `refetch` và hiển thị đúng khối còn thiếu, không toast lặp. Mức `critical`: sau duyệt người thứ nhất, hiện "Chờ người duyệt thứ hai" theo `approval.status`/`decidedBy` (chỉ phản ánh dữ liệu; không tự suy).
6. `RiskOverrideMenu.tsx`: `DropdownMenu` nhỏ ("Bỏ qua cổng…") chỉ hiện khi backend cho phép (không suy quyền ở client: hiện khi `useImpactAssessment.canOverride` — cờ lấy từ phản hồi `impact.get` nếu có `viewerCan.override`, nếu không có cờ thì ẩn menu và ghi câu hỏi mở); mở `Dialog` yêu cầu lý do ≥ 20 ký tự (`aria-invalid`, bộ đếm); không phím tắt; nút "Bỏ qua cổng" mặc định (không destructive), Huỷ ghost; gọi `risk.override {requestId, gate, reason}`; thành công thì `refetch`; lỗi `REQUEST_RISK_OVERRIDE_REASON_REQUIRED` lỗi trường, `forbidden` thông báo.
7. Dòng người duyệt thiếu quyền: `REQUEST_RISK_APPROVER_NOT_ALLOWED` → ẩn nút Duyệt và hiện "Cần người duyệt thuộc nhóm được chỉ định".
8. Phối hợp SOL-022: cung cấp `getApprovalRequirements` cho `ApprovalRow`; việc bỏ "Duyệt nhanh" và hiện `RiskBadge` ở hộp duyệt do FE-REQ-TASK-022-05 thực hiện (ghi chú trong PR của cả hai).

## Bảng tham chiếu nhanh

| Mức | Chặn Duyệt khi `enforce` | Gửi kèm `approval.approve` |
|---|---|---|
| Thấp / Chưa đánh giá | không | không |
| Trung bình | chưa mở phần tác động | `viewedImpactDigest` |
| Cao | còn phát hiện Cao chưa `RiskAcceptance`, hoặc digest đổi | `acceptedFindingIds`, `viewedImpactDigest` |
| Nghiêm trọng | như Cao; hiện "Chờ người duyệt thứ hai" | như Cao |

- Kênh WS và payload: `impact.accept {assessmentId, findingId, rationale, assessmentDigest}`, `risk.override {requestId, gate, reason}`, `approval.approve {id, expectedVersion, expectedDigest, comment?, viewedImpactDigest?, acceptedFindingIds?}` (hai trường sau tạm).
- Phím tắt: không có phím tắt cho Duyệt hay ghi đè; `Mod+Enter` chỉ gửi ô lý do nếu hợp lệ (nền tảng đúng như 036-03).
- Khoá i18n: `auto.components.request.impact.{acceptance.title,acceptance.reasonLabel,acceptance.reasonRequired,acceptance.record,acceptance.recorded,acceptance.invalidated,acceptance.secondApprover,acceptance.splitPhase,gate.viewImpactFirst,gate.needsAcceptance,gate.approverNotAllowed,override.menu,override.title,override.reasonRequired,override.submit}`.
- Trạng thái lỗi: `REQUEST_RISK_ACCEPTANCE_REQUIRED`, `ASSESSMENT_STALE`, `ASSESSMENT_PENDING` thì `refetch` và hiện đúng khối còn thiếu, không toast lặp.

## Trình tự làm gợi ý

1. Viết `risk-approval-rules.test.ts` (bảng 4 mức × 2 chế độ) trước, rồi hàm thuần.
2. Mở rộng `useApprovals.approve` với hai tham số mới và test.
3. Viết `RiskAcceptanceChecklist` và `RiskOverrideMenu` kèm test.
4. Ghép vào ba thanh duyệt (`SolutionDecisionBar`, `PlanApprovalBar`, `PhaseApprovalBar`).
5. Báo cho người sở hữu SOL-022 về `getApprovalRequirements` và việc bỏ "Duyệt nhanh".

## Kiểm thử

- `risk-approval-rules.test.ts` (bảng test): 4 mức × `shadow|enforce` × {chưa xem, đã xem, thiếu chấp nhận, đủ chấp nhận, digest đổi}; `summary==null`; `level==='unknown'`; `critical` đặt `needsSecondApprover`.
- `RiskAcceptanceChecklist.test.tsx`: rationale 9 ký tự không bật nút ghi nhận, 10 bật; gọi `onAccept` đúng `findingId`; `acceptancesInvalidated` xoá trạng thái và hiện banner; `critical` hiện "Chờ người duyệt thứ hai".
- `SolutionDecisionBar.test.tsx`/`PlanApprovalBar.test.tsx`/`PhaseApprovalBar.test.tsx` (mở rộng): Duyệt khoá khi còn phát hiện Cao chưa chấp nhận; `medium` khoá tới khi mở phần tác động; `approve` gửi `viewedImpactDigest`, `acceptedFindingIds`; lỗi `ACCEPTANCE_REQUIRED` refetch; `APPROVER_NOT_ALLOWED` ẩn nút.
- `RiskOverrideMenu.test.tsx`: 19 ký tự khoá, 20 gửi; không có phím tắt; ẩn khi không có cờ quyền.
- `useApprovals.test.tsx` (mở rộng): tham số mới đi vào lời gọi.
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/components/request/impact src/renderer/src/components/request/solution src/renderer/src/components/request/plan src/renderer/src/hooks/useApprovals`.

## Tiêu chí hoàn thành

- [ ] Mức Cao: không Duyệt được khi còn phát hiện Cao chưa có `RiskAcceptance`; mức Nghiêm trọng hiện "Chờ người duyệt thứ hai".
- [ ] Mức Trung bình: Duyệt chỉ bật sau khi mở phần tác động; gửi `viewedImpactDigest`.
- [ ] `RiskAcceptance` cũ mất hiệu lực khi `assessmentDigest` đổi.
- [ ] `shadow` và "Chưa đánh giá" không chặn duyệt.
- [ ] Ghi đè cổng là menu thừa, lý do ≥ 20 ký tự, không phím tắt.
- [ ] Không có chỗ sửa điểm rủi ro.

## Rủi ro và lưu ý

- Tham số `viewedImpactDigest`, `acceptedFindingIds` và `required_approvals` chưa có trong CONTRACT; thay đổi gói trong `useApprovals` và `risk-approval-rules.ts`.
- Mệt mỏi bấm cho qua: lớp xác nhận nhiều; không thêm lớp ngoài bảng; theo dõi sau triển khai (chưa có nghiên cứu người dùng).
- Cờ `viewerCan.override` chưa có trong hợp đồng; nếu không có, menu ghi đè ẩn và quyền chỉ qua backend (câu hỏi mở).
- Hai ngưỡng khác nhau (10 cho chấp nhận, 20 cho ghi đè): đặt hằng riêng `RISK_ACCEPTANCE_MIN_LENGTH = 10`, `RISK_OVERRIDE_MIN_LENGTH = 20` trong `risk-approval-rules.ts`.
- Task này sửa ba thanh duyệt đang thuộc SOL-020 và 021: giữ thay đổi ở mức ghép nối.

## Ghi chú triển khai (2026-10-07)

Cổng theo mức chỉ nối vào `SolutionDecisionBar` (`useRiskApprovalGate`, `RiskAcceptanceChecklist`, Collapsible "Xem tác động", `RiskOverrideMenu` chỉ khi `canOverride`). `approval.approve` mang `viewedImpactDigest`/`acceptedFindingIds` (tên tạm) qua `useApprovals.approve` và `useSolutionDecision`. `PlanApprovalBar`/`PhaseApprovalBar` chưa có cổng rủi ro; "Duyệt nhanh" ở hộp duyệt (SOL-022) chưa sửa (ngoài phạm vi).
