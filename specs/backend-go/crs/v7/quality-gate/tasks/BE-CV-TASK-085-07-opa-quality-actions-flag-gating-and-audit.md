# BE-CV-TASK-085-07: OPA `quality_read|quality_waive|quality_profile_write`, cổng cờ chất lượng, audit

**From Solution:** BE-CV-SOL-085-quality-gate-evaluator-and-profiles
**Priority:** P0
**Service:** `policy/orca-authz`, `code-intel-service`
**File:** `backend-go/policy/orca-authz/code_intel.rego`, `code_intel_test.rego` (do BE-CV-SOL-013 tạo; task này **thêm**); `internal/adapter/grpc/quality_gate_guard.go` (mới)
**Depends on:** BE-CV-SOL-013-authorization-flags-and-audit
**Status:** [x] DONE

## Context
Bảng quyền: CR-085 §2.9, hợp đồng §6.3. Hiện `policy/orca-authz/` **không** có `code_intel.rego` (đã `ls`); không tạo file song song với SOL-013. Chuỗi: cờ → OPA → selector → … (hợp đồng §3).

## Việc cần làm
1. Rego: `quality_read` (owner, member, admin), `quality_waive` (owner, member, admin; ràng buộc hạn/`check`/`error` kiểm ở use case), `quality_profile_write` (owner, admin).
2. `quality_gate_guard.go`: interceptor/bọc gọi cổng cờ của SOL-013: `code_intel` tắt ⇒ `CODEINTEL_DISABLED`; `quality_gate` tắt ⇒ `CODEINTEL_QUALITY_GATE_DISABLED`; lỗi đọc cờ ⇒ tắt (fail closed); áp cho mọi RPC `QualityGateService` (kể cả của 089/090/092/093).
3. Ánh xạ lỗi: không quyền dự án ⇒ `CODEINTEL_NOT_AUTHORIZED`; hạ tầng ⇒ `CODEINTEL_AUTHZ_UNAVAILABLE`.

## Kiểm thử
- `opa test policy/orca-authz/` (bảng vai trò); Go: cờ tắt từng tầng cho mỗi RPC; `member` gọi `SaveQualityProfile` bị từ chối.

## Tiêu chí hoàn thành
- [x] `opa test` xanh; [ ] mọi RPC quality qua guard (test phản chiếu danh sách method); [ ] cờ cache ≤ 5 s.

## Rủi ro
- Phụ thuộc cứng SOL-013; nếu chưa merge thì task chỉ viết được ở nhánh sau nó.
