# BE-CV-TASK-013-02: `code_intel.rego` và `code_intel_test.rego` (8 action)

**From Solution:** BE-CV-SOL-013-authorization-flags-and-audit
**Priority:** P0
**Service:** `policy/orca-authz`
**File:** `backend-go/policy/orca-authz/code_intel.rego`, `backend-go/policy/orca-authz/code_intel_test.rego` (mới)
**Depends on:** không (làm song song từ đầu)
**Status:** [x] DONE

---

## Context

Hợp đồng §6.3: 8 action (`read`, `read_source`, `review_write`, `reindex`, `c4_write`, `quality_read`, `quality_waive`, `quality_profile_write`); `c4_write` và `quality_profile_write` chỉ owner/admin. Mẫu: `policy/orca-authz/project.rego` + `project_test.rego` (`package orca.authz.project`, `import rego.v1`, `action_roles`, `default allow := false`, admin toàn cục).

## Việc cần làm

1. `code_intel.rego`: `package orca.authz.codeintel`; bảng `action_roles` theo SOL-013-authorization mục 2.A; `default allow := false`; `allow if input.caller_global_role == "admin"`; `allow if` theo bảng. Chú thích đầu file nêu input shape và việc `GetProject` là cổng tenant.
2. `code_intel_test.rego`: mỗi action × `{owner, member, "", admin toàn cục}`; action lạ; thiếu `action`; `caller_global_role="user"` với project role rỗng → false.
3. Không đổi file `.rego` khác.

## Kiểm thử

- `cd backend-go && make opa-test` (cần CLI `opa`; `opa test policy/orca-authz/`).

## Tiêu chí hoàn thành

- [x] Toàn bộ bảng role × action đúng; `member` bị từ chối `c4_write`, `quality_profile_write`.
- [x] Test Rego xanh; không phá test của bundle khác.

## Rủi ro và lưu ý

- `quality_waive` cho `member`: ràng buộc "tối đa 7 ngày, không miễn `check`/`error`" ở use case SOL-085, không ở Rego.
- Dockerfile/compose đã mang bó `orca-authz` (SOL-010).
