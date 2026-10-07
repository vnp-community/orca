# AG-CV-TASK-006-01: Test giữ lõi codeintel trung lập truyền tải

**From Solution:** [AG-CV-SOL-006-relay-ssh-part-b-handlers](../solutions/AG-CV-SOL-006-relay-ssh-part-b-handlers.md) mục 2.1
**Priority:** P2
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-transport-neutrality.test.ts` (mới)
**Depends on:** [AG-CV-TASK-001-09](./AG-CV-TASK-001-09-codeintel-status-method-table-and-dispatcher.md) (và các CR 002–005 khi chúng xong)
**Status:** [x] DONE

## Context
Part B không có `ws`/`WireState`/`makeError`/`makeNotifier`; sao lõi sang `desktop/` không được kéo phụ thuộc vòng (CR-006 2.1).

## Việc cần làm
1. Test quét mọi `agent/src/relay/{codeintel-*,gitnexus-*,codegraph-*}.ts` (trừ `agent-rpc-dispatch-codeintel.ts`, `codeintel-relay-handlers.ts`, file `*.test.ts`): không import `ws`, `orca-dev-agent-transport`, `./agent-rpc-dispatch`, `./dispatcher`, `agent-git-handler-extended`; không `console.*`, `process.stdout.write`.
2. Liệt kê import tương đối ngoài nhóm để biết cần sao gì sang `desktop/` (xuất danh sách vào thông báo test khi lỗi).

## Kiểm thử
Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-transport-neutrality.test.ts`

## Tiêu chí hoàn thành
- [x] Test xanh trên 001–005; vi phạm cố ý làm test đỏ.

## Rủi ro
- Quét nguồn bằng regex có thể bỏ sót import động; thêm kiểm `import(`.
