# BE-CV-TASK-082-05: Repository MySQL cho run và finding

**From Solution:** BE-CV-SOL-082
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/mysql/quality_run_repository.go`, `quality_finding_repository.go` (mới)
**Depends on:** BE-CV-TASK-082-02, 082-03
**Status:** [x] DONE

## Context
MySQL không RLS (`SupportsRLS=false`): `WHERE tenant_id = ?` ở mọi câu. Vi phạm duy nhất `1062`. Không `RETURNING`.

## Việc cần làm
1. Cài cùng cổng như 082-04; `InsertBatch` dùng `ON DUPLICATE KEY UPDATE id=id`.
2. `Finish` CAS bằng `UPDATE … WHERE id=? AND tenant_id=? AND version=?` kiểm `RowsAffected`.
3. Dọn `DELETE … ORDER BY created_at LIMIT n`.
4. CHECK có thể bị bỏ qua (MySQL < 8.0.16): domain đã kiểm.

## Kiểm thử
- Bộ hợp đồng 082-06 trên MySQL; test AST bắt phương thức thiếu `tenant_id`.

## Tiêu chí hoàn thành
- [x] Cùng kết quả như Postgres trên bộ kịch bản chung.

## Rủi ro
TiDB chưa kiểm; `ON DUPLICATE KEY` chưa chạy.
