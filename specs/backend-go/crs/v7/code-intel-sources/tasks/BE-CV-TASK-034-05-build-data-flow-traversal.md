# BE-CV-TASK-034-05: Bộ duyệt dựng `DataFlow` (server-side expansion, chọn adapter theo dialect, kho/RPC/agent/event)

**From Solution:** BE-CV-SOL-034-data-flow-model
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/build_data_flow.go` (mới), `.../usecase/component_ref_assignment.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-034-03, -04; BE-CV-TASK-032-07; BE-CV-TASK-033-05; BE-CV-TASK-031-10
**Status:** [ ] TODO

---

## Context

Solution 2.C.

## Việc cần làm

1. Bước 0 và đích `WsTarget` (rpc/agent-method/local).
2. Mở rộng server: `Unimplemented` → gap; handler → use case → cổng → adapter; chọn theo `dialect` (mặc định `postgres`), `MULTIPLE_IMPLEMENTATIONS` khi tham số rỗng mà có hai; không có adapter → `UNRESOLVED_CLIENT`.
3. Tại adapter: `accessedBy` → `StoreAccess`+bước `db-*`; `RpcEdge` → đệ quy; `Exec` → `ext-agent` (`AGENT_METHOD_DYNAMIC`); `Publish/OutboxWriter` → bước `event`.
4. Giới hạn hop/step, `CYCLE`, `DEPTH_LIMIT`; `completeness`.
5. `component_ref_assignment.go`: `filePath` → component (hàm của 033), `ui` cho actor; `ext-*` id theo 033.
6. `origin`, `confidence`, `evidence` mỗi bước.

## Kiểm thử

`go test ... -run BuildDataFlow` (chưa chạy): `mini-flow` đủ chuỗi 5 bước; `dialect=mysql` đổi adapter; `Unimplemented`; vòng `CYCLE`; vượt hop; hai tenant không chéo cache.

## Tiêu chí hoàn thành

- [ ] Ca `accounts.selectClaude` trên fixture đúng 5 bước + `StoreAccess dev_servers read`.
- [ ] Ma trận hai dialect.

## Rủi ro và lưu ý

- Nhánh/điều kiện trong use case làm `partial`; không đoán nhánh.
