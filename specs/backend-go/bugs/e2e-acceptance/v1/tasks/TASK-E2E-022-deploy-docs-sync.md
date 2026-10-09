# TASK-E2E-022: Cập nhật tài liệu triển khai

**From Solution:** SOL-E2E-004 · **Resolves:** BUG-E2E-002, BUG-E2E-005
**Priority:** P2 · **Service/Area:** docs
**Files:**
- deploy/dev/README.md
- docs/guides/request/admin-enable-request-flow.md
- tests/stages/e2e/orca-flows/README.md (chế độ `--ai-stub`, CI)
**Depends on:** TASK-E2E-019, 021
**Status:** `[~]` MỘT PHẦN: đã viết và kiểm từng lệnh không phá stack, chưa dựng thử trên máy sạch (cập nhật 2026-10-09, chưa commit)

---

## Context

README "Local-only" còn hướng dẫn `docker compose up -d postgres vault nats` trong khi compose gốc không có Vault.

## Việc cần làm

- [x] Mô tả overlay `docker-compose.local.yml`; token request; `ensure-request-env.sh`; `redeploy-local.sh`. — _`deploy/dev/README.md`: cây thư mục, mục "Local-only" viết lại (overlay, `VAULT_ADDR=http://vault:8200`, `vault-init`, `migrate.sh`, `GOARCH`), "Redeploy một vài service trên stack local", "Luồng Request trên stack local" (bảng ba token và bên dùng, `start-local-agent.sh`, mã `REQUEST_AI_*`)_
- [x] Ghi `DOCKER_HOST` cho testcontainers dưới Colima/Docker Desktop. — _mục "Docker daemon": `DOCKER_HOST` lấy từ `docker context inspect`, `TESTCONTAINERS_RYUK_DISABLED=true`_
- [x] Sửa mô tả cờ luồng (hai tầng) cho khớp. — _bảng hai tầng ở README triển khai; `admin-enable-request-flow.md` thêm "Điều kiện trước khi bật", lệnh bật/đọc cờ, sửa các câu "chưa có" sai (xem TASK-030)_

## Tiêu chí hoàn thành

- [~] Một người mới dựng được stack local theo README mà không phải hỏi. — _chưa kiểm trên máy sạch (không dựng lại được stack ở đây vì cấm tạo lại Vault/Postgres/NATS)_

## Kiểm thử

Kiểm bằng cách làm theo từng bước trên máy sạch.

## Kết quả thực thi

**Một phần.** Viết xong, đối chiếu với compose, `.env.example`, các script và mã; **chưa** làm theo từ máy sạch.

Lệnh đã kiểm thật (chỉ đọc hoặc không đổi trạng thái): `docker compose -f docker-compose.yml -f docker-compose.local.yml config --services` (liệt kê đủ service gồm `vault`, `vault-init`); `bash deploy/dev/scripts/redeploy-local.sh` (không đối số in usage, thoát mã 2); `bash deploy/dev/scripts/ensure-request-env.sh` trên `.env` đã đủ (in "is up to date", không đổi file); `docker context inspect --format '{{.Endpoints.docker.Host}}'` (trả socket của Colima và dùng được làm `DOCKER_HOST`); đoạn Python gọi `request.flowStatus` (trả `{'enabled': True}`); `start-local-agent.sh` qua `run_acceptance.py --ai-stub` (chạy và khôi phục agent thật). Kiểm liên kết tương đối của các tài liệu đã sửa: chỉ còn hai liên kết hỏng có sẵn từ trước (`specs/backend-go/architecture/06-...`, `08-...` trong README triển khai).

Chưa kiểm: toàn bộ dãy "Local-only" từ đầu (`build-local.sh`, `up -d`, `migrate.sh` trên volume rỗng); các đường dẫn socket của Docker Desktop và Linux (viết theo hiểu biết chung, không có máy để thử); nhận xét về `ORCA_AGENT_DEFAULT_TENANT_ID` đối chiếu mã `agentwsserver/config.go` nhưng chưa dựng thử tenant khác sentinel bằng tay (CI script làm đúng việc này, chưa chạy).
