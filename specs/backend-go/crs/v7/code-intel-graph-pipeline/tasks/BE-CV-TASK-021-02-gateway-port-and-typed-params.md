# BE-CV-TASK-021-02: Cổng `AgentCodeIntelGateway`, `AgentRPCCaller`, tham số kiểu hẹp, `AgentStatusData`

**From Solution:** BE-CV-SOL-021-agent-collector
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/agent_code_intel_gateway.go`, `.../domain/agent_status.go` (mới) và test
**Depends on:** TASK-021-01, BE-CV-TASK-020-04
**Status:** [ ] TODO

---

## Context

H3/D5: không có đường truyền lệnh/`args` tự do. Tham số phải là struct kiểu hẹp. `AgentStatusData` theo agent contract §4.1 dùng chung SOL-012/022.

## Việc cần làm

1. `AgentTarget{TenantID, DevServerID, WorkspaceRoot, HostPlatform}`, `RawCodeIntelResult{Sources, HeadCommit, Stale, Truncated, TotalCount *int64, Warnings, Perf, Data json.RawMessage}`.
2. `AgentRPCCaller` (`Call(ctx, target, method, params map[string]any)`) và `AgentCodeIntelGateway` 16 method (status, overview, processes, process, subgraph, impact, symbol, routes, detectChanges, structuralFacts, reindex, reindexStatus, reindexCancel, watch, codegraphSearch, files).
3. Struct tham số có `Validate()` theo biên agent contract §4 (ví dụ `depth 1..3`, `limit ≤ 1500`, chuỗi không bắt đầu `-`, đường dẫn tương đối không `..`); `toParams()` luôn thêm `workspaceRoot`.
4. `domain/agent_status.go`: kiểu `AgentStatusData` (binding, tools, indexes.gitnexus/codegraph, limits...) với `rootMismatch` và `worktreeMismatch` theo PQ-19.
5. Nếu SOL-012 đã có kiểu tương đương thì dùng lại, không nhân đôi.

## Kiểm thử

- `go test ./internal/usecase/ -run 'AgentParams|AgentStatus' -race`.
- Test phản chiếu: không struct tham số nào có field tên `args|argv|command|cmd|cwd|env|repo|cypher|shell|timeout|tool`.
- Ca biên: `depth=4`, `limit=1501`, `-x`, `../a` → `CODEINTEL_INVALID_PARAMS`.

## Tiêu chí hoàn thành

- [ ] 16 method có tham số kiểu hẹp và test biên.
- [ ] `toParams()` luôn có `workspaceRoot`.
- [ ] Không file tên `helpers/utils/common/misc`.

## Rủi ro và lưu ý

- `codegraphSearch`/`files` phát hành đợt 4 (P1): chấp nhận `-32601` (CODEINTEL_AGENT_UNSUPPORTED).
- Hình `status` chưa chạy thật (chỉ theo hợp đồng).
