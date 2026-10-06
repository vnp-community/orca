# BE-CV-TASK-038-12: Detector `sql.*` cắm vào `ListFindings`, chế độ thử nghiệm và cổng bộ vàng

**From Solution:** BE-CV-SOL-038-static-tenant-filter-rule
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/tenant_filter_detector.go`, `tenant_filter_detector_test.go`; `internal/config/config.go` (sửa: `SQLTenantRuleMode`); `cmd/server/main.go` (sửa: đăng ký bộ); `internal/usecase/tenant_filter_golden_test.go` (mới)
**Depends on:** BE-CV-TASK-038-09, 038-10, 038-11; BE-CV-TASK-037-06 (`Detector`); BE-CV-SOL-031-erd-model-and-access-scan, BE-CV-SOL-030
**Status:** [ ] TODO

---

## Context

Solution 2.D–2.E. Bộ chỉ chạy khi mode cho phép và (ở `experimental`) khi `rules[]` nêu tường minh. Biến môi trường là **đề xuất** chưa có trong hợp đồng §6.2 (L3).

## Việc cần làm

1. `config.go`: `SQLTenantRuleMode` đọc `CODEINTEL_SQL_TENANT_RULE_MODE` (`off|experimental|on`, mặc định `experimental`; giá trị lạ ⇒ `off` và log cảnh báo, fail-safe); ghi vào PR yêu cầu chủ hợp đồng thêm vào §6.2.
2. `tenant_filter_detector.go`: cài `Detector{Name:"sql-tenant", Rules:[sql.missing-tenant-filter, sql.insert-missing-tenant], Experimental: mode==experimental}`; `Detect`: liệt kê `backend-go/services/*/internal/adapter/{postgres,mysql}/*.go` (không `_test.go`) qua `RepoSourceReader` — `scope=CHANGED` ⇒ chỉ tệp trong `ChangedFiles`; `ALL` ⇒ toàn bộ (job nền, hạn mức đồng thời CR-013, cache `(repo, commit, "sql-tenant")`); `TenantTables` từ `ErdTable.tenantScoped` của đúng service/dialect (thiếu ERD ⇒ `DetectorStatus{state:"skipped", code:"erd_unavailable"}`); `ServiceSetsTenantGUC` theo service; `Analyze` → `Evaluate` → `Tier` → `ToFinding`; `origin:"introduced"` khi `finding_key` có ở head mà không ở base (base qua `ReadFile(commit mergeBase)` cho các tệp đã đổi).
3. `mode=off` ⇒ bộ không đăng ký (rule không có trong `rules` hợp lệ).
4. Metric `codeintel_sql_tenant_candidates_total{tier}`, `codeintel_sql_tenant_skipped_dynamic_total` (tên đề xuất; `BE-CV-SOL-071` chốt).
5. `tenant_filter_golden_test.go`: đọc `testdata/sqltenant/golden/queries.yaml` (đã gán nhãn), chạy `Evaluate`+`Tier`, tính precision/recall theo tầng, **ghi ra** `testdata/sqltenant/golden/report.json`; test **không** fail theo ngưỡng (ngưỡng là quyết định vận hành); thêm `TestPrecisionGateDocumented` chỉ kiểm file `README.txt` nêu ngưỡng đã chốt.

## Kiểm thử

- `go test ./services/code-intel-service/internal/usecase/ -run TenantFilter -race`: mode `off`/`experimental`/`on`; `experimental` + `rules` rỗng ⇒ bộ không chạy; `rules=[sql.missing-tenant-filter]` ⇒ chạy; thiếu ERD ⇒ `skipped`; `scope=CHANGED` chỉ đọc tệp đã đổi (cổng giả ghi yêu cầu, không `.env`); `origin` đúng; **cô lập tenant**: hai tenant, cache `sql-tenant` riêng; quyền sai không chạm cổng.
- Dismiss đầu-cuối với `ListFindings`: dismiss một `finding_key` `sql.*` ⇒ ẩn.
- Chạy bộ vàng và ghi precision/recall vào mô tả PR.

## Tiêu chí hoàn thành

- [ ] Toàn bộ §9 solution 038-static-tenant-filter-rule đạt trên fixture.
- [ ] Mặc định `experimental` đúng; giá trị biến lạ ⇒ `off`.
- [ ] Precision/recall đã ghi; quyết định chuyển `on` để cho chủ sản phẩm.

## Rủi ro và lưu ý

- Không bật `on` trước khi có số đo bộ vàng và duyệt chủ hợp đồng về biến môi trường.
- `ALL` qua SSH tốn nhiều lệnh đọc; đo ở `BE-CV-SOL-071`.
