# AG-CV-TASK-006-07: Kiểm thử `codeintel.*` qua `--stdio`/`--detach` (Part A, đường Go)

**From Solution:** [AG-CV-SOL-006-relay-ssh-part-b-handlers](../solutions/AG-CV-SOL-006-relay-ssh-part-b-handlers.md) mục 2.4
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/agent-connection-stdio-codeintel.test.ts` (mới); ghi cập nhật `specs/agent/api/agent-rpc-catalog-runtime.md` cột Part A/B (chỉ ghi vào PR, người điều phối sửa)
**Depends on:** [AG-CV-TASK-001-09](./AG-CV-TASK-001-09-codeintel-status-method-table-and-dispatcher.md) (độc lập với O-5)
**Status:** [x] DONE

## Context
Go `relay-ssh` chạy `node agent.js --stdio|--detach|--connect` = cùng `createSession` Part A (`agent-entry.ts:80-108`, `agent-connection-stdio.ts:262-280`) nên method mới tự có; cần chứng minh. Daemon `--detach` tạo `createSession` mới mỗi kết nối socket; Map module sống qua kết nối.

## Việc cần làm
1. Test gửi khung `codeintel.status` qua `StdioWebSocketAdapter` (khuôn `agent-connection-stdio.test.ts`; **chưa kiểm** tên/cách dựng) và nhận kết quả; handshake có `codeintel*`.
2. Test `runDetachedStdioMode` nếu khả thi (`ORCA_RELAY_DETACHED_CHILD=1`; tiến trình thật chậm).
3. Ca hai phiên đồng thời: thông báo chỉ tới phiên gọi gần nhất (ghi hạn chế, Q4).
4. Xác nhận không có `console.log` từ `codeintel-*` làm hỏng khung stdout.

## Kiểm thử
Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/agent-connection-stdio-codeintel.test.ts src/relay/agent-connection-stdio.test.ts`

## Tiêu chí hoàn thành
- [x] `codeintel.status` đúng qua stdio; stdout chỉ chứa khung.

## Rủi ro
- Chưa kiểm chứng Go dùng chế độ nào trong thực tế (`--stdio` trực tiếp hay `--detach`).
