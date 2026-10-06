# FE-REQ-TASK-036-04: Decision: lý do khi chọn khác đề xuất, xác nhận lần hai gõ tên, lịch sử chọn

**From Solution:** [FE-REQ-SOL-036](../solutions/FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md) mục 2.5
**Priority:** P0
**Area:** frontend / request / decision
**File:** `frontend/src/renderer/src/components/request/decision/{DecisionRationaleField,HighRiskDecisionConfirmDialog,DecisionHistoryList}.tsx`, `decision-rules.ts` (mới); `components/request/solution/SolutionDecisionBar.tsx`, `hooks/useSolutionDecision.ts` (sửa, FE-REQ-TASK-020-04); `components/request/plan/PlanApprovalBar.tsx` (sửa, 021-04); test cùng tên
**Depends on:** FE-REQ-TASK-036-01, 036-02, FE-REQ-TASK-020-04 (`SolutionDecisionBar`, `useSolutionDecision`, `RejectReasonDialog`), 021-04
**Status:** [ ] TODO

## Context

- CR-028 2.8 (đã đọc): `ChooseSolutionOption` ghi Decision cùng transaction; `chosen_option_id != recommended_option_id` thì `rationale` không rỗng (`REQUEST_DECISION_RATIONALE_REQUIRED`); `DecisionRisk.Assess(option)` quyết định `risk_level='high'` (breaking change, `risks[].severity=high`, số service ≥ 3 mặc định đề xuất): phương án `high` chỉ `status=chosen`, cần `ConfirmDecision {decisionId, confirmationText, expectedVersion}` mới `effective`; xác nhận phải trùng tiêu đề phương án sau chuẩn hoá NFC, cắt khoảng trắng, **không phân biệt hoa thường** (`REQUEST_DECISION_CONFIRMATION_MISMATCH`); người xác nhận là chính người chọn hoặc admin; danh tính máy bị từ chối; chọn lại khi Approval còn `pending` xoá xác nhận cũ; duyệt chặn khi Decision không `effective` hoặc `subject_digest` không khớp (`REQUEST_DECISION_NOT_EFFECTIVE`); `REQUEST_DECISION_SELF_CHOICE_FORBIDDEN` khi người báo cáo tự chọn và `self_approval_allowed=false`.
- **Lệch với CR-036 2.3:** CR-036 viết `decision.record` rồi `choose`/`approve`, và "phân biệt hoa thường". Backend: không có `decision.record` (ghi cùng `solution.choose`) và so khớp không phân biệt hoa thường. Dùng backend (SOL-036 mục 1, hàng 2 và 3).
- `useSolutionDecision` (020-04): chuỗi `choose` → `approve`, ghi nhớ bước đã xong; chặn bấm đôi; `RejectReasonDialog` dùng `REJECT_REASON_MIN_LENGTH = 10`.
- STYLEGUIDE dòng 296: xác nhận chọn phương án không mất dữ liệu nên **không** `variant=destructive`; Cancel là ghost yên lặng; chip phím chỉ cho phím đã làm thật. Dialog: `ui/dialog.tsx`; `Esc` đóng (Radix mặc định), `Enter` xác nhận khi khớp.
- `solution.choose` hiện nhận `comment?` (CONTRACT 2.2); trường `rationale` là đề xuất (tạm, câu hỏi mở 4 SOL-036): đổi một chỗ trong `useSolutionDecision`.

## Việc cần làm

1. `decision-rules.ts` (thuần): `normalizeTitle(s: string): string` (`s.normalize('NFC').trim().toLowerCase()`); `matchesConfirmation(input, title): boolean` (so `normalizeTitle`); `requiresRationale(chosenId, recommendedId | undefined): boolean` (true khi có đề xuất và khác; false khi không có đề xuất); `isRationaleValid(text, required): boolean` (tối thiểu 10 ký tự sau trim khi bắt buộc; tuỳ chọn khi không); `getDecisionGate(decision: Decision | null, approval: Approval | null, currentDigest: string): 'ok' | 'noDecision' | 'needsConfirmation' | 'digestChanged' | 'superseded' | 'selfChoiceForbidden'`; hằng `DECISION_RATIONALE_MIN_LENGTH = 10` (import chung với `REJECT_REASON_MIN_LENGTH` nếu cùng giá trị, đặt tên nguồn duy nhất).
2. `DecisionRationaleField.tsx`: `Textarea` có nhãn "Lý do chọn"; bắt buộc khi `requiresRationale`, kèm dòng "Cần nhập lý do khi chọn khác đề xuất"; `aria-invalid` khi chạm mà chưa đủ; bộ đếm ký tự; `Mod+Enter` (`isScreenSubmitShortcut`) gọi `onSubmit` khi hợp lệ.
3. `HighRiskDecisionConfirmDialog.tsx`: props `{ open; decision: Decision; optionTitle: string; reasons: string[]; onConfirm(text: string): Promise<Result>; onCancel(): void }`. Hiện `reasons` (từ `Decision.options[i].risk.reasons`), ô `Input` gõ lại tên phương án (cho dán), tiêu điểm vào ô; nút "Xác nhận chọn" (mặc định, không destructive) khoá tới `matchesConfirmation`; `Enter` xác nhận khi khớp; `Esc` thoát; Huỷ là `Button variant="ghost"` không chip phím. Server từ chối (`CONFIRMATION_MISMATCH`) thì lỗi cạnh ô, không đóng. Chỉ gọi `decision.confirm {decisionId, confirmationText: text (nguyên văn người dùng), expectedVersion: decision.version}`.
4. `DecisionHistoryList.tsx`: từ `decision.list`, hiển thị theo thứ tự thời gian: ai (`chooserId`), lúc nào (thời gian tương đối), chọn gì, lý do, trạng thái (`open|chosen|effective|superseded` bằng `Badge`); bản `superseded` mờ.
5. `useSolutionDecision` (sửa): quy trình `approveSelected`: (a) nếu cần lý do, kiểm `isRationaleValid`; (b) `solution.choose {requestId, solutionId, optionId, rationale}`; (c) `useDecisions.refetch`; nếu `decision.riskLevel==='high'` thì trả `{ ok:true, needsConfirmation:true }` để `SolutionDecisionBar` mở hộp xác nhận (không gọi `approve`); (d) sau xác nhận (`status` thành `effective`) mới `approval.approve {id, expectedVersion, expectedDigest}`; lỗi bước sau không gọi lại bước trước (kế thừa 020-04). `Decision.subjectDigest` đổi so với lúc mở màn (`getDecisionGate==='digestChanged'`) thì khoá Duyệt và hiện banner "Nội dung vừa thay đổi, hãy xem lại" (khoá tới `refetch`).
6. `SolutionDecisionBar` (sửa): ghép các thành phần: chọn khác đề xuất → `DecisionRationaleField` bắt buộc, nút Duyệt khoá và nhãn "Cần nhập lý do khi chọn khác đề xuất"; `getDecisionGate` ánh xạ banner: `needsConfirmation` ("Chờ xác nhận lần hai", nút "Xác nhận chọn" mở hộp), `selfChoiceForbidden` ("Người báo cáo không được tự chọn", ẩn nút), `noDecision`. Danh sách lịch sử trong `Collapsible` bên dưới.
7. `PlanApprovalBar` (sửa, 021-04): nút Duyệt Plan khoá kèm tooltip "Chưa ghi nhận quyết định chọn phương án" khi Solution `kind==='solution'` chưa có Decision `effective` (đọc `useDecisions`); `unsupported` thì không khoá (hành vi cũ).
8. Quyền: `REQUEST_DECISION_AGENT_FORBIDDEN` và `SELF_CHOICE_FORBIDDEN` là `forbidden`: ẩn nút và dòng giải thích; không suy quyền ở client.

## Bảng tham chiếu nhanh

| Tình huống | UI | Mã lỗi backend liên quan |
|---|---|---|
| Chọn đúng đề xuất | lý do tuỳ chọn | không |
| Chọn khác đề xuất | lý do bắt buộc (≥ 10 ký tự) | `REQUEST_DECISION_RATIONALE_REQUIRED` |
| `riskLevel==='high'` | hộp gõ lại tên phương án | `REQUEST_DECISION_CONFIRMATION_MISMATCH` |
| Decision `chosen` chưa `effective` | banner "Chờ xác nhận lần hai", Duyệt khoá | `REQUEST_DECISION_NOT_EFFECTIVE` |
| `subjectDigest` đổi | banner "Nội dung vừa thay đổi", Duyệt khoá | `REQUEST_APPROVAL_VERSION_CONFLICT` |
| Người báo cáo tự chọn | ẩn nút | `REQUEST_DECISION_SELF_CHOICE_FORBIDDEN` |

- Kênh WS và payload: `solution.choose {requestId, solutionId, optionId, rationale?}` (tạm), `decision.confirm {decisionId, confirmationText, expectedVersion}`, `decision.list {requestId}`, `approval.approve {id, expectedVersion, expectedDigest, comment?}`.
- Phím tắt: `Mod+Enter` gửi lý do (nền tảng đúng như 036-03); `Enter` xác nhận khi tên khớp; `Esc` đóng hộp.
- Khoá i18n: `auto.components.request.decision.{rationaleLabel,rationaleRequired,confirmTitle,confirmBody,confirmInputLabel,confirmSubmit,pendingConfirmation,digestChanged,selfChoiceForbidden,history.title,history.empty,status.open,status.chosen,status.effective,status.superseded}`.

## Trình tự làm gợi ý

1. Viết `decision-rules.test.ts` (NFC, hoa thường, 9/10 ký tự), rồi `decision-rules.ts`.
2. Viết `DecisionRationaleField`, `HighRiskDecisionConfirmDialog`, `DecisionHistoryList` và test.
3. Sửa `useSolutionDecision` (thứ tự `choose` → `confirm` → `approve`) và test.
4. Ghép vào `SolutionDecisionBar` và `PlanApprovalBar`.
5. Kiểm nút xác nhận không có `variant=destructive`; chạy test nhóm `decision` và `solution`.

## Kiểm thử

- `decision-rules.test.ts`: `matchesConfirmation` với khoảng trắng thừa, hoa/thường khác, NFC vs NFD (chuỗi tiếng Việt có dấu soạn sẵn và tổ hợp), tiêu đề có ký tự đặc biệt; `requiresRationale` (không có đề xuất → false); `isRationaleValid` 9 và 10 ký tự sau trim; `getDecisionGate` đủ 6 giá trị.
- `HighRiskDecisionConfirmDialog.test.tsx`: tiêu điểm mặc định ở ô gõ; sai tên thì nút khoá; đúng (khác hoa thường) mở khoá; `Enter` xác nhận khi khớp; `Esc` đóng; nút không có `variant=destructive` (kiểm lớp); server lỗi giữ hộp.
- `SolutionDecisionBar.test.tsx` (mở rộng): chọn khác đề xuất không lý do → Duyệt khoá; chọn đúng đề xuất không bắt buộc; `riskLevel==='high'` mở hộp thay vì gọi `approve`; `digestChanged` hiện banner; `selfChoiceForbidden` ẩn nút; `Mod+Enter` đúng nền tảng.
- `useSolutionDecision.test.tsx` (mở rộng): thứ tự `choose` → (`confirm`) → `approve`; lỗi `approve` không gọi lại `choose`; `RATIONALE_REQUIRED` hiện lỗi trường.
- `DecisionHistoryList.test.tsx`: thứ tự, `superseded` mờ.
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/components/request/decision src/renderer/src/components/request/solution/SolutionDecisionBar src/renderer/src/hooks/useSolutionDecision`.

## Tiêu chí hoàn thành

- [ ] Chọn khác đề xuất không lý do thì không duyệt được; chọn đúng đề xuất thì lý do tuỳ chọn.
- [ ] Phương án `high` đòi gõ đúng tên (NFC, không phân biệt hoa thường); sai thì nút khoá; nút không `destructive`.
- [ ] Không gọi `decision.record` (không tồn tại); ghi Decision đi qua `solution.choose`.
- [ ] `subjectDigest` đổi hiện banner và khoá Duyệt; Plan khóa khi chưa có Decision `effective`.
- [ ] Lịch sử chọn hiển thị đủ ai, lúc nào, chọn gì, lý do, trạng thái.
- [ ] Phím tắt đúng nền tảng; `Esc` thoát hộp.

## Rủi ro và lưu ý

- `rationale` gửi ở `solution.choose` là giả định (CONTRACT hiện `comment?`); nếu backend chốt tên khác, đổi duy nhất ở `useSolutionDecision`.
- So khớp NFC phải giống server; chênh lệch ở ký tự Unicode lạ có thể khiến client báo khớp mà server từ chối: server luôn thắng, UI hiển thị lỗi cạnh ô.
- Nhiều lớp xác nhận (lý do, gõ tên, chấp nhận rủi ro) có thể gây mệt mỏi bấm cho qua; không thêm lớp nào ngoài bảng này.
- `PlanApprovalBar` và `SolutionDecisionBar` do SOL-020, 021 sở hữu: chỉ thêm đúng phần Decision, tránh xung đột.
