# BE-CV-TASK-073-04: Script kiểm đăng ký `check-code-intel-service-wiring.sh`

**From Solution:** BE-CV-SOL-073
**Priority:** P0
**Service:** `backend-go/ci`
**File:** `backend-go/ci/check-code-intel-service-wiring.sh` (mới), `backend-go/ci/check-code-intel-service-wiring_test.sh` (mới), `.github/workflows/backend-go-code-intel-service.yml` (thêm bước)
**Depends on:** BE-CV-SOL-010 (wiring service, workflow service)
**Status:** `[ ] TODO`

---

## Context

- Hợp đồng §10 liệt kê nơi phải thêm: `backend-go/go.work`, `Makefile`, `deploy/postgres-init-databases.sh` **và** `deploy/dev/docker/postgres/init-databases.sh`, `deploy/dev/docker-compose.yml`, `deploy/dev/scripts/{migrate.sh,build-local.sh}`, `backend-go/ci/check-opa-bundle-in-images.sh`, workflow service. `deploy/` ở **gốc repo** (không `backend-go/deploy`, README v7 §8 điểm 17; riêng `postgres-init-databases.sh` đúng là ở `backend-go/deploy/`).
- Tên theo PQ-23: DB `codeintel`, `migrate-codeintel`, `orca-go-code-intel`, `CODE_INTEL_SERVICE_ADDR`, `CODEINTEL_ENABLED`.
- Mẫu: `backend-go/ci/check-opa-bundle-in-images.sh` (`set -euo pipefail`, `FAIL:`).

## Việc cần làm

1. Script kiểm lần lượt (mỗi mục `grep -q` có neo từ, không `sed`; in `FAIL: <tệp> thiếu <mục>`, thoát 1): `go.work` có `./services/code-intel-service`; `Makefile` `SERVICES` có `code-intel-service`; hai tệp init-databases có `codeintel`; `migrate.sh` `SERVICES` có `codeintel`; compose có `code-intel-service:`, `container_name: orca-go-code-intel`, `migrate-codeintel:`, `CODE_INTEL_SERVICE_ADDR` trong `api-gateway`, `CODEINTEL_ENABLED: ${CODEINTEL_ENABLED:-false}`; `check-opa-bundle-in-images.sh` `svcs` có `code-intel-service`; workflow `backend-go-code-intel-service.yml` có `dialect: [postgres, mysql]`.
2. Cho phép biến `ROOT` để test chạy trên bản sao.
3. Test (`_test.sh`): sao các tệp liên quan vào thư mục tạm, xoá từng mục, khẳng định script đỏ; bản nguyên vẹn xanh.
4. Thêm bước chạy script vào workflow service (và vào `code-intel-contract.yml` nếu hợp lý).

## Kiểm thử

- `bash backend-go/ci/check-code-intel-service-wiring_test.sh`.
- Chạy trên Linux/macOS; Windows dev dùng WSL (ghi trong chú thích đầu script).

## Tiêu chí hoàn thành

- [ ] Script xanh trên cây đã đăng ký đủ; đỏ khi xoá `codeintel` khỏi `migrate.sh`.

## Rủi ro và lưu ý

- Danh sách nơi đăng ký có thể đổi khi CR-010 triển khai; sửa script cùng PR đó.
