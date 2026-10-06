# BE-CV-TASK-031-12: Golden ERD, ma trận hai dialect, cô lập tenant và đối chiếu DB thật ở CI

**From Solution:** BE-CV-SOL-031-erd-model-and-access-scan
**Priority:** P1
**Service:** `code-intel-service` / CI
**File:** `backend-go/services/code-intel-service/internal/usecase/build_erd_golden_test.go` (mới), `.../usecase/testdata/golden/erd/*.json` (mới), `.../adapter/sqlmigration/live_database_crosscheck_test.go` (mới, `//go:build integration`), `.github/workflows/backend-go-code-intel-service.yml` (sửa: thêm ca; file do BE-CV-SOL-010 tạo)
**Depends on:** BE-CV-TASK-031-10
**Status:** [ ] TODO

---

## Context

Chốt tiêu chí chấp nhận cuối của CR-031: golden theo CR-CV-070, ma trận CI `dialect: [postgres, mysql]` (§8.3-3), cô lập tenant (§8.3-4) và phương án D của CR (dựng DB thật bằng testcontainers, đọc `information_schema`, so với `ErdModel`) **chỉ ở CI**.

## Việc cần làm

1. Golden: snapshot `ErdModel` JSON (không `warnings` động) cho `infra-fleet-service` (postgres + mysql), `mcp-service` (postgres), `task-service` (cả hai) từ **commit cố định** trong repo mẫu hoặc corpus sao chép; cờ `-update` để cập nhật có chủ đích; kèm một `erd-links.yaml` mẫu.
2. Ma trận dialect: test use case chạy cho cả hai dialect với cùng fixture; sửa workflow để job có `matrix.dialect` và chạy `go test -tags=integration` cho phần cần DB (cache snapshot).
3. `live_database_crosscheck_test.go` (`integration`): với testcontainers (`testcontainers-go v0.44.0` có trong `common/go.mod`) khởi Postgres/MySQL, áp migration của **một** service nhỏ (ví dụ `usage-service`) bằng `golang-migrate` hoặc chạy tuần tự `.up.sql`, đọc `information_schema.columns` và so với `ErdModel` (tên bảng, tên cột, nullable, PK); lệch → test đỏ. Không chạy ở PR thường nếu quá chậm: gắn nhãn `crosscheck` và job `workflow_dispatch` + hằng đêm.
4. Cô lập tenant: test repository snapshot (từ BE-CV-SOL-011) cho view `erd` với hai tenant (Postgres: role `NOSUPERUSER NOBYPASSRLS`; MySQL: kiểm mọi truy vấn có `tenant_id` bằng test AST của SOL-011).
5. Hiệu năng: `BenchmarkBuildErd_InfraFleet` với agent giả trễ 100 ms; ghi số đo thật vào PR (mục tiêu < 10 s lạnh, < 1 s nóng là mục tiêu, chưa đo).
6. Đối chiếu số liệu: thống kê tính năng thật so với bảng CR 1.2, ghi vào `INVENTORY.md` (BE-CV-TASK-031-01).

## Kiểm thử

- `cd backend-go && go test ./services/code-intel-service/... ` và `go test -tags=integration ./services/code-intel-service/...` (chưa chạy); lệnh golden: `go test ./services/code-intel-service/internal/usecase -run Golden -update`.
- CI: `act`/GitHub Actions matrix hai dialect xanh.

## Tiêu chí hoàn thành

- [ ] Golden ba service, hai dialect, ổn định giữa hai lần chạy.
- [ ] Ma trận CI hai dialect xanh; job crosscheck chạy được thủ công.
- [ ] Test cô lập tenant có cả hai dialect.

## Rủi ro và lưu ý

- Golden dễ vỡ khi migration nguồn thêm file; dùng corpus sao chép + một job riêng quét repo thật để cảnh báo (không chặn PR).
- Testcontainers cần Docker trong CI; job crosscheck có thể bị bỏ qua ở môi trường không có Docker.
