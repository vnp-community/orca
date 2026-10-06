# FE-CV-TASK-089-05: View-model đối chiếu lượt agent

**From Solution:** [FE-CV-SOL-089-agent-turn-recorder](../solutions/FE-CV-SOL-089-agent-turn-recorder.md) mục 2.4
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/turns/agent-turn-verification-view-model.ts` (mới) + test
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge
**Status:** [ ] TODO

## Context

- `AgentTurn.claims.items[].agreement` bốn giá trị; enum lạ → `unknown`.
- Không quy kết: cấm "nói dối", "giả mạo", "đánh lừa".

## Việc cần làm

1. `ranLine` từ `commandsSummary` ("Agent đã chạy: test (3 lần), lint").
2. Ánh xạ agreement → khoá chữ trung lập; `unverified` không thành `consistent`; `basis:"stated"` nhãn "Suy luận từ lời agent".

## Kiểm thử

- Bảng ca bốn agreement + lạ + không claims.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không chuỗi cấm.
- [ ] Hàm thuần.

## Rủi ro

- `ran_command` chỉ chứng minh đã chạy, không chứng minh đạt.
