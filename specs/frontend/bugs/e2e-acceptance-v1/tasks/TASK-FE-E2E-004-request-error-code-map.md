# TASK-FE-E2E-004: Bổ sung bảng ánh xạ mã lỗi Request

**From Solution:** SOL-FE-E2E-003 · **Resolves:** FE-E2E-B
**Priority:** P1 · **Area:** frontend
**Files:**
- frontend/src/shared/request-errors.ts
- frontend/src/shared/request-errors.test.ts
**Depends on:** TASK-E2E-015, 024, 031, 033 (mã lỗi)
**Status:** `[x]` DONE (2026-10-10)

---

## Context

`REQUEST_RATE_LIMITED` và các mã mới rơi vào `unknown`.

## Việc cần làm

- [x] Thêm vào `CODE_TO_KIND`: `REQUEST_RATE_LIMITED`, `REQUEST_FLOW_DISABLED`, `REQUEST_PROJECT_NOT_FOUND`, `REQUEST_AI_NOT_CONFIGURED`, `REQUEST_AI_CREDENTIAL_UNAVAILABLE`, `REQUEST_AI_AGENT_UNAVAILABLE`, `REQUEST_AI_SESSION_EXPIRED`, `REQUEST_AI_QUOTA_EXHAUSTED`, `PROJECT_DEV_SERVER_*` (5 mã có thật trong Go).
- [x] Thêm kiểu `no_ai` và `feature_off` vào union; mọi nơi phân nhánh theo kind đã rà (xem blast radius). `ERROR_COPY` nay là `Record<RequestRpcErrorKind, ...>` nên thêm kind mà thiếu lời là lỗi biên dịch.
- [x] Đọc `retryAfterMs` từ thông điệp: đuôi JSON `{"retryAfterMs":N}` mà gateway thêm vào cuối (`channels_request_errors.go`).

## Tiêu chí hoàn thành

- [x] Test bảng cho từng mã (it.each, mỗi mã kiểm cả `kind` lẫn `backendCode`).
- [x] Không còn mã nào trong bảng SOL-FE-E2E-003 bị phân loại `unknown`.

## Kiểm thử

Vitest.

## Kết quả thực thi

**Phát hiện thêm khi đối chiếu với Go (đã sửa)**: CONTRACT mục 5 chốt mọi mã có tiền tố `REQUEST_`, và gateway thật gửi `REQUEST_APPROVAL_*`, `REQUEST_SOLUTION_*`; `CODE_TO_KIND` chỉ có dạng ngắn (`APPROVAL_*`, `SOLUTION_*`) nên `REQUEST_APPROVAL_NOT_FOUND`, `REQUEST_APPROVAL_VERSION_CONFLICT`, `REQUEST_APPROVAL_ALREADY_DECIDED`... đều là `unknown`. Nay bảng có cả hai dạng cho toàn bộ mã trong CONTRACT mục 5 (131 mục), thêm `INVALID_ARGUMENT` (gateway trả khi tham số sai), `REQUEST_UNAVAILABLE`, `REQUEST_NOT_IMPLEMENTED`, `TASK_NOT_FOUND`, `TASK_FORBIDDEN`.

**Ánh xạ chính** (kiểu → hành vi): `REQUEST_RATE_LIMITED`/`REQUEST_AI_QUOTA_EXHAUSTED` → `rate_limited`; `REQUEST_FLOW_DISABLED` → `feature_off`; `REQUEST_PROJECT_NOT_FOUND` → `not_found`; `REQUEST_AI_NOT_CONFIGURED|CREDENTIAL_UNAVAILABLE|SESSION_EXPIRED|NO_PROVIDER` → `no_ai`; `REQUEST_AI_AGENT_UNAVAILABLE`, `REQUEST_SOLUTION_NO_CONNECTION`, `REQUEST_ANALYSIS_NO_CONNECTION`, `PROJECT_DEV_SERVER_{NOT_FOUND,UNREACHABLE,CONNECTION_FAILED,HEALTH_CHECK_FAILED}` → `no_dev_server`; `REQUEST_AI_BUSY`, `REQUEST_AI_COMPLETE_TIMEOUT`, `PROJECT_DEV_SERVER_LOOKUP_FAILED` → `unavailable`.
- Lưu ý: `REQUEST_AI_CREDENTIAL_UNAVAILABLE` có trong task/solution nhưng **backend chưa phát ra mã này** (grep Go: không có); đã ánh xạ để sẵn.
- `REQUEST_AI_QUOTA_EXHAUSTED` là `rate_limited` nhưng KHÔNG được thử lại tự động (TASK-FE-E2E-006 chỉ thử lại đúng mã `REQUEST_RATE_LIMITED`).
- Mã chưa gặp vẫn là `unknown`; nay `RequestRpcError` mang thêm `backendCode` (mã trong thông điệp) và `retryAfterMs`. Trước đây `error.code` là mã vận chuyển (`runtime_error`), vài nơi (TypeConfirmationCard, CreateRequestDialog) so sánh nhầm; đã chuyển sang `backendCode ?? code`.

**File đổi**
- `shared/request-errors.ts`: kind `no_ai`, `feature_off`; trường `backendCode`, `retryAfterMs`; `requestErrorKindForCode`, `parseRetryAfterMs`, `canonicalApprovalSolutionCode`.
- `components/request/request-error-message.ts`: `ERROR_COPY` đủ kiểu (trước đó thiếu `unsupported`, `expired`, `pending`, `no_dev_server` → toàn bị "Something went wrong."), thêm `requestErrorMessageForError` cho lời theo mã.
- `components/request/solution/solution-error-classification.ts`: chuẩn hoá `REQUEST_APPROVAL_*`/`REQUEST_SOLUTION_*` về dạng ngắn trước khi so (trước đó `REQUEST_APPROVAL_EXPIRED` không nhận ra).
- `components/request/request-action-feedback.ts`, `plan/plan-decision-error-message.ts`, `CreateRequestDialog.tsx`: dùng `requestErrorMessageForError`.
- Locale 5 ngôn ngữ: `error.unsupported|expired|pending|noDevServer|noAi|featureOff|code.{rateLimited,projectNotFound,aiQuotaExhausted,aiBusy,aiTimeout,aiAgentUnavailable}`.
- Test: `request-errors.test.ts` (+45 ca bảng cho các mã trong SOL cộng các mã tiền tố, cùng test `retryAfterMs`, `backendCode`), `request-error-message.test.ts` (mới).

**Lệnh**: `pnpm exec vitest run --config config/vitest.config.ts src/shared/request-errors.test.ts src/renderer/src/components/request/request-error-message.test.ts` → xanh (83 test, 2 file); `tsc --noEmit -p tsconfig.json --composite false` không có lỗi mới ở file nào tôi đụng.

**Blast radius (rà tay)**: `grep` mọi nơi dùng `RequestRpcErrorKind|requestErrorMessage|classifyRequestRpcError` (25 file): không có `switch` nào bao trọn union kind (các chỗ có `switch` đều có `default`), nên thêm hai kind không làm hỏng nhánh nào; riêng `solution-error-classification` map `rate_limited` → `rateLimited` nay cũng gồm `REQUEST_AI_QUOTA_EXHAUSTED` (khoá nút sinh lại, đúng ý). `lint:switch-exhaustiveness` không chạy được ở cây này (thiếu `config/oxlint-switch-exhaustiveness.json`).

**Chưa làm**: `feature_off` chưa có nút "Settings" cho admin (không có màn hình bật/tắt luồng Request trong UI để trỏ tới; SOL ghi "admin thấy nút tới Settings"). Chuỗi hiển thị cho mã `REQUEST_FLOW_DISABLED` chỉ nêu "chưa được bật cho tổ chức này".
