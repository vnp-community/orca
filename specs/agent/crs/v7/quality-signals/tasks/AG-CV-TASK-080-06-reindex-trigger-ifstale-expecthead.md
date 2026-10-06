# AG-CV-TASK-080-06: `codeintel.reindex`: `trigger`, `ifStale`, `expectHead`, `skipped_scope_repo_root`

**From Solution:** [AG-CV-SOL-080-index-basis-and-reindex-triggers](../solutions/AG-CV-SOL-080-index-basis-and-reindex-triggers.md) mục 5.6
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-reindex-job.ts` (sửa; AG-CV-SOL-004), `agent/src/relay/codeintel-reindex-job.test.ts` (sửa)
**Depends on:** AG-CV-TASK-080-02, 080-03; AG-CV-SOL-004 (tạo file)
**Status:** [ ] TODO

## Context

PQ-16 và hợp đồng §4.10. `analyze` luôn `--index-only` (đã có ở `codeintel-reindex-commands.ts`, không đổi ở task này). Không có `tiers`.

## Việc cần làm

1. Mở rộng `validate`: `trigger` (`manual|agent_done|head_change`, mặc định `manual`), `ifStale` (boolean), `expectHead` (`^[0-9a-f]{7,64}$`); mọi khoá ngoài `{workspaceRoot, mode, tools, trigger, ifStale, expectHead, _trace}` → `CODEINTEL_INVALID_PARAMS` (kể cả `tiers`).
2. Thứ tự quyết định trước khi spawn: (a) `expectHead` có và HEAD hiện tại khác → trả `outcome:"superseded"`; (b) worktree liên kết: `manual` → `CODEINTEL_PATH_NOT_ALLOWED hint="reindex_linked_worktree_unsupported"`, còn lại → `outcome:"skipped_scope_repo_root"` + `skipped`; (c) `ifStale` ∧ mọi công cụ đã chọn có `freshness==="fresh"` → `outcome:"already_up_to_date"`; (d) còn lại như SOL-004.
3. Với (a)(b)(c) trả đối tượng `{jobId, state:"succeeded", outcome, skipped, …}` ngay, **không** gọi `spawn`, **không** xếp hàng job, **không** chiếm cổng nặng.
4. `trigger` ghi vào journal job và truyền cho `indexChanged` (task 07).

## Kiểm thử

Mở rộng `codeintel-reindex-job.test.ts`: mỗi nhánh (a)(b)(c) khẳng định `spawn` giả không được gọi; `tiers` → INVALID_PARAMS; `manual` ở worktree liên kết → PATH_NOT_ALLOWED; `agent_done` ở worktree liên kết → thành công không lỗi; `head_change` giống `agent_done`; test snapshot argv `--index-only` của SOL-004 vẫn xanh. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-reindex-job.test.ts src/relay/codeintel-reindex-commands.test.ts`.

## Tiêu chí hoàn thành

- [ ] 3 nhánh sớm không spawn; thứ tự quyết định đúng như mục 2.
- [ ] Không có `tiers` trong schema (test phản chiếu).

## Rủi ro

- `ifStale` dựa `freshness` tính tại thời điểm gọi (cache 5 s của HEAD): hai lần gọi liên tiếp có thể thấy giá trị cũ; chấp nhận.
