# BUG-BE-HLD-008 — Chọn AI provider theo từng workflow step: 0% code dù là tính năng trung tâm của F36

**Mức độ:** 🟠 HIGH (Feature gap)
**Status:** 🔴 Open
**Module:** `backend/src/main/workflow/WorkflowTypes.ts`, `StepExecutors.ts`, `WorkflowOrchestrator.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.15/F36)

---

## Mô tả

`docs/features/F36-multi-server-workflow-orchestration.md` minh hoạ (ví dụ YAML) mix nhiều AI provider giữa các bước trong cùng 1 workflow (Claude ở bước 1, GPT-4o ở bước 2), qua cú pháp `provider: { account, model }` per step.

Code thực tế:
- `WorkflowStepConfig` (`backend/src/main/workflow/WorkflowTypes.ts:19-27`) chỉ là bag opaque `[key: string]: unknown`.
- Agent step handler (`StepExecutors.ts:86-103`) chỉ đọc `prompt`, `worktreePath`, `trustPreset` — **không đọc `provider`/`model`/`account` field nào**.
- `WorkflowOrchestrator` và `StepExecutors` **không import** `AIProviderService` hay `ProviderResolver` ở bất kỳ đâu (xác nhận bằng grep — 0 kết quả).
- `server-bootstrap.ts` khởi tạo `aiProviderService`/`providerResolver` và `workflowOrchestrator` hoàn toàn độc lập, không truyền cross-reference nào giữa 2 domain.

## Hậu quả

- Tính năng "mix AI provider theo từng bước workflow" — một trong những use-case chính được minh hoạ trong F36 — **hoàn toàn không hoạt động**. Mọi step `agent` trong 1 workflow execution đều dùng cùng 1 provider mặc định (theo cấu hình global/project), không thể override per-step.

## Bằng chứng

- `backend/src/main/workflow/WorkflowTypes.ts:19-27` — `WorkflowStepConfig` không có field `provider`.
- `backend/src/main/workflow/StepExecutors.ts:86-103` — agent step chỉ dùng `prompt`/`worktreePath`/`trustPreset`.
- Grep `AIProviderService`/`ProviderResolver` trong `backend/src/main/workflow/*.ts`: 0 kết quả.
- `backend/src/main/server-bootstrap.ts:428-464` (AI Provider) vs dòng khởi tạo Workflow — không cross-reference.

## Đề xuất fix

1. Thêm field `provider?: {accountId, model}` vào `WorkflowStepConfig` schema.
2. Wire `ProviderResolver` vào `StepExecutors` (agent step handler) — resolve provider theo step config trước khi spawn agent, fallback về provider mặc định của project nếu step không chỉ định.
3. Cập nhật `TemplateResolver` merge logic để field `provider` override đúng theo step id khi có template inheritance.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.15 (F36), §6 mục 6 (Top 10)
- Doc gốc: `docs/features/F36-multi-server-workflow-orchestration.md`
- Liên quan: [BUG-BE-HLD-009](./BUG-BE-HLD-009-workflow-pause-resume-not-implemented.md)
