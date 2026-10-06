# BE-CV-TASK-092-06: Repository `requirement_trace_links` và use case `Confirm`/`Link`

**From Solution:** BE-CV-SOL-092-requirement-trace
**Priority:** P2
**Service:** `code-intel-service`
**File:** `internal/adapter/{postgres,mysql}/requirement_trace_link_repository.go`, `internal/usecase/{confirm_requirement_evidence,link_worktree_task}.go` (mới)
**Depends on:** BE-CV-TASK-092-01, 092-05
**Status:** [ ] TODO

## Việc cần làm
1. Repo: `Upsert` (PG `ON CONFLICT … DO UPDATE`; MySQL `ON DUPLICATE KEY UPDATE`), `Delete`, `ListByScope`; `evidence_ref` > 255 ⇒ `sha256:<hex>`.
2. `ConfirmRequirementEvidence`: `CONFIRM`/`REJECT` trong một transaction xoá dòng đối lập cùng khoá; `scope` mặc định `worktree:<binding>`.
3. `LinkWorktreeTask`: `ResolvePermission` + cùng project; rỗng ⇒ xoá `worktree_task`.
4. Audit `codeintel.trace.confirm`; trả `trace` tính lại.
5. Test AST `tenant_id`.

## Kiểm thử
- Integration hai dialect: upsert, thay `reject`↔`confirm`, cách ly tenant; kích thước body > 8 KiB bị gateway chặn (kiểm ở 040, không ở đây).

## Tiêu chí hoàn thành
- [ ] một khoá một trạng thái; [ ] xoá worktree không xoá xác nhận mức `repo`.

## Rủi ro
- Quyền `review_write` cho `LinkWorktreeTask` chưa phân biệt người sở hữu task.
