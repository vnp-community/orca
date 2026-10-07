# FE-REQ-TASK-019-01: Mô hình dòng thời gian bước và quy tắc Request con

**From Solution:** [FE-REQ-SOL-019](../solutions/FE-REQ-SOL-019-request-list-detail-classification-ui.md) mục 2.3, 2.5
**Priority:** P0
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/request-stage-timeline-model.ts` (mới), `frontend/src/shared/request-flow-registry.ts` (sửa: thêm `CHILD_REQUEST_RULES`, `isLowConfidence`), test `request-stage-timeline-model.test.ts`
**Depends on:** FE-REQ-TASK-018-01
**Status:** [x] DONE

## Context

- Registry theo README v6 3.4 đã có ở 018-01 (`REQUEST_FLOW_REGISTRY`, `flowHasPhase`). Trạng thái theo 3.3.
- Hàm thuần, không React, để test bảng đầy đủ 11 loại.
- Loại con/`reason` suy từ README 3.4 (đường nâng cấp) và CR-016 (`request.spawnChild.reason ∈ request_links.reason`); CR-REQ-006 chưa chốt bảng.

## Việc cần làm

1. `StepId = 'classification'|'analysis'|'plan'|'phase'|'execution'`; `Step {id, labelKey, state:'done'|'current'|'pending'|'skipped', variant?}`.
2. `buildStageTimeline({type, size, status, returnedFromStage})`: bước `analysis` có `variant` = `analysisKind` (`solution|diagnosis|findings|answer`); bước `plan` có `variant` `plan|task_list|single_task`; chỉ thêm `phase` khi `flowHasPhase(type,size)`; loại không có analysis/plan thì không vẽ bước đó. Ánh xạ status → current như SOL-019 2.3; `request_backlog` giữ bước theo `returnedFromStage` (`classification|analysis|plan|phase|task`); `cancelled` đánh dấu bước hiện tại `skipped`.
3. `note: 'hotfix_fast_diagnosis'` khi `type==='hotfix'`.
4. `type` hoặc `status` là `'unknown'`: trả `steps: []` và `current: null` (không ném).
5. `CHILD_REQUEST_RULES: Partial<Record<RequestType,{reason: RequestLinkReason, suggestedTypes: RequestType[]}>>` theo SOL-019 2.5; export `isLowConfidence(c: number|undefined): boolean` dùng `LOW_CONFIDENCE_THRESHOLD`.

## Kiểm thử

- Bảng test 11 loại: `question` không có `plan`; `bug` có `phase` chỉ khi `size='L'`; `task`/`docs` có `plan` variant `task_list`; `hotfix` có note; `change_request` luôn có `phase`.
- Mỗi trạng thái 3.3 cho `change_request` đúng `current`; `request_backlog` + `returnedFromStage='plan'` đúng bước.
- `unknown` không ném. `isLowConfidence(0.59)` true, `(0.6)` false, `(undefined)` true.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/request-stage-timeline-model`.

## Tiêu chí hoàn thành

- [ ] Không có `if (type === ...)` trong model (chỉ đọc registry).
- [ ] Test phủ 11 loại, mọi trạng thái, `unknown`.
- [ ] `tsc` sạch lỗi mới.

## Rủi ro và lưu ý

- Thêm loại mới ở backend: registry cập nhật là đủ.
- Ngưỡng 0.6 chỉ là đề xuất.
