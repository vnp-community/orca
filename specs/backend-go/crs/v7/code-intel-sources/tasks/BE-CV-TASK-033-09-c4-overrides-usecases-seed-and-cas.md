# BE-CV-TASK-033-09: Use case `GetC4Overrides`/`SaveC4Overrides` (seed repo, CAS, huỷ cache)

**From Solution:** BE-CV-SOL-033-c4-overrides-yaml
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/get_c4_overrides.go`, `save_c4_overrides.go` (mới), `.../usecase/ports.go` hoặc `c4_overrides_repository.go` (cổng, mới), `.../adapter/c4overrides/repo_seed_reader.go` (mới)
**Depends on:** BE-CV-TASK-033-07, -08; bảng `c4_overrides` và repository của BE-CV-SOL-011; BE-CV-SOL-030
**Status:** [ ] TODO

---

## Context

Solution mục 2.D.

## Việc cần làm

1. Cổng `C4OverridesRepository{Get, Create, UpdateCAS}` (cài đặt bởi SOL-011; nếu chưa, fake để test).
2. `Get`: DB > seed repo (`docs/code-intel/c4/<container>.yaml` qua `RepoSourceReader`, qua kiểm `CleanRel`) > rỗng; `seed_source` đúng.
3. `Save`: validate; `expected_version=0` tạo (UNIQUE → `VERSION_CONFLICT`); khác 0 CAS; trả `warnings` từ thử merge; audit (SOL-013); huỷ cache `architecture` của binding.
4. Khoá nghiệp vụ `repo_id` từ binding; mọi truy vấn có `tenant_id`.

## Kiểm thử

`go test ... -run C4Overrides` (chưa chạy) với repository giả; tích hợp thật ở task 11.

## Tiêu chí hoàn thành

- [ ] CAS đúng; seed không ghi đè DB.
- [ ] Huỷ cache được kiểm.

## Rủi ro và lưu ý

- `repo_id` vs `repo_binding_id`: lưu cả hai (T6), khoá theo `repo_id`.
