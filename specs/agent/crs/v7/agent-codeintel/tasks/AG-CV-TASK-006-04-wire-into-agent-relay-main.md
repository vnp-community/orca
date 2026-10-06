# AG-CV-TASK-006-04: Nối `registerCodeIntelHandlers` vào `relay.ts` (cây `agent/`)

**From Solution:** [AG-CV-SOL-006-relay-ssh-part-b-handlers](../solutions/AG-CV-SOL-006-relay-ssh-part-b-handlers.md) mục 2.1
**Priority:** P2
**Area:** `agent/`
**File:** `agent/src/relay/relay.ts` (sửa 1 dòng + import)
**Depends on:** [003](./AG-CV-TASK-006-03-dispatcher-forward-error-data-agent-tree.md)
**Status:** [ ] TODO

## Context
`relay.ts:493` `registerAuthStatusHandlers(dispatcher, authStatusConfig, authStatusLogger)`; config và logger (`relayLogLine`) đã dựng ngay trên. `relay.ts` 1 114 dòng: không thêm `max-lines` disable.

## Việc cần làm
1. Impact `main` (relay) trước khi sửa.
2. Ngay sau dòng `:493`: `await registerCodeIntelHandlers(...)` hoặc import động bọc `try/catch` (lỗi nạp -> `relayLogLine`, không phá khởi động).
3. Không thêm trường vào `relay.status` (`:715`).

## Kiểm thử
Test hiện có của `relay.ts` (nếu có) không hồi quy; test tích hợp nhỏ: dựng `RelayDispatcher` như `main()` và gọi `codeintel.status`.
Lệnh: `pnpm exec vitest run src/relay/codeintel-relay-handlers.test.ts`; `pnpm test`.

## Tiêu chí hoàn thành
- [ ] Đúng một dòng gọi; `relay.status` không đổi.

## Rủi ro
- `loadAgentConfig()` ở chế độ socket của `relay.ts` chưa kiểm chứng.
