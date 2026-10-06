# TASK-REQ-025-06: Script `check-request-service-wiring.sh` kiểm đăng ký đầy đủ

**From Solution:** BE-REQ-SOL-025
**Priority:** P0
**Service:** `backend-go/ci`
**File:** `backend-go/ci/check-request-service-wiring.sh` (mới), `backend-go/ci/check-request-service-wiring_test.sh` (mới, hoặc bước test trong workflow), `.github/workflows/backend-go-request-service.yml`
**Depends on:** CR-REQ-001 (đã đăng ký `request` ở mọi nơi)
**Status:** `[ ] TODO`

---

## Context

- Mẫu: `backend-go/ci/check-opa-bundle-in-images.sh` (`#!/usr/bin/env bash`, `set -euo pipefail`, `cd "$(dirname "$0")/.."`, thông báo `FAIL:` ra stderr, `exit 1`).
- Các nơi đăng ký (đã đọc): `backend-go/go.work` (có `./services/issue-status-sync`, `./services/mcp-service`), `backend-go/Makefile` (`SERVICES :=` dòng 6-12, tên dạng `xxx-service`), `backend-go/deploy/postgres-init-databases.sh` (`DATABASES="... mcp"` dòng 8), `deploy/dev/docker-compose.yml` (`mcp-service` dòng 408, `migrate-mcp` dòng 741), `deploy/dev/scripts/migrate.sh` (`SERVICES="... mcp issuestatussync"`).
- Bài học: `migrate.sh` từng quên `issuetracking` (BUG-014): DB không bao giờ được migrate.
- Chạy được trên Linux runner và macOS; Windows dùng WSL (AGENTS.md: đa nền tảng).

## Việc cần làm

1. Viết script kiểm lần lượt, mỗi kiểm in `OK:`/`FAIL:`: (a) `go.work` có `./services/request-service`; (b) `Makefile` `SERVICES` có `request-service`; (c) `postgres-init-databases.sh` `DATABASES` có từ `request`; (d) `deploy/dev/docker-compose.yml` có khoá `request-service:` và `migrate-request:`; (e) cùng file có `REQUEST_SERVICE_ADDR` cho `api-gateway` và `issue-status-sync`; (f) `migrate.sh` `SERVICES` có từ `request`; (g) `.github/workflows/backend-go-request-service.yml` tồn tại và có `matrix` với `postgres` và `mysql`. Dùng `grep -Eq` với neo từ (`(^|[[:space:]"])request([[:space:]"]|$)`); không dùng `sed -i`, `readlink -f`, `grep -P` (không đa nền tảng).
2. Test của script: `check-request-service-wiring_test.sh` sao thư mục cần thiết ra `mktemp -d`, xoá `request` khỏi từng nơi một và khẳng định script thoát mã khác 0 với đúng thông báo (7 ca) và thoát 0 khi nguyên vẹn.
3. Gọi script trong workflow `backend-go-request-service.yml` một lần (job riêng `wiring`, không theo ma trận).

## Kiểm thử

- Chạy: `bash backend-go/ci/check-request-service-wiring.sh` (sau khi CR-REQ-001 đăng ký) thoát 0; `bash backend-go/ci/check-request-service-wiring_test.sh` thoát 0.
- Cố ý xoá `request` khỏi `migrate.sh` (bản sao) thì script đỏ.

## Tiêu chí hoàn thành

- [ ] 7 kiểm và 7 ca âm tính.
- [ ] Chạy trong CI, đỏ khi thiếu.

## Rủi ro và lưu ý

- Tên DB ngắn khác nhau giữa `postgres-init-databases.sh` (`request`) và tên service (`request-service`); script phải kiểm theo quy ước từng file.
- Khi thêm service khác sau này, đừng nhồi vào script này; làm script riêng hoặc tổng quát hoá có chủ ý.
