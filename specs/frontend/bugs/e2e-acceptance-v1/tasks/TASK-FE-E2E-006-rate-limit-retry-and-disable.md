# TASK-FE-E2E-006: Xử lý `rate_limited`: khoá nút trong khoảng chờ và thử lại thao tác idempotent

**From Solution:** SOL-FE-E2E-003 · **Resolves:** FE-E2E-B
**Priority:** P1 · **Area:** frontend
**Files:**
- frontend/src/renderer/src/runtime/request-rpc-client.ts
- các hook ghi: useApprovals, useSolutions, request-plan-generation
**Depends on:** TASK-FE-E2E-004, TASK-E2E-031
**Status:** `[x]` DONE (2026-10-10), kiểm bằng vitest với đồng hồ giả; chưa thử với bộ giới hạn thật của gateway

---

## Context

Bấm lại nhiều lần khi bị giới hạn làm tăng tải.

## Việc cần làm

- [x] `callRequestRpc`: khi `REQUEST_RATE_LIMITED` và thao tác an toàn để lặp, thử lại **một lần** sau `retryAfterMs` (mặc định 1 giây). Quy tắc nằm ở `runtime/request-rate-limit.ts`; ngoại lệ gọi tường minh bằng `options.idempotent`.
- [x] Thao tác không idempotent: không thử lại; trả lỗi và khoá nút.
- [x] Trạng thái `coolingDown` để khoá nút: không lặp lại ở từng hook mà gom ở `hooks/useRequestCooldown.ts` (`useRequestCooldown('read'|'write'|'ai')` → `{coolingDown, remainingMs}`); `usePlanDecision` trả thêm `coolingDown`/`generateCoolingDown`.

## Tiêu chí hoàn thành

- [x] Test client với đồng hồ giả (`request-rpc-client.test.ts`, 8 ca); test chính sách và kho cooldown (`request-rate-limit.test.ts`); test render khoá/mở nút (`TypeConfirmationCard.ai-setup.test.tsx`, `SolutionPanel.test.tsx`).

## Kiểm thử

Vitest.

## Kết quả thực thi

**Thiết kế (và lý do)**
- Gateway giới hạn theo **lớp** (đọc/ghi/AI), nên cooldown theo lớp: `ai` = `solution.generate`, `request.generatePlan`, `request.classify`; `write` = thao tác đổi trạng thái (create, cancel, confirmType, approve/reject, clarification.answer...); còn lại là `read`.
- Tự thử lại một lần cho: mọi lệnh **đọc**, và các lệnh ghi có bảo vệ (`solution.generate` có `idempotencyKey`; `clarification.answer`, `approval.approve|reject` có `expectedVersion`/digest nên lần hai thất bại an toàn); gọi tường minh `idempotent: true` cho `request.create` (có `clientRequestId`, lần lặp trả `created:false`) và commit Plan (`alreadyExists`). **Không** thử lại: `request.cancel/reopen/confirmType/startPhase...`, `request.generatePlan` propose (mỗi lần mở một run AI), `request.classify`.
- Chỉ mã `REQUEST_RATE_LIMITED` mới thử lại/khoá. `REQUEST_AI_QUOTA_EXHAUSTED` (cũng kind `rate_limited`) và `REQUEST_CLASSIFICATION_LIMIT` thì không (chờ lâu hoặc vĩnh viễn).
- `retryAfterMs` > 5 giây: không giữ cuộc gọi, trả lỗi ngay và nút bị khoá đúng thời gian đó (tối đa 30 giây).
- Sau lần thử lại vẫn bị chặn: bắt đầu cooldown của lớp; nút ghi/AI của các màn hình đã nối tự mở lại khi hết giờ.

**File đổi / mới**
- `runtime/request-rate-limit.ts` (mới), `hooks/useRequestCooldown.ts` (mới), `runtime/request-rpc-client.ts` (vòng thử lại, tham số `options`).
- Nút nối vào cooldown: `TypeConfirmationCard` (Confirm = write, Reclassify = ai), `SolutionDecisionBar` (Approve/Reject = write, Regenerate/Generate plan = ai), `SolutionPanel` (thay bộ đếm 5 giây tự chế bằng cooldown lớp `ai`, giữ sàn 5 giây cho lỗi `rateLimited` từ thao tác người dùng), `PlanDecisionButtons`, `PhaseApprovalBar` (Start phase), `PlanApprovalBar` (Regenerate), `PlanStates` (Generate), `ApprovalInboxTab` → `ApprovalList`/`ApprovalRow` (quick approve/reject; nút Open vẫn dùng được), `ClarificationPanel` (Submit), `CreateRequestDialog` (Create).
- `CreateRequestDialog`/`request-action-feedback`: lời lỗi theo mã (`REQUEST_RATE_LIMITED` hiện "You are going too fast...").
- `request-plan-generation.ts`: commit đánh dấu `idempotent`; `useRequestActions.test.ts` cập nhật đối số thứ ba.

**Lệnh**: `pnpm exec vitest run --config config/vitest.config.ts src/renderer/src/runtime/request-rpc-client.test.ts src/renderer/src/runtime/request-rate-limit.test.ts src/renderer/src/components/request` → xanh.

**Blast radius (rà tay)**: `callRequestRpc` được dùng bởi ~25 hook/component; chữ ký thêm tham số thứ ba tuỳ chọn, mọi caller cũ giữ nguyên, hành vi chỉ đổi khi có `REQUEST_RATE_LIMITED` (trước đó là lỗi `unknown` hoặc, với `REQUEST_CLASSIFICATION_LIMIT`, `rate_limited` — mã đó nay không thay đổi). Các test mock `callRequestRpc` (mocked module) không bị ảnh hưởng. Electron và web dùng chung (`callRuntimeRpc` chọn đích). Trạng thái cooldown là biến mô-đun của renderer (không lưu localStorage), mỗi cửa sổ riêng — đủ vì giới hạn tính theo người dùng ở máy chủ.

**Chưa làm / chưa kiểm**
- Chưa chạy với bộ giới hạn thật (OBS-C: 5 ghi/giây, burst 10): kiểm bằng đồng hồ giả và thông điệp mô phỏng đúng định dạng `channels_request_errors.go`.
- `useApprovals`/`useSolutions` không trả `coolingDown` riêng (tránh trường chết): nút đọc cooldown trực tiếp qua `useRequestCooldown`.
- Các nút không thuộc luồng Request (task, chat...) không đổi; `CancelRequestDialog`, `ReturnToBacklogDialog`, `ChangeTypeConfirmDialog`, `SpawnChildRequestDialog` và nút Reopen vẫn chỉ hiện thông báo khi bị giới hạn (không khoá nút).

**Bổ sung (TASK-FE-E2E-009, 2026-10-10)**: nút "Generate solution" mới dùng lớp giới hạn `ai` của `useRequestCooldown` (khoá khi đang chờ; test trong `SolutionPanel.test.tsx`).
