# BE-CV-TASK-085-05: Repository profile hai dialect và use case `SaveQualityProfile`/`GetQualityProfile`

**From Solution:** BE-CV-SOL-085-quality-gate-evaluator-and-profiles
**Priority:** P0
**Service:** `code-intel-service`
**File:** `internal/adapter/{postgres,mysql}/quality_profile_repository.go`, `internal/usecase/{get,save}_quality_profile.go`, `quality_gate_ports.go` (mới)
**Depends on:** BE-CV-TASK-085-01, 085-03; BE-CV-SOL-011-repositories-and-maintenance (`withTenantTx`, TxRunner)
**Status:** [x] DONE

## Context
Mẫu `withTenantTx`: `mcp-service/internal/adapter/postgres/tenant_tx.go`. MySQL: `WHERE tenant_id = ?`, không `RETURNING` (`dbcapability.SupportsReturning=false`).

## Việc cần làm
1. Repo: `GetByScope(tenant, scopeKey, name)`, `Create`, `UpdateCAS(id, tenant, expectedVersion)`; 0 hàng ⇒ phân biệt `NOT_FOUND`/`VERSION_CONFLICT` (đọc lại cùng transaction).
2. `GetQualityProfile`: `repo:<id>` > `tenant` > builtin (không trộn trường); trả `origin`, `version` (builtin 0), `runnable_profiles` qua port `RunnableProfileLister` (lọc `security-*`/`dependency-diff` khi `quality_security_scan_enabled`=false; lỗi nguồn ⇒ `[]`).
3. `SaveQualityProfile`: OPA `quality_profile_write` (kiểm ở 085-07), decode (085-03), `expected_version=0`=tạo, `warnings[]` khi `checks[].profile` ∉ runnable; audit `codeintel.quality.profile.save` (không `reason`).
4. Test AST kiểu `tenant_scope_guard_test.go` cho repository mới.

## Kiểm thử
- Integration hai dialect: CAS (hai goroutine), unique, UTF-8/`\u0000`, cách ly tenant; use case với port giả.

## Tiêu chí hoàn thành
- [x] lưu/đọc/CAS đúng hai dialect; [ ] `mode=block` bị từ chối; [ ] tenant B không thấy profile tenant A.

## Rủi ro
- `RunnableProfileLister` chưa có chủ (Q4 SOL-085-evaluator): thiếu ⇒ `warnings[]` rỗng/không kiểm.
