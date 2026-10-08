# FE-REQ-TASK-020-04: `RejectReasonDialog`, `SolutionDecisionBar` và chuỗi chọn rồi duyệt

**From Solution:** [FE-REQ-SOL-020](../solutions/FE-REQ-SOL-020-solution-review-ui.md) mục 2.3, 2.4
**Priority:** P0
**Area:** frontend / request / solution
**File:** `frontend/src/renderer/src/components/request/solution/RejectReasonDialog.tsx`, `SolutionDecisionBar.tsx` (mới); `hooks/useSolutionDecision.ts` (mới); test cùng tên
**Depends on:** FE-REQ-TASK-020-01, 020-02, 020-03, 018-03
**Status:** [x] DONE (verified 2026-10-07: RejectReasonDialog.test.tsx, useSolutionDecision.test.ts, SolutionPanel.test.tsx pass)

## Context

- Kênh: `solution.choose {requestId, solutionId, optionId, comment?}`; `approval.approve {id, expectedVersion, expectedDigest, comment?}`; `approval.reject {id, expectedVersion, expectedDigest, comment}` (bắt buộc); `solution.generate {requestId, feedback?, idempotencyKey?}`.
- Mã lỗi: `APPROVAL_VERSION_CONFLICT`, `SOLUTION_VERSION_CONFLICT`, `APPROVAL_ALREADY_DECIDED`, `APPROVAL_EXPIRED`, `APPROVAL_NOT_APPROVER`, `APPROVAL_COMMENT_REQUIRED`.
- `RejectReasonDialog` sẽ được dùng lại ở SOL-021 và SOL-022: props độc lập Solution.
- `isScreenSubmitShortcut`, `getScreenSubmitModifierLabel`, `ShortcutKeyCombo`.

## Việc cần làm

1. `RejectReasonDialog({open, title, onSubmit(comment, {regenerate}), onCancel, offerRegenerate})`: `Textarea` tự lấy tiêu điểm; tối thiểu `REJECT_REASON_MIN_LENGTH` ký tự sau trim; `aria-invalid` và dòng "Cần nhập lý do" khi chạm vào mà chưa đủ; nút "Từ chối" `variant="destructive"` khoá khi chưa đủ; ô "Sinh lại dựa trên phản hồi" mặc định bật khi `offerRegenerate`. `Mod+Enter` gửi khi hợp lệ; chip nhãn bằng `ShortcutKeyCombo keys={[getScreenSubmitModifierLabel(), 'Enter']}` (`⌘` Mac, `Ctrl` nơi khác).
2. `useSolutionDecision({solution, approval, request})`: `approveSelected(optionId?, comment?)` chạy bước `choose` (nếu `kind==='solution'` và `chosenOption!==optionId`) rồi `approve`; ghi nhớ bước đã xong để thử lại bước hai không gọi lại bước một; `reject(comment, regenerate)` gọi `reject` rồi, nếu `regenerate`, `generate({feedback: clampFeedback(comment), idempotencyKey})`; trạng thái `submitting` khoá mọi nút (chống bấm đôi).
3. `SolutionDecisionBar`: dính đáy, theo `getSolutionPresentation.actions`; nhãn "Chấp nhận" cho `answer`; không có phím tắt cho Duyệt; khi `readOnly` hoặc nhớ `APPROVAL_NOT_APPROVER` cho `approval.id` thì "Bạn không có quyền duyệt".
4. Xử lý lỗi: `conflict` → `refetch` + banner "Solution vừa được cập nhật" và giữ lựa chọn nếu còn; `invalid_state`/`APPROVAL_ALREADY_DECIDED` → `refetch`, chỉ đọc; `expired` → nhãn "Quá hạn" chỉ cho "Sinh lại"; `forbidden` → toast; `validation` → lỗi cạnh trường.
5. Không bao giờ có đường từ chối không lý do.

## Kiểm thử

- `RejectReasonDialog`: rỗng/dưới 10 ký tự không gọi `onSubmit`; đúng 10 sau trim gọi; `Mod+Enter` với `metaKey` (mock Mac) và `ctrlKey` (mock Linux/Windows); `ctrlKey` trên Mac không gửi; chip nhãn đúng.
- `useSolutionDecision`: chuỗi `choose`→`approve`; lỗi bước hai rồi thử lại không gọi `choose` lần hai; `conflict` giữ lựa chọn; `reject` + `regenerate` gửi `feedback` đã cắt 2000; chống bấm đôi.
- Lệnh gọi đúng `expectedVersion`, `expectedDigest` từ `Approval`.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/solution/RejectReasonDialog src/renderer/src/components/request/solution/SolutionDecisionBar src/renderer/src/hooks/useSolutionDecision`.

## Tiêu chí hoàn thành

- [ ] Từ chối lý do rỗng/ngắn: không có yêu cầu gửi đi.
- [ ] `approve` kèm đúng `expectedVersion`/`expectedDigest`.
- [ ] Phím tắt đúng nền tảng; nhãn khớp.
- [ ] i18n `RejectReasonDialog.*`, `SolutionDecisionBar.*` đủ 5 locale.

## Rủi ro và lưu ý

- Chuỗi hai lời gọi có thể dừng giữa chừng khi mất mạng: UI báo rõ "đã chọn nhưng chưa duyệt" và cho thử lại.
- Nếu CONTRACT gộp `choose` vào `approve`, thay bước 1 bằng tham số; giữ `useSolutionDecision` là điểm đổi duy nhất.

## Ghi chú triển khai (2026-10-07)

- Sai lệch: hook đặt tại `components/request/solution/useSolutionDecision.ts` (không phải `hooks/`) do phạm vi tệp; chữ ký `onSubmit(comment, {regenerate})` tương thích API được yêu cầu cho SOL-021.
- `approve` dùng `approvalDigest` do `solution.choose` trả về làm `expectedDigest` (CONTRACT).
- CẦN SỬA ngoài phạm vi: `useApprovals.approve/reject` đang gửi `approvalId`, CONTRACT yêu cầu `id` (agent 019 đang xử lý).
