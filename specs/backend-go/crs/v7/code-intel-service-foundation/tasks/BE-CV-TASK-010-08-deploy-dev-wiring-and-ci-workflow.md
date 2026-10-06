# BE-CV-TASK-010-08: Dockerfile, compose dev, init DB, `migrate.sh`, `build-local.sh`, kiểm OPA, workflow CI, README service

**From Solution:** BE-CV-SOL-010-scaffold-code-intel-service
**Priority:** P1
**Service:** `code-intel-service` · `deploy/dev` · `.github/workflows`
**File:** `backend-go/services/code-intel-service/deploy/Dockerfile` (mới), `backend-go/services/code-intel-service/README.md` (mới), `backend-go/deploy/postgres-init-databases.sh`, `deploy/dev/docker/postgres/init-databases.sh`, `deploy/dev/docker-compose.yml`, `deploy/dev/scripts/migrate.sh`, `deploy/dev/scripts/build-local.sh`, `backend-go/ci/check-opa-bundle-in-images.sh`, `.github/workflows/backend-go-code-intel-service.yml` (mới)
**Depends on:** BE-CV-TASK-010-05, 010-07
**Status:** [ ] TODO

---

## Context

Hợp đồng §10 và §6.2 (container `orca-go-code-intel`, job `migrate-codeintel`, DB `codeintel`). `deploy/` nằm ở **gốc repo**, không `backend-go/deploy` (trừ file init DB của `backend-go/docker-compose.yml`). Hai file `init-databases.sh` có danh sách khác nhau (ví dụ file gốc không có `scm`, `issuestatussync`), nên thêm vào cả hai. `x-go-common-env` đã có bốn biến địa chỉ (SOL-010 C3).

## Việc cần làm

1. `deploy/Dockerfile`: sao `task-service/deploy/Dockerfile`, đổi tên service; giữ `COPY policy ./policy` ở stage build và `COPY --from=build /src/policy/orca-authz /policy/orca-authz` ở runtime (SOL-013 dùng OPA); `COPY services/code-intel-service/migrations /migrations`.
2. Thêm `codeintel` vào biến `DATABASES` của **cả** `backend-go/deploy/postgres-init-databases.sh` và `deploy/dev/docker/postgres/init-databases.sh`.
3. `docker-compose.yml`: service `code-intel-service` (mẫu `task-service` dòng 325; `<<: *go-defaults`; `container_name: orca-go-code-intel`; `volumes: ./bin/code-intel-service/orca:/app/bin/orca:ro` và `./policy/orca-authz:/policy/orca-authz:ro`; `environment: <<: *go-common-env`; `DATABASE_DSN: postgresql://orca:${POSTGRES_PASSWORD}@postgres:5432/codeintel?sslmode=disable`; `CODEINTEL_ENABLED`, `CODEINTEL_QUALITY_GATE_ENABLED`, `CODEINTEL_AI_REVIEW_ENABLED`, `CODEINTEL_TENANT_DEFAULT_ENABLED` mặc định `false`; `CODEINTEL_INTERNAL_CALLER_TOKEN: ${CODEINTEL_INTERNAL_CALLER_TOKEN:-}`; `OPA_BUNDLE_PATH: /policy/orca-authz`; `mem_limit: 256m`, `mem_reservation: 128m`; `depends_on` postgres healthy, `project-service`/`infra-fleet-service` started). Job `migrate-codeintel` (mẫu `migrate-task` dòng 705, `./bin/code-intel-service/migrations`, DSN `…/codeintel…`).
4. `migrate.sh` dòng 27: thêm `codeintel` vào chuỗi `SERVICES`.
5. `build-local.sh`: thêm `code-intel-service` vào mảng `ALL_SERVICES` (đã thấy cơ chế làm phẳng `migrations/postgres` vào `bin/<svc>/migrations`).
6. `check-opa-bundle-in-images.sh`: thêm `code-intel-service` vào `svcs`.
7. Workflow `backend-go-code-intel-service.yml`: sao `backend-go-task-service.yml` (ma trận `dialect: [postgres, mysql]`) cộng bước `buf lint --path orca/codeintel` và `buf breaking` (mẫu `backend-go-mcp-service.yml` dòng 26–36); `paths` theo SOL-010 mục 2.I. Gọi `buf` trực tiếp.
8. `README.md`: bảng "thật / chưa làm" cho từng RPC (ban đầu: không RPC), mục "Yêu cầu RLS" (role `NOSUPERUSER NOBYPASSRLS`, compose dev dùng superuser nên RLS không hiệu lực; sao cách viết `mcp-service/README.md`), danh sách env.

## Kiểm thử

- Lệnh: `bash -n` các script đã sửa; `docker compose -f deploy/dev/docker-compose.yml config` (xác thực YAML); `deploy/dev/scripts/build-local.sh` tạo `bin/code-intel-service/{orca,migrations}`; `deploy/dev/scripts/migrate.sh codeintel` chạy xong; `backend-go/ci/check-opa-bundle-in-images.sh` xanh cho service mới.
- Workflow: mở PR thử, hai job `postgres` và `mysql` xanh, bước `buf` chạy.

## Tiêu chí hoàn thành

- [ ] Compose dựng được `code-intel-service` và `migrate-codeintel` chạy xong; `/readyz` 200.
- [ ] `codeintel` có ở cả hai file init DB và `migrate.sh`.
- [ ] Workflow xanh hai dialect; `buf lint` chạy trực tiếp.
- [ ] Ảnh Docker chứa `policy/orca-authz/*.rego`.

## Rủi ro và lưu ý

- Dev compose chỉ Postgres: MySQL chỉ kiểm bằng CI.
- `mem_limit: 256m` chưa đo.
- `CODE_INTEL_SERVICE_ADDR` của gateway **không** thêm ở task này (SOL-040).
