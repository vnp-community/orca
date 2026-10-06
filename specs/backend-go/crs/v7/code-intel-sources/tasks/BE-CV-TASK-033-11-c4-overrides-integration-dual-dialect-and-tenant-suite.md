# BE-CV-TASK-033-11: Tích hợp hai dialect, cô lập tenant và golden C4

**From Solution:** BE-CV-SOL-033-c4-overrides-yaml
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/postgres/c4_overrides_integration_test.go`, `backend-go/services/code-intel-service/internal/adapter/mysql/c4_overrides_integration_test.go` (mới, `//go:build integration`); `backend-go/services/code-intel-service/internal/usecase/get_architecture_golden_test.go` (mới); `.../testdata/golden/c4/*.json`
**Depends on:** BE-CV-TASK-033-06, -10; repository SOL-011
**Status:** [ ] TODO

---

## Context

Chốt §8.3-3/4 cho cả hai nửa CR-033.

## Việc cần làm

1. Integration (testcontainers, từng dialect): tạo/CAS/UNIQUE/đọc `c4_overrides`; xung đột đồng thời hai ghi.
2. Cô lập tenant: Postgres role `NOSUPERUSER NOBYPASSRLS`; MySQL kiểm mọi truy vấn có `tenant_id`.
3. Golden view `infra-fleet-service` (+ `usage-service`) với và không có `c4.yaml` mẫu (CR-070).
4. Ghi chi phí đọc thật (số file, byte, thời gian với agent giả trễ 100 ms) vào PR.

## Kiểm thử

`go test -tags=integration ./services/code-intel-service/... -run C4` (chưa chạy); CI ma trận `dialect: [postgres, mysql]`.

## Tiêu chí hoàn thành

- [ ] Hai dialect xanh; cô lập tenant có test.
- [ ] Golden ổn định.

## Rủi ro và lưu ý

- Golden phụ thuộc commit cố định; dùng corpus sao chép.
