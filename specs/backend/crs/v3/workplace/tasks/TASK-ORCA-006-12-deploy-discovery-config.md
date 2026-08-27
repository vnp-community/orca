> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-006-12 — `deploy/`: Orca Discovery Config + Health-Gate CI Script

**Phase:** 0 — Nền tảng (song song, không phụ thuộc CR khác để bắt đầu)
**Scope:** ✅ vnp-workplace — Go (`backend/services/planner-service/config`) + deploy config (docker-compose, bash)
**Source:** [SOL-ORCA-006 §4](../solutions/SOL-ORCA-006-orca-headless-deployment.md#4-phần-thuộc-phạm-vi-planner-service-vnp-workplace)
**Depends On:** [TASK-ORCA-002-06](./TASK-ORCA-002-06-temporal-worker-wiring.md) (mở rộng `Config` của `planner-service` đã tạo ở đó — làm sau task đó để tránh 2 người sửa cùng file song song, không phải phụ thuộc kỹ thuật cứng)
**Estimated Files:** ~5 files
**Working Dir:** `/opt/repos/vnp-workplace/deploy/docker/`, `/opt/repos/vnp-workplace/backend/services/planner-service/config/`

---

## Bối cảnh quan trọng

- **Port thật của Orca: HTTP `:6769`, WebSocket `:6768`** — không phải `:3000` như trong CR gốc. Mọi giá trị mặc định/comment trong task này phải dùng `:6769`. (Sự kiện Orca-side này không đổi theo re-scope 2026-08-10 — xem SOL-ORCA-006 §10.)
- **Không tự "phát minh" cấu hình headless Orca** — Orca team **đã có sẵn** `deploy/prod/Dockerfile`, `deploy/prod/docker-compose.yml`, và một `deploy/dev/docker-compose.orca.yml` họ tự vận hành (build image `vnpblc/orca-server`, KHÔNG PHẢI `ghcr.io/stablyai/orca:latest`). Task này chỉ viết phần **thuộc `vnp-workplace`**: (1) mở rộng config discovery ở `planner-service` (`backend/services/planner-service/config/config.go`, đã tạo field cơ bản `OrcaURL`/`OrcaAPISecret`/`OrcaCallbackURL`/`OrcaCallbackSecret` ở CR-ORCA-006 Change 4) để trỏ tới 1 Orca instance (đơn giản, KHÔNG implement `OrcaInstancePool`/load-balancing — hoãn lại theo SOL-ORCA-006 §4.4/CR-ORCA-006 Change 5), (2) 1 file compose overlay riêng của `vnp-workplace` để tự host Orca cho integration test cục bộ (dev only — `deploy/docker/docker-compose.yml` chính KHÔNG đóng gói Orca, Orca team vận hành riêng), (3) health-gate script cho CI.
- **`ORCA_PLANNER_API_SECRET` và `/api/planner-tasks*` chưa tồn tại** — checklist trong task này phản ánh đúng thực tế "cần Orca team xây mới", không phải "xác nhận cấu hình có sẵn".
- **Re-scope 2026-08-10:** task này trước đây thuộc `temporal-worker/config` (repo `vnp-planner`, ngoài `vnp-workplace`). Nay chuyển thành `backend/services/planner-service/config` trong chính monorepo này — path/service name đổi, nội dung kỹ thuật (Orca discovery, health-gate) giữ nguyên tinh thần.

---

## Mục tiêu

1. Mở rộng `backend/services/planner-service/config/config.go` (đã tạo ở TASK-ORCA-002-06, theo CR-ORCA-006 Change 4) với `OrcaInstancesJSON` — chuẩn bị chỗ cho multi-instance sau này, MVP chỉ dùng 1 instance.
2. `deploy/docker/docker-compose.orca.yml` — chạy 1 Orca container cục bộ cho integration test, tham khảo đúng image/port thật của Orca team.
3. `deploy/docker/scripts/wait-for-orca.sh` — health-gate cho CI trước khi chạy integration test CR-ORCA-002/005.
4. Checklist vận hành (markdown, đặt trong `deploy/docker/` theo cấu trúc hiện có).

---

## Acceptance Criteria

- [ ] `Config.OrcaInstancesJSON` optional (`default:""`) — khi rỗng, dùng `OrcaURL`/`OrcaAPISecret` đơn instance đã có ở TASK-ORCA-002-06/CR-ORCA-006 Change 4 (không code chết `OrcaInstancePool.SelectBest` chưa có test)
- [ ] `docker-compose.orca.yml` dùng đúng port `6768:6768`/`6769:6769`, healthcheck `wget http://localhost:6769/health/ready` (không phải `/health` trơn), image pin tag cụ thể qua biến `${ORCA_VERSION}` (không dùng `:latest` ở staging/prod — dev được phép)
- [ ] `wait-for-orca.sh` dùng đúng port `6769` + route `/health/ready`
- [ ] Checklist ghi rõ: `/api/planner-tasks` và `ORCA_PLANNER_API_SECRET`/`ORCA_API_SECRET` là "cần xây mới", không phải "cần xác nhận cấu hình có sẵn"
- [ ] `go build ./services/planner-service/...` vẫn pass sau khi thêm field mới vào `Config`

---

## File 1: `backend/services/planner-service/config/config.go` [MODIFY — bổ sung so với TASK-ORCA-002-06 / CR-ORCA-006 Change 4]

```go
// Thêm vào struct Config đã có (TASK-ORCA-002-06, field OrcaURL/OrcaAPISecret/
// OrcaCallbackURL/OrcaCallbackSecret/OrcaPollIntervalSecs/... đã tồn tại):

	// Orca — hỗ trợ multi-instance qua danh sách JSON (dự phòng cho load-balancing
	// tương lai, KHÔNG triển khai OrcaInstancePool.SelectBest ở giai đoạn này —
	// xem SOL-ORCA-006 §4.4 / CR-ORCA-006 Change 5). MVP: để rỗng, dùng
	// OrcaURL/OrcaAPISecret đơn instance.
	OrcaInstancesJSON string `envconfig:"ORCA_INSTANCES_JSON" default:""`
```

Thêm type (không dùng ở MVP nhưng khai báo để JSON schema ổn định khi cần bật sau):

```go
// backend/services/planner-service/config/orca_instance.go [NEW]
package config

// OrcaInstanceConfig describes one Orca server for future multi-instance
// support. NOT used by MVP dispatch logic — ORCA_INSTANCES_JSON parsing and
// selection logic are deferred until ErrUnavailable frequency from
// planner-service's Orca client justifies it (SOL-ORCA-006 §4.4). Declared
// now only so the config surface does not need another breaking change later.
type OrcaInstanceConfig struct {
	URL                 string   `json:"url"`
	Name                string   `json:"name"`
	APISecret           string   `json:"api_secret"`
	MaxConcurrentTasks  int      `json:"max_concurrent_tasks"`
	PreferredAgentTypes []string `json:"preferred_agent_types"`
}
```

---

## File 2: `deploy/docker/docker-compose.orca.yml` [NEW]

```yaml
# Orca instance for LOCAL integration testing only. staging/prod do NOT bundle
# Orca — it is operated independently by the Orca team (see
# docs/hld/C1-System-Context.md, System_Ext(orca, ...)). Point ORCA_URL at
# their instance for those environments instead of using this file.
#
# Image/ports below mirror the Orca team's own deploy/prod/Dockerfile +
# docker-compose.yml (repo `orca`) — vnpblc/orca-server, NOT
# ghcr.io/stablyai/orca:latest (that image does not exist anywhere in the
# orca repo — likely a stale/incorrect reference from an earlier draft).
version: '3.9'

services:
  orca:
    image: vnpblc/orca-server:${ORCA_VERSION:-latest}   # pin ${ORCA_VERSION} outside dev
    container_name: wkp-dev-orca
    ports:
      - "6768:6768"   # WebSocket
      - "6769:6769"   # HTTP — /health, /health/ready, /api/trace-stream, /api/agent-token
                       # NOTE: /api/planner-tasks* is NOT yet implemented by this image
                       # until the Orca team ships TASK-ORCA-001-13.
    environment:
      ORCA_PORT: "6768"
      ORCA_HTTP_PORT: "6769"
      ORCA_MULTI_USER: "false"
      ORCA_AUTH_MODE: "${ORCA_AUTH_MODE:-local}"
      # ORCA_PLANNER_API_SECRET — set once the Orca team adds this env var
      # (does not exist in the image today, see SOL-ORCA-001 §9 pt.3). Must
      # match ORCA_API_SECRET on the planner-service side (Change 4).
      ORCA_PLANNER_API_SECRET: "${ORCA_API_SECRET:-dev-secret-placeholder}"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:6769/health/ready"]
      interval: 10s
      timeout: 5s
      retries: 6
    restart: unless-stopped
    networks:
      - kwp-network

networks:
  kwp-network:
    external: true
```

---

## File 3: `deploy/docker/scripts/wait-for-orca.sh` [NEW hoặc MODIFY nếu pattern script tương tự đã có]

```bash
#!/usr/bin/env bash
# Health-gate for CI: block until Orca's real HTTP port/route is ready before
# running CR-ORCA-002/005 integration tests. Port 6769 (HTTP) + /health/ready
# (the route the Orca team's own Docker healthcheck uses) — NOT :3000/health.
set -euo pipefail

ORCA_HOST="${ORCA_HOST:-orca}"
ORCA_HTTP_PORT="${ORCA_HTTP_PORT:-6769}"
TIMEOUT_SECS="${WAIT_FOR_ORCA_TIMEOUT:-60}"

echo "Waiting up to ${TIMEOUT_SECS}s for http://${ORCA_HOST}:${ORCA_HTTP_PORT}/health/ready ..."
timeout "${TIMEOUT_SECS}" bash -c \
  "until curl -sf http://${ORCA_HOST}:${ORCA_HTTP_PORT}/health/ready > /dev/null; do sleep 2; done" \
  || { echo "Orca not healthy after ${TIMEOUT_SECS}s"; exit 1; }

echo "Orca is healthy."
```

Cấp quyền thực thi: `chmod +x deploy/docker/scripts/wait-for-orca.sh`.

---

## File 4: `deploy/docker/checklists/orca-integration-checklist.md` [NEW]

```markdown
# Orca Integration Checklist (CR-ORCA-002/005/006)

## Trước khi tích hợp (Orca team xác nhận)
- [ ] `curl http://orca:6769/health/ready` → 200 (ổn định, không chỉ lúc khởi động)
- [ ] `/api/planner-tasks*` được XÂY MỚI đúng SOL-ORCA-001 (bao gồm idempotency §4)
      — endpoint này CHƯA TỒN TẠI hôm nay, đây là yêu cầu xây mới, không phải
      "xác nhận version đã triển khai"
- [ ] `ORCA_PLANNER_API_SECRET` đã chia sẻ an toàn (không qua Slack/email plaintext)
      — biến MỚI HOÀN TOÀN, Orca team cần thêm vào cấu hình của họ trước

## `planner-service` config (đội vnp-workplace)
- [ ] `planner-service`: `ORCA_URL`/`ORCA_API_SECRET`/`ORCA_CALLBACK_URL` đúng (TASK-ORCA-002-06)
- [ ] `ORCA_CALLBACK_URL` (`http://planner-service:3013/api/v1/orca-callback`) reachable từ phía Orca (test bằng curl từ Orca host)
- [ ] `deploy/docker/docker-compose.orca.yml` chạy được cho integration test cục bộ
- [ ] `deploy/docker/scripts/wait-for-orca.sh` tích hợp vào CI trước khi chạy integration test

## Sau khi lên production
- [ ] Dashboard `orca.instance.offline`/`orca.session.timeout` (qua `diagnostics-service`, tái dùng thay `signal-svc`) route tới kênh cảnh báo vận hành
- [ ] Theo dõi tần suất `ErrUnavailable` từ Orca client của `planner-service` — ngưỡng cân nhắc multi-instance
      (KHÔNG implement `OrcaInstancePool` cho tới khi có bằng chứng tải thật)
```

---

## Verification

```bash
cd /opt/repos/vnp-workplace/backend

go build ./services/planner-service/...
go vet ./services/planner-service/...

# Compose smoke test (dev only — không chạy ở CI nếu Orca team chưa cấp image thật)
docker compose -f ../deploy/docker/docker-compose.yml -f ../deploy/docker/docker-compose.orca.yml --profile phase2 config
bash ../deploy/docker/scripts/wait-for-orca.sh   # kỳ vọng fail có kiểm soát nếu Orca chưa chạy, không crash script
```
