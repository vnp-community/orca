# BE-CV-TASK-085-13: Test cách ly tenant, fixture hợp đồng và ma trận hai dialect của CR-085

**From Solution:** BE-CV-SOL-085-waivers-and-trend
**Priority:** P1
**Service:** `code-intel-service`, CI
**File:** `internal/adapter/{postgres,mysql}/*_scope_guard_test.go`, `testdata/quality-gate/*.json` (mới), workflow CI service (sửa)
**Depends on:** BE-CV-TASK-085-05, 085-08, 085-12
**Status:** [ ] TODO

## Việc cần làm
1. Test AST kiểu `tenant_scope_guard_test.go` cho 3 repository mới (MySQL: mọi truy vấn có `tenant_id`).
2. Test tích hợp: tenant A/B cho profile, waiver, trend (Postgres role `NOBYPASSRLS`).
3. Fixture JSON `QualityGate`, `QualityProfile`, `QualityWaiver`, `QualityTrendPoint` (có/không trường tuỳ chọn) đặt cạnh fixture CR-CV-070 để FE đối chiếu.
4. CI: ma trận `dialect: [postgres, mysql]` chạy `-tags=integration`; `opa test`; `buf lint`+`buf breaking` trực tiếp.

## Kiểm thử / Tiêu chí hoàn thành
- [ ] guard bắt được phương thức quên `tenant_id` (thử cố ý); [ ] fixture khớp ui-api §4.7; [ ] CI hai dialect xanh.

## Rủi ro
- Dev compose dùng superuser: RLS không hiệu lực ở dev; test phải dùng role riêng.
