# TASK-FE-E2E-005: Thông báo cấu hình AI có hướng khắc phục

**From Solution:** SOL-FE-E2E-003 · **Resolves:** BUG-E2E-010
**Priority:** P1 · **Area:** frontend
**Files:**
- frontend/src/renderer/src/components/request/ (banner trong chi tiết Request và panel Solution/Plan)
- i18n
**Depends on:** TASK-FE-E2E-004, TASK-E2E-024, 034
**Status:** `[x]` DONE (2026-10-10): mã + vitest + Playwright mock; banner `REQUEST_AI_*` chưa thấy trên stack thật vì AI đang sẵn có (chế độ `--ai-stub` đã kiểm đường `REQUEST_AI_NOT_CONFIGURED` ở backend)

---

## Context

Hiện chỉ có "classifier error".

## Việc cần làm

- [x] Banner theo mã lỗi/`classificationReason`: `REQUEST_AI_NOT_CONFIGURED` nêu "chạy `claude auth login` trên dev server" (lệnh hiện trong thẻ `<code>`); `REQUEST_AI_SESSION_EXPIRED` nêu đăng nhập lại; hết hạn mức, agent mất kết nối, AI bận, không đọc được khoá cũng có lời riêng. Admin thấy nút "Open dev server settings" (Settings → Servers; trong UI **không có** pane "AI providers"), người khác thấy "Ask your administrator to fix this.".
- [x] Hiển thị ở phân loại (`TypeConfirmationCard`), Solution (`SolutionPanel`) và Plan (`PlanStates`).
- [x] Không hiển thị khi AI chạy tốt (kiểm trong test: lý do phân loại bình thường, run `succeeded`/`running` không ra banner).

## Tiêu chí hoàn thành

- [x] Test render theo từng mã (vitest, xem dưới).
- [x] Playwright (mock) cho banner. — _ca e (Solution run lỗi do thiếu AI) và f (phân loại lỗi do thiếu AI) của `request-solution-start-and-ai-setup.web.e2e.ts`, 8/8 xanh_

## Kiểm thử

Vitest + Playwright.

## Kết quả thực thi

**Nguồn dữ liệu thật của lỗi AI** (đối chiếu Go + `suite_request_ai_stages.py`), vì đây quyết định chỗ phải đọc:
- Phân loại: `request.classificationReason` chứa **mã** (`REQUEST_AI_NOT_CONFIGURED`...) hoặc một trong các chuỗi cố định `no dev server connected`, `classifier error`, `classifier timeout`, `invalid classifier output`.
- Solution: `solution.generate` trả `runId`, lỗi nằm ở `runs[].errorCode` của `solution.list` và **không có sự kiện** khi run hỏng. UI trước đó bỏ qua `runs`, nên panel kẹt ở "AI is analyzing" mãi.
- Plan: `request.generatePlan` (propose) chạy nền, trả `{runId, proposal:null}`; kết quả/mã lỗi đọc bằng `request.planProposal`. **UI cũ dừng sau propose (proposal rỗng) nên không bao giờ commit Plan và không thấy lỗi**: đã sửa trong `request-plan-generation.ts` (xem dưới).

**File đổi / mới**
- `components/request/request-ai-setup.ts` (mới): `aiSetupCodeOf` (mã trần, `CODE: chi tiết`, hoặc chuỗi lý do đã biết; lý giải AI dạng văn xuôi không bao giờ khớp), `firstAiSetupCode`.
- `components/request/RequestAiSetupBanner.tsx` (mới): banner `role="alert"`, tiêu đề + cách khắc phục theo mã, dòng "Error code: ...", nút "Try again" tuỳ chọn, nút/dòng theo vai trò admin.
- `TypeConfirmationCard.tsx`: banner thay cho lý do thô khi là mã AI; chuỗi `classifier error|timeout|invalid classifier output` đổi thành câu dễ hiểu (lý do thô vẫn hiện nhỏ bên dưới); sửa việc khoá "Reclassify" vĩnh viễn: giờ chỉ khoá với `REQUEST_CLASSIFICATION_LIMIT` (trước đó mọi `rate_limited` đều khoá và ghi "đạt giới hạn 5 lần", sai với throttle/quota).
- `hooks/useSolutions.ts`: trả `latestRun` (run mới nhất); `shared/request-analysis-run-parsers.ts` (mới, do `request-wire-parsers.ts` sát trần 300 dòng) + kiểu `AnalysisRun` trong `request-types.ts`.
- `solution/SolutionPanel.tsx`, `solution/SolutionRunFailure.tsx` (mới), `solution/SolutionGenerationState.tsx`: run lỗi → banner (mã AI) hoặc khối "Could not generate" kèm "Error code"; Solution cũ vẫn hiện, lỗi của lần sinh lại nằm phía trên. Panel đọc lại `solution.list` mỗi 5 giây khi request còn `analyzing` mà chưa có Solution/lỗi (vì không có sự kiện).
- `hooks/request-plan-generation.ts`: propose → thăm dò `request.planProposal` mỗi 2 giây, tối đa 240 giây (cùng mức suite backend) → commit; run `failed` trả lỗi đã phân loại (có `backendCode`), nên `PlanStates` nhận ra mã AI và hiện banner (`generateAiCode`); vẫn tương thích máy chủ cũ trả proposal ngay. Thêm `REQUEST_RPC_METHODS.PLAN_PROPOSAL`.
- `plan/PlanStates.tsx`, `plan/RequestPlanTab.tsx`: truyền `generateAiCode`.
- Locale 5 ngôn ngữ: `aiSetup.*` (16 khoá), `TypeConfirmationCard.classifyFailed`, `SolutionGenerationState.errorCode`.
- Test: `TypeConfirmationCard.ai-setup.test.tsx` (12), `SolutionPanel.test.tsx` (+8), `useSolutions.test.ts` (+2), `request-plan-generation.test.ts` (8, mới), `RequestPlanTab.test.tsx` (+1), `request-analysis-run-parsers.test.ts` (mới).

**Selector Playwright**
- Banner: `data-testid="request-ai-setup-banner"` với `data-ai-code="REQUEST_AI_NOT_CONFIGURED"` (v.v.), `request-ai-setup-code` (chứa mã), `request-ai-setup-ask-admin` (không phải admin), nút tên "Try again" và "Open dev server settings" (admin); lệnh nằm trong `<code>`.
- Phân loại: `type-confirmation-card`, `classification-failed`, nút "Reclassify"/"Confirm type".
- Solution: `solution-panel-empty`, `solution-generation-failed`, `solution-generation-error-code`, `solution-generation-pending`; tab `request-tab-analysis`.
- Plan: `request-tab-plan`, `plan-generating`, `plan-generate` (nút), `plan-generate-error`.

**Lệnh**: `pnpm exec vitest run --config config/vitest.config.ts src/renderer/src/components/request src/renderer/src/hooks/useSolutions.test.ts src/renderer/src/hooks/request-plan-generation.test.ts src/shared` → các file này xanh (3 lỗi ở `src/shared/{feature-interactions,remote-runtime-shared-control-boundary}.test.ts` là CÓ SẴN, thiếu `src/main`, `src/relay` trong cây frontend).

**Blast radius (rà tay)**: `generatePlanProposeCommit` có hai caller (`useRequestActions.generatePlan`, `usePlanDecision.regeneratePlan`; qua đó `SolutionPanel`, `RequestPlanTab`, `PlanApprovalBar`). Hợp đồng trả về không đổi (`Result`), chỉ nay chờ lâu hơn (có thể tới 240 giây) trong khi nút ở trạng thái "đang chạy". `useSolutions` chỉ thêm trường. `TypeConfirmationCard`: đổi điều kiện khoá Reclassify (bug fix).

**Chưa làm / chưa kiểm**
- Chưa kiểm bằng trình duyệt thật hoặc stack thật: banner phân loại cần Request thật bị `REQUEST_AI_NOT_CONFIGURED` (stack AI-stub của agent F); hình dạng dữ liệu đối chiếu bằng đọc Go và `suite_request_ai_stages.py`, không gọi RPC lên stack.
- Thăm dò Plan không có cách huỷ: rời tab thì vòng lặp vẫn chạy đến khi xong hoặc hết 240 giây (mỗi 2 giây một lệnh đọc, nếu bị giới hạn tốc độ thì đi qua cơ chế thử lại của TASK-FE-E2E-006).
- `REQUEST_PLAN_AI_INVALID_JSON`, `REQUEST_SOLUTION_INVALID_OUTPUT` hiện là lỗi chung có kèm mã (không có banner riêng).

**Bổ sung (TASK-FE-E2E-009, 2026-10-10)**: đã có Playwright mock cho banner AI (`request-solution-start-and-ai-setup.web.e2e.ts`, ca e Solution và f thẻ phân loại), xanh trên SPA web qua cấu hình tạm (xem ghi chú hạ tầng trong TASK-FE-E2E-009). Mục "Playwright (mock) cho banner" nay có thể đánh `[x]` mức mock; kiểm trên stack thật vẫn chưa.
