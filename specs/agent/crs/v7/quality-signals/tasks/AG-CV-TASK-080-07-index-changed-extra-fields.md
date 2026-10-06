# AG-CV-TASK-080-07: `codeintel.indexChanged` thêm `indexScope`, `mergeBase`, `trigger`

**From Solution:** [AG-CV-SOL-080-index-basis-and-reindex-triggers](../solutions/AG-CV-SOL-080-index-basis-and-reindex-triggers.md) mục 5.7
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-index-watcher.ts`, `agent/src/relay/codeintel-reindex-job.ts` (sửa; AG-CV-SOL-004), test tương ứng
**Depends on:** AG-CV-TASK-080-02, 080-06; AG-CV-SOL-004
**Status:** [ ] TODO

## Context

Hợp đồng §6.1: các trường `indexScope`, `mergeBase`, `trigger` là tuỳ chọn; backend dùng để cập nhật chip index và huỷ cache mà không cần gọi lại `status`. Thông báo chỉ gửi khi WS mở (bộ phát của SOL-004).

## Việc cần làm

1. Khi watcher phát hiện đổi `meta.json`/CodeGraph DB/HEAD, hoặc sau `reindex` thành công, tính `indexScope` bằng `classifyIndexBasis` (dùng probe task 03, cache 5 s) và thêm `indexScope`, `mergeBase` vào `params`.
2. `trigger` lấy từ job (task 06); sự kiện do watcher (không có job) bỏ trường này.
3. `tool:"git"` + `reason:"head"` vẫn chỉ cập nhật `stale`, không kích hoạt `analyze`.
4. Không thêm `freshness` hay đường dẫn tuyệt đối vào payload.

## Kiểm thử

Mở rộng `codeintel-index-watcher.test.ts`/`codeintel-notification-sink.test.ts`: payload đúng khoá; thiếu `mergeBase` thì khoá vắng (không `undefined` trong JSON); `reindex` có `trigger:"agent_done"` → payload mang `trigger`. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-index-watcher.test.ts src/relay/codeintel-notification-sink.test.ts`.

## Tiêu chí hoàn thành

- [ ] `JSON.stringify(params)` khớp ví dụ §6.1 (3 trường cuối tuỳ chọn).
- [ ] Không gửi thông báo khi WS đóng (hành vi SOL-004 giữ nguyên).

## Rủi ro

- Tần suất thông báo khi watcher thăm dò 10 s: không làm tăng (chỉ thêm trường).
